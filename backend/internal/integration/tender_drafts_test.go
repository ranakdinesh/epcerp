package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authmodule "github.com/ranakdinesh/epcERP/internal/modules/auth"
	packagemodule "github.com/ranakdinesh/epcERP/internal/modules/document_packages"
	packageout "github.com/ranakdinesh/epcERP/internal/modules/document_packages/adapters/outbound"
	packageapp "github.com/ranakdinesh/epcERP/internal/modules/document_packages/application"
	packagecontracts "github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	packagedomain "github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
	readout "github.com/ranakdinesh/epcERP/internal/modules/document_reading/adapters/outbound"
	readapp "github.com/ranakdinesh/epcERP/internal/modules/document_reading/application"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	readomain "github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	readports "github.com/ranakdinesh/epcERP/internal/modules/document_reading/ports"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	tenderhttp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/inbound"
	tenderout "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	tenderapp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	tenderdomain "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentDraftRetentionPublicationRecoveryAndScope(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	ids := tenderout.Values{}
	owner, outsider, role := ids.ID(), ids.ID(), ids.ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'draft-fixture','Fixture',true)", role)
	for i, id := range []string{owner, outsider} {
		name := []string{"draft_owner", "draft_outsider"}[i]
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser,must_change_password)VALUES($1,$2,$3,'unusable',true,false,false)", id, name, name+"@fixture.invalid")
		execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", id, role)
	}
	// Deliberately no register.assign permission or Manager: upload/read is independent.
	for _, code := range []string{"tenders.register.view", "tenders.register.read", "tenders.register.write", "tenders.processing.read", "tenders.processing.write"} {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'Fixture')ON CONFLICT(code)DO NOTHING", ids.ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	identity := projectIdentities{"owner": {ID: owner, Active: true}, "outsider": {ID: outsider, Active: true}}
	auth := &authmodule.Module{}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	key := strings.Repeat("cf", 32)
	cipher, e := infrastructure.NewDocumentCipher(key)
	if e != nil {
		t.Fatal(e)
	}
	store, e := infrastructure.NewDocumentStreamStorage(filepath.Join(t.TempDir(), "sources"), cipher)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	rc, e := readout.NewCipher(key)
	if e != nil {
		t.Fatal(e)
	}
	reader := func(c database.Connection) reading.Reader {
		return readapp.New(readout.NewRepository(c, journal(c)), rc, readout.Values{}, readout.Inspector{}, readout.Codec{})
	}
	prepared := func(c database.Connection) reading.PreparedReader {
		return readapp.New(readout.NewRepository(c, journal(c)), rc, readout.Values{}, readout.Inspector{}, readout.Codec{})
	}
	packages := func(c database.Connection) packagecontracts.DraftConsumer {
		return packagemodule.DraftConsumer(c, auth.Assignments(c))
	}
	unit := tenderout.NewDraftUnit(pool, auth.Assignments, packages, reader, prepared)
	service := tenderapp.NewDrafts(unit, tenderout.NewAuthBridge(identity))
	operation := ids.ID()
	draft, e := service.Create(ctx, "owner", operation, "Reading fixture")
	if e != nil {
		t.Fatal("draft create", e)
	}
	router := chi.NewRouter()
	tenderhttp.RegisterDrafts(router, service)
	for _, tc := range []struct {
		method, path, body, actor string
		status                    int
	}{
		{"GET", "/api/v1/tender-fms/drafts/" + draft.ID, "", "owner", 200},
		{"GET", "/api/v1/tender-fms/drafts/" + draft.ID, "", "outsider", 404},
		{"POST", "/api/v1/tender-fms/drafts", `{"name":"Invalid extra","manager_id":"not-allowed"}`, "owner", 422},
		{"GET", "/api/v1/tender-fms/drafts?page_size=1000", "", "owner", 422},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.actor)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", ids.ID())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("HTTP %s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	replay, e := service.Create(ctx, "owner", operation, "Reading fixture")
	if e != nil || replay.ID != draft.ID {
		t.Fatal("draft replay", e)
	}
	if _, e = service.Create(ctx, "owner", operation, "Different"); e == nil {
		t.Fatal("changed operation accepted")
	}
	if _, e = service.Get(ctx, "outsider", draft.ID); !errors.Is(e, tenderdomain.Missing) {
		t.Fatal("draft leaked", e)
	}
	pu := packageout.NewUnit(pool, auth.Assignments)
	ps := packageapp.New(pu, packageout.Auth{Resolver: identity}, packageout.Values{}, packageout.Objects{Store: store})
	data, e := os.ReadFile("../modules/document_reading/adapters/outbound/testdata/native.pdf")
	if e != nil {
		t.Fatal(e)
	}
	receipt, _, e := ps.Create(ctx, "owner", ids.ID(), []packagedomain.Declaration{{ClientKey: ids.ID(), Filename: "tender.pdf", Kind: "pdf", Size: int64(len(data)), SHA256: packageHash(data)}})
	if e != nil {
		t.Fatal(e)
	}
	if e = service.Attach(ctx, "owner", draft.ID, receipt.ID); e != nil {
		t.Fatal("retain", e)
	}
	if e = service.Attach(ctx, "owner", draft.ID, receipt.ID); e != nil {
		t.Fatal("attach replay", e)
	}
	var retained bool
	if e = pool.QueryRow(ctx, "SELECT expires_at>clock_timestamp()+interval '100 years' FROM document_package_receipts WHERE id=$1", receipt.ID).Scan(&retained); e != nil || !retained {
		t.Fatal("expiry", e)
	}
	file := receipt.Files[0]
	receipt, _, e = ps.Upload(ctx, "owner", receipt.ID, file.ID, ids.ID(), file.Version, bytes.NewReader(data))
	if e != nil {
		t.Fatal("upload", e)
	}
	receipt, _, e = ps.Seal(ctx, "owner", receipt.ID, ids.ID(), receipt.Version)
	if e != nil {
		t.Fatal("seal", e)
	}
	preparer := readapp.New(nil, rc, readout.Values{}, readout.Inspector{}, readout.Codec{})
	pw := packageapp.NewWorker(pu, packageout.Values{}, packageout.Verifier{Store: store, Scanner: fixturePackageScanner{}, Pages: fixturePackagePages{}, Preparer: preparer})
	deferred := packageapp.NewWorker(pu, packageout.Values{}, packageout.Verifier{Store: store, Scanner: draftUnavailableScanner{}, Pages: fixturePackagePages{}, Preparer: preparer})
	if worked, e := deferred.Once(ctx); !worked || e != nil {
		t.Fatal("transient scanner attempt", e)
	}
	waiting, e := ps.Get(ctx, "owner", receipt.ID)
	if e != nil || waiting.State != "verifying" || waiting.ErrorCode != "dependency_unavailable" {
		t.Fatal("scanner retry not durable", waiting, e)
	}
	execFixture(t, pool, "UPDATE document_package_receipts SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", receipt.ID)
	if worked, e := pw.Once(ctx); !worked || e != nil {
		t.Fatal("verification", e)
	}
	publish := func() error { return tenderapp.NewDraftPublisher(unit).Once(ctx) }
	if e = publish(); e != nil {
		t.Fatal("publish", e)
	}
	if e = publish(); e != nil {
		t.Fatal("publish replay", e)
	}
	loaded, e := service.Get(ctx, "owner", draft.ID)
	if e != nil || len(loaded.Batches) != 1 || len(loaded.Batches[0].Sources) != 1 {
		t.Fatal("reopen", loaded, e)
	}
	doc := loaded.Batches[0].Sources[0]
	if doc.Status != "queued" {
		t.Fatal(doc)
	}
	var n int
	pool.QueryRow(ctx, "SELECT count(*) FROM document_readings WHERE module='tender_draft' AND entity_id=$1", draft.ID).Scan(&n)
	if n != 1 {
		t.Fatal("duplicate job", n)
	}
	// An interrupted worker leaves its durable claim. A new worker reclaims after
	// lease expiry; a previous claim is fenced and cannot publish a result.
	ru := readout.NewUnitOfWork(pool, journal)
	// Simulate process termination after durable claim without running a parser.
	started := make(chan struct{})
	interruptedCtx, cancelWorker := context.WithCancel(ctx)
	finished := make(chan error, 1)
	interrupted := readapp.NewWorker(ru, rc, readout.Values{}, draftInterruptedEngine{started}, readout.Codec{})
	go func() { _, e := interrupted.Once(interruptedCtx); finished <- e }()
	<-started
	cancelWorker()
	if e := <-finished; !errors.Is(e, context.Canceled) {
		t.Fatal("worker interruption was not recoverable", e)
	}
	var abandonedClaim string
	if e = pool.QueryRow(ctx, "SELECT claim_id::text FROM document_readings WHERE id=$1 AND status='processing' AND attempts=1", doc.ID).Scan(&abandonedClaim); e != nil {
		t.Fatal("interrupted claim lost", e)
	}
	execFixture(t, pool, "UPDATE document_readings SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", doc.ID)
	rw := readapp.NewWorker(ru, rc, readout.Values{}, draftFixtureEngine{}, readout.Codec{})
	if worked, e := rw.Once(ctx); !worked || e != nil {
		t.Fatal("restart recovery", e)
	}
	loaded, e = service.Get(ctx, "owner", draft.ID)
	if e != nil || loaded.Batches[0].Sources[0].Status != "ready" || loaded.Batches[0].Sources[0].Attempts != 2 {
		t.Fatal("recovered projection", loaded, e)
	}
	if e = ru.Within(ctx, func(ctx context.Context, r readports.Repository) error {
		_, e := r.Finish(ctx, readomain.Document{ID: doc.ID, ClaimID: abandonedClaim}, nil, "", readomain.Result{}, "stale worker")
		return e
	}); !errors.Is(e, readomain.Conflict) {
		t.Fatal("stale claim publication accepted", e)
	}
	if len(loaded.Batches[0].Manifest) == 0 {
		t.Fatal("missing manifest")
	}
	text, e := service.Artifact(ctx, "owner", draft.ID, doc.ID, "text")
	if e != nil || !bytes.Contains(text, []byte("Fixture reading")) {
		t.Fatal("artifact", e)
	}
	if _, e = service.Artifact(ctx, "outsider", draft.ID, doc.ID, "original"); !errors.Is(e, tenderdomain.Missing) {
		t.Fatal("original leaked", e)
	}
	verifyTSCNative(t, ctx, pool, unit, identity, journal, cipher, loaded, owner, role)
	// A second immutable batch is independently retained. Identical bytes reuse
	// the same scoped reading while preserving both source occurrences and names.
	second, _, e := ps.Create(ctx, "owner", ids.ID(), []packagedomain.Declaration{{ClientKey: ids.ID(), Filename: "reissued.pdf", Kind: "pdf", Size: int64(len(data)), SHA256: packageHash(data)}})
	if e != nil {
		t.Fatal(e)
	}
	if e = service.Attach(ctx, "owner", draft.ID, second.ID); e != nil {
		t.Fatal("append batch", e)
	}
	sf := second.Files[0]
	second, _, e = ps.Upload(ctx, "owner", second.ID, sf.ID, ids.ID(), sf.Version, bytes.NewReader(data))
	if e != nil {
		t.Fatal(e)
	}
	second, _, e = ps.Seal(ctx, "owner", second.ID, ids.ID(), second.Version)
	if e != nil {
		t.Fatal(e)
	}
	if worked, e := pw.Once(ctx); !worked || e != nil {
		t.Fatal("second verification", e)
	}
	if e = publish(); e != nil {
		t.Fatal("second publication", e)
	}
	appended, e := service.Get(ctx, "owner", draft.ID)
	if e != nil || len(appended.Batches) != 2 {
		t.Fatal("append projection", e)
	}
	if appended.Batches[0].Revision != receipt.ID || appended.Batches[1].Revision != second.ID {
		t.Fatal("source identities changed")
	}
	firstSource, nextSource := appended.Batches[0].Sources[0], appended.Batches[1].Sources[0]
	if firstSource.ID != nextSource.ID || firstSource.MemberID == nextSource.MemberID || nextSource.Filename != "reissued.pdf" || nextSource.Status != "ready" {
		t.Fatal("duplicate bytes lost source occurrence or requeued completed work")
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", owner)
	if _, e = service.Artifact(ctx, "owner", draft.ID, doc.ID, "text"); e == nil {
		t.Fatal("revoked derived access")
	}
}

