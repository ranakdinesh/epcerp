package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authmodule "github.com/ranakdinesh/epcERP/internal/modules/auth"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox"
	inboxcontracts "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	packages "github.com/ranakdinesh/epcERP/internal/modules/document_packages"
	packageout "github.com/ranakdinesh/epcERP/internal/modules/document_packages/adapters/outbound"
	packageapp "github.com/ranakdinesh/epcERP/internal/modules/document_packages/application"
	packagecontracts "github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	packagedomain "github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
	readout "github.com/ranakdinesh/epcERP/internal/modules/document_reading/adapters/outbound"
	readapp "github.com/ranakdinesh/epcERP/internal/modules/document_reading/application"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	tender "github.com/ranakdinesh/epcERP/internal/modules/tender_fms"
	tenderout "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	tenderapp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	tendercontracts "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/contracts"
	tenderdomain "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fixturePackageScanner struct{ fail bool }

func (s fixturePackageScanner) Scan(context.Context, *os.File) (string, error) {
	if s.fail {
		return "", packagedomain.Fail(422, "malware_detected", "Fixture scanner detected malicious content")
	}
	return "ClamAV fixture-reviewed", nil
}

type fixturePackagePages struct{}

func (fixturePackagePages) Pages(context.Context, *os.File) (int32, error) { return 1, nil }
func packageHash(b []byte) string                                          { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func TestReceiptCaptureAtomicReplayScopeAndReassignment(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	ids := tenderout.Values{}
	sales, manager, next, outsider, bundle := ids.ID(), ids.ID(), ids.ID(), ids.ID(), ids.ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'capture-fixture','Fixture only',true)", bundle)
	for i, id := range []string{sales, manager, next, outsider} {
		name := []string{"capture_sales", "capture_manager", "next_manager", "capture_outsider"}[i]
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser,must_change_password)VALUES($1,$2,$3,'unusable',true,false,false)", id, name, name+"@fixture.invalid")
		execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", id, bundle)
	}
	for _, code := range tenderdomain.ManagerPermissions() {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'Fixture only')ON CONFLICT(code)DO NOTHING", ids.ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", bundle, code)
	}
	identities := projectIdentities{"sales": {ID: sales, Active: true}, "manager": {ID: manager, Active: true}, "next": {ID: next, Active: true}, "outsider": {ID: outsider, Active: true}}
	authmod := &authmodule.Module{}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	key := strings.Repeat("ca", 32)
	cipher, e := infrastructure.NewDocumentCipher(key)
	if e != nil {
		t.Fatal(e)
	}
	store, e := infrastructure.NewDocumentStreamStorage(filepath.Join(t.TempDir(), "private"), cipher)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	readCipher, e := readout.NewCipher(key)
	if e != nil {
		t.Fatal(e)
	}
	preparer := readapp.New(nil, readCipher, readout.Values{}, readout.Inspector{}, readout.Codec{})
	readFactory := func(c database.Connection) reading.PreparedReader {
		return readapp.New(readout.NewRepository(c, journal(c)), readCipher, readout.Values{}, readout.Inspector{}, readout.Codec{})
	}
	unit := packageout.NewUnit(pool, authmod.Assignments)
	service := packageapp.New(unit, packageout.Auth{Resolver: identities}, packageout.Values{}, packageout.Objects{Store: store})
	verifier := packageout.Verifier{Store: store, Scanner: fixturePackageScanner{}, Pages: fixturePackagePages{}, Preparer: preparer}
	worker := packageapp.NewWorker(unit, packageout.Values{}, verifier)
	makeReceipt := func(filename, kind string, data []byte) (packagedomain.Receipt, string) {
		t.Helper()
		operation := ids.ID()
		decl := []packagedomain.Declaration{{ClientKey: ids.ID(), Filename: filename, Kind: kind, Size: int64(len(data)), SHA256: packageHash(data)}}
		receipt, replay, e := service.Create(ctx, "sales", operation, decl)
		if e != nil || replay {
			t.Fatal("create receipt", e)
		}
		same, replay, e := service.Create(ctx, "sales", operation, decl)
		if e != nil || !replay || same.ID != receipt.ID {
			t.Fatal("receipt replay", e)
		}
		changed := append([]packagedomain.Declaration{}, decl...)
		changed[0].Filename = "changed." + kind
		if _, _, e = service.Create(ctx, "sales", operation, changed); e == nil {
			t.Fatal("changed key accepted")
		}
		if e = service.Preflight(ctx, "outsider", receipt.ID, receipt.Files[0].ID); !errors.Is(e, packagedomain.Missing) {
			t.Fatal("cross owner preflight", e)
		}
		if _, e = service.Get(ctx, "outsider", receipt.ID); !errors.Is(e, packagedomain.Missing) {
			t.Fatal("receipt leak", e)
		}
		uploadKey := ids.ID()
		file := receipt.Files[0]
		receipt, _, e = service.Upload(ctx, "sales", receipt.ID, file.ID, uploadKey, file.Version, bytes.NewReader(data))
		if e != nil {
			t.Fatal("upload", e)
		}
		if _, replay, e = service.Upload(ctx, "sales", receipt.ID, file.ID, uploadKey, file.Version, bytes.NewReader(data)); e != nil || !replay {
			t.Fatal("upload replay", e)
		}
		receipt, _, e = service.Seal(ctx, "sales", receipt.ID, ids.ID(), receipt.Version)
		if e != nil {
			t.Fatal("seal", e)
		}
		if worked, e := worker.Once(ctx); e != nil || !worked {
			t.Fatal("verification", e)
		}
		receipt, e = service.Get(ctx, "sales", receipt.ID)
		if e != nil {
			t.Fatal(e)
		}
		return receipt, operation
	}
	pdf := []byte("%PDF-1.7\nFixture bounded source\n%%EOF")
	// ZIP and direct-file captures use the same manifest and every occurrence is retained.
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	for _, name := range []string{"specification.pdf", "annex/payment.pdf"} {
		f, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = f.Write(pdf)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	receipt, _ := makeReceipt("tender-package.zip", "zip", archive.Bytes())
	if receipt.State != "ready" || receipt.MemberCount != 2 {
		t.Fatal("manifest coverage", receipt.State, receipt.MemberCount)
	}
	foundation := func(c database.Connection) tendercontracts.Records {
		return tender.Records(c, authmod.Assignments(c), authmod.Bundles(c), []string{bundle}, journal(c))
	}
	consumer := func(c database.Connection) packagecontracts.Consumer {
		return packages.Consumer(c, authmod.Assignments(c))
	}
	notices := func(c database.Connection) inboxcontracts.TenderWriter {
		return inbox.TenderWriter(c, journal(c), tender.AssignmentActions(c, journal(c)))
	}
	build := func(n func(database.Connection) inboxcontracts.TenderWriter) *tenderapp.CaptureService {
		return tenderapp.NewCapture(tenderout.NewCaptureUnit(pool, authmod.Assignments, authmod.Bundles, []string{bundle}, journal, foundation, consumer, readFactory, n), tenderout.NewAuthBridge(identities), ids)
	}
	capture := build(notices)
	input := tenderdomain.CaptureInput{Name: "Solar lighting EPC opportunity", ReceiptID: receipt.ID, ReceiptVersion: receipt.Version, ManagerID: manager}
	// Failure after source queueing must roll back consumption, named record, work and mail.
	failing := func(c database.Connection) inboxcontracts.TenderWriter { return failingTenderNotice{} }
	if _, _, e = build(failing).Capture(ctx, "sales", ids.ID(), input); e == nil {
		t.Fatal("mandatory inbox failure accepted")
	}
	var count int
	for _, table := range []string{"tender_capture_operations", "tender_provision_operations", "tender_source_actions", "tender_email_jobs", "tender_source_links", "document_readings"} {
		if e = pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); e != nil || count != 0 {
			t.Fatal("partial transaction", table, count, e)
		}
	}
	current, e := service.Get(ctx, "sales", receipt.ID)
	if e != nil || current.State != "ready" {
		t.Fatal("receipt consumption escaped rollback", e)
	}
	operation := ids.ID()
	var wg sync.WaitGroup
	results := make(chan tenderdomain.Captured, 4)
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, _, e := capture.Capture(ctx, "sales", operation, input)
			results <- out
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("concurrent capture", e)
		}
	}
	var saved tenderdomain.Captured
	for v := range results {
		if saved.Record.ID == "" {
			saved = v
		}
		if saved.Record.ID != v.Record.ID {
			t.Fatal("capture duplicated")
		}
	}
	if saved.ProvisionState != "pending" || saved.ProjectID != "" || saved.EstimateID != "" || len(saved.Deliveries) != 1 || saved.Deliveries[0].NoticeID == "" || saved.Deliveries[0].EmailJobID == "" {
		t.Fatal("false handoff or missing Manager work", saved)
	}
	for table, want := range map[string]int{"tender_capture_operations": 1, "tender_provision_operations": 1, "tender_source_actions": 1, "tender_email_jobs": 1, "tender_source_links": 2, "document_readings": 1} {
		if e = pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); e != nil || count != want {
			t.Fatal("wrong committed count", table, count, want, e)
		}
	}
	changed := input
	changed.Name = "Different opportunity"
	if _, _, e = capture.Capture(ctx, "sales", operation, changed); e == nil {
		t.Fatal("capture key conflict missing")
	}
	if _, _, e = capture.Capture(ctx, "sales", ids.ID(), input); e == nil {
		t.Fatal("consumed receipt reused")
	}
	if _, e = capture.Get(ctx, "outsider", saved.Record.ID); !errors.Is(e, tenderdomain.Missing) {
		t.Fatal("cross tender detail leak", e)
	}
	rows, total, e := capture.List(ctx, "outsider", 1, 20)
	if e != nil || total != 0 || len(rows) != 0 {
		t.Fatal("scoped count leak", e)
	}
	handover := tenderdomain.ChangeManager{Version: 1, AssignmentVersion: 1, ManagerID: next, Reason: "Explicit responsible Manager handover"}
	updated, replay, e := capture.Change(ctx, "sales", saved.Record.ID, ids.ID(), handover)
	if e != nil || replay {
		t.Fatal("handover", e)
	}
	if updated.Record.SalesCreatorID != sales || updated.Record.ManagerID != next || updated.Record.ContentRevision != 1 || updated.Record.BusinessVersion != 2 || updated.Record.AssignmentVersion != 2 {
		t.Fatal("handover damaged identity", updated)
	}
	if _, e = capture.Get(ctx, "manager", saved.Record.ID); !errors.Is(e, tenderdomain.Missing) {
		t.Fatal("former Manager retained access", e)
	}
	var state string
	if e = pool.QueryRow(ctx, "SELECT state FROM tender_email_jobs WHERE id=$1", saved.Deliveries[0].EmailJobID).Scan(&state); e != nil || state != "cancelled_obsolete" {
		t.Fatal("old addressed mail not cancelled", e, state)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM communication_notices WHERE notice_kind='tender_manager'").Scan(&count); e != nil || count != 2 {
		t.Fatal("notice evidence lost", e, count)
	}
	// Same source bytes under a fresh receipt form a separate named opportunity.
	another, _ := makeReceipt("same-source.pdf", "pdf", pdf)
	anotherInput := input
	anotherInput.ReceiptID = another.ID
	anotherInput.ReceiptVersion = another.Version
	separate, _, e := capture.Capture(ctx, "sales", ids.ID(), anotherInput)
	if e != nil || separate.Record.ID == saved.Record.ID {
		t.Fatal("hash merged distinct tender", e)
	}
	// Consumed original/manifest/history and current Manager FK cannot be rewritten.
	for _, q := range []string{"UPDATE document_package_receipts SET state='ready',consumed_by_tender_id=NULL WHERE id=$1", "UPDATE document_package_members SET path='tampered' WHERE receipt_id=$1"} {
		if _, e = pool.Exec(ctx, q, receipt.ID); e == nil {
			t.Fatal("immutable consumed evidence changed")
		}
	}
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='tenders.register.write')", bundle)
	if _, _, e = capture.Capture(ctx, "sales", operation, input); !errors.Is(e, tenderdomain.Forbidden) {
		t.Fatal("replay restored revoked permission", e)
	}
}

type failingTenderNotice struct{}

func (failingTenderNotice) TenderAssignment(context.Context, string) (string, error) {
	return "", errors.New("fixture mandatory inbox failure")
}