type draftFixtureEngine struct{}

func (draftFixtureEngine) Read(ctx context.Context, b []byte, format, hash string) (readomain.Result, error) {
	return readomain.Result{Schema: readomain.ReaderVersion, ReaderVersion: readomain.ReaderVersion, SourceSHA256: hash, Segments: []readomain.Segment{{Kind: "page", Label: "Page 1", Text: "Fixture reading", Method: "native"}}, Warnings: []string{}}, nil
}

func TestDocumentPipelineSharedSlot(t *testing.T) {
	_, pool := owned(t)
	ctx := context.Background()
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, e := infrastructure.WithDocumentSlot(ctx, pool, func(ctx context.Context) (bool, error) { close(entered); <-release; return true, nil })
		finished <- e
	}()
	<-entered
	called := false
	if worked, e := infrastructure.WithDocumentSlot(ctx, pool, func(ctx context.Context) (bool, error) { called = true; return true, nil }); e != nil || worked || called {
		close(release)
		t.Fatal("parallel document processing allowed", e)
	}
	close(release)
	if e := <-finished; e != nil {
		t.Fatal(e)
	}
	if worked, e := infrastructure.WithDocumentSlot(ctx, pool, func(ctx context.Context) (bool, error) { return true, nil }); e != nil || !worked {
		t.Fatal("slot not released", e)
	}
}

type draftUnavailableScanner struct{}

func (draftUnavailableScanner) Scan(context.Context, *os.File) (string, error) {
	return "", packagedomain.Unavailable
}

type draftInterruptedEngine struct{ started chan struct{} }

func (e draftInterruptedEngine) Read(ctx context.Context, b []byte, format, hash string) (readomain.Result, error) {
	close(e.started)
	<-ctx.Done()
	return readomain.Result{}, ctx.Err()
}
