package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"

	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"image"
	"image/png"
	"strings"
	"sync"
	"testing"
)

func TestNativeSupplierEvidenceFinanceAndOrderGate(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actor := "135bcaf6-e4ec-4809-b7e9-42b94c332891"
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'supplier_workflow','fixture@example.invalid','fixture',true,false)", actor)

	role := (masterRepository.VendorVerificationValues{}).ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'supplier review fixture','test',true)", role)
	for _, code := range []string{"settings.suppliers.read", "accounts.vendor_verification.read", "accounts.vendor_verification.write"} {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", (masterRepository.VendorVerificationValues{}).ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", actor, role)
	identities := projectIdentities{"admin": auth.Identity{ID: actor, Active: true, Superuser: true}, "master": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.suppliers.read", "settings.suppliers.write", "settings.supplier_documents.read", "settings.supplier_documents.write"}}, "finance": auth.Identity{ID: actor, Active: true, Permissions: []string{"accounts.vendor_verification.view", "accounts.vendor_verification.read", "accounts.vendor_verification.write"}}, "reader": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.suppliers.read"}}, "denied": auth.Identity{ID: actor, Active: true}}
	journal := func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }
	directory := func(c database.Connection) masterContracts.VendorDirectory {
		return masterRepository.NewVendorDirectory(c)
	}
	store := &objects{data: map[string][]byte{}}
	cipher, e := documentInfra.NewDocumentCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	evidence := func(c database.Connection) masterContracts.VendorDocumentReader {
		return masterRepository.NewVendorDocumentEvidenceReader(c, cipher, documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)).WithObjects(store)
	}
	master := masterApplication.NewVendorService(masterRepository.NewVendorUnitOfWork(pool, journal), documentInfra.NewAuthBridge(identities), masterRepository.VendorValues{})
	docs := masterApplication.NewVendorDocumentService(masterRepository.NewVendorDocumentUnitOfWork(pool, directory, journal), documentInfra.NewAuthBridge(identities), cipher, masterRepository.VendorDocumentValues{}).WithObjects(store, "fixture-documents").WithInspector(documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes))
	finance := masterApplication.NewVendorVerificationService(masterRepository.NewVendorVerificationUnitOfWork(pool, directory, evidence, journal, directorUsers), documentInfra.NewAuthBridge(identities), masterRepository.VendorVerificationValues{})
	input := masterDomain.VendorCreate{Code: "NATIVE", Name: "Native fixture", Data: map[string]any{"gst_number": "fixture-gst", "qualification": "unverified", "indent_notes": "Vendor-specific note", "indent_terms": "Original vendor condition", "delivery_terms": "Vendor delivery basis"}, Reason: "Fixture setup"}
	if _, e = master.Create(ctx, "finance", input); !errors.Is(e, masterDomain.VendorForbidden) {
		t.Fatal("Finance wrote master", e)
	}
	v, e := master.Create(ctx, "master", input)
	if e != nil {
		t.Fatal(e)
	}
	if v.Source.ID != "" || v.Imported || v.Version != 1 || *v.CreatedBy != actor {
		t.Fatal("native provenance or actor wrong")
	}
	if _, e = master.Create(ctx, "master", input); !errors.Is(e, masterDomain.VendorConflict) {
		t.Fatal("duplicate code accepted", e)
	}
	if _, e = master.List(ctx, "finance"); e != nil {
		t.Fatal("Finance cannot read master", e)
	}
	st, e := finance.Status(ctx, "reader", v.ID)
	if e != nil || !st.CanInvite || st.CanOrder || len(st.MissingTypes) != 3 {
		t.Fatal("initial gate wrong", e)
	}
	var img bytes.Buffer
	_ = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	in := masterDomain.VendorDocumentInput{DocType: "pan", Filename: "fixture.png", Content: img.Bytes()}
	if _, e = docs.Upload(ctx, "finance", v.ID, in, "fixture"); !errors.Is(e, masterDomain.VendorDocumentForbidden) {
		t.Fatal("Finance uploaded without permission", e)
	}
	in.Content = []byte("<svg>fake</svg>")
	if _, e = docs.Upload(ctx, "master", v.ID, in, "fixture"); e == nil {
		t.Fatal("unsafe upload accepted")
	}
	in.Content = img.Bytes()
	stored := []masterDomain.VendorDocumentDocument{}
	for _, kind := range []string{"pan", "cheque", "gst"} {
		in.DocType = kind
		d, e := docs.Upload(ctx, "master", v.ID, in, "Fixture upload")
		if e != nil {
			t.Fatal(e)
		}
		stored = append(stored, *d)
	}
	_, data, e := docs.Preview(ctx, "finance", v.ID, stored[0].ID)
	if e != nil || !bytes.Equal(data, img.Bytes()) {
		t.Fatal("Finance preview failed", e)
	}
	if _, _, e = docs.Preview(ctx, "denied", v.ID, stored[0].ID); !errors.Is(e, masterDomain.VendorDocumentForbidden) {
		t.Fatal("unauthorized preview", e)
	}
	if _, _, e = docs.Preview(ctx, "finance", "11111111-1111-4111-8111-111111111111", stored[0].ID); !errors.Is(e, masterDomain.VendorDocumentMissing) {
		t.Fatal("cross supplier preview", e)
	}
	st, e = finance.Status(ctx, "finance", v.ID)
	if e != nil {
		t.Fatal(e)
	}
	cmd := masterDomain.VendorVerificationCommand{Fingerprint: st.Fingerprint, Reason: "Fixture Finance review", Decisions: []masterDomain.VendorVerificationDecision{}}
	for _, d := range stored {
		cmd.Decisions = append(cmd.Decisions, masterDomain.VendorVerificationDecision{DocumentID: d.ID, Decision: "accept", Comment: "Fixture inspected"})
	}
	if _, e = finance.Review(ctx, "master", v.ID, cmd); !errors.Is(e, masterDomain.VendorVerificationForbidden) {
		t.Fatal("master verified", e)
	}
	partial := cmd
	partial.Decisions = partial.Decisions[:1]
	partialStatus, partialErr := finance.Review(ctx, "finance", v.ID, partial)
	if partialErr != nil || partialStatus.CanOrder || partialStatus.Review == nil || len(partialStatus.Review.Decisions) != 1 {
		t.Fatal("document decision did not save independently", partialStatus, partialErr)
	}
	reloaded, reloadErr := finance.Status(ctx, "finance", v.ID)
	if reloadErr != nil || reloaded.CanOrder || reloaded.Review == nil || reloaded.Review.Decisions[0].DocumentID != stored[0].ID {
		t.Fatal("saved row approval did not survive reload", reloaded, reloadErr)
	}
	if _, e = finance.Review(ctx, "finance", v.ID, cmd); !errors.Is(e, masterDomain.VendorVerificationStale) {
		t.Fatal("concurrent row approval was overwritten", e)
	}
	cmd.Version = partialStatus.Review.Version
	st, e = finance.Review(ctx, "finance", v.ID, cmd)
	if e != nil || !st.CanOrder || st.Review.Version != 2 || *st.Review.UpdatedBy != actor {
		t.Fatal("review failed", e)
	}
	if _, e = finance.Review(ctx, "finance", v.ID, cmd); !errors.Is(e, masterDomain.VendorVerificationStale) {
		t.Fatal("stale review accepted", e)
	}
	ordinary, e := finance.Status(ctx, "reader", v.ID)
	if e != nil || ordinary.Review != nil || !ordinary.CanOrder {
		t.Fatal("Finance details leaked or gate incorrect", e)
	}
	manager := database.NewManager(pool)
	e = manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return masterRepository.NewVendorVerificationGate(c, directory(c), evidence(c)).RequirePurchaseOrder(ctx, v.ID)
	})
	if e != nil {
		t.Fatal("approved published gate blocked", e)
	}
	v.Data["indent_terms"] = "Revised vendor condition"
	update := masterDomain.VendorUpdate{Name: v.Name, Data: v.Data, Version: v.Version, Reason: "Fixture profile changed"}
	if _, e = master.Update(ctx, "reader", v.ID, update); !errors.Is(e, masterDomain.VendorForbidden) {
		t.Fatal("reader changed vendor terms", e)
	}
	v, e = master.Update(ctx, "master", v.ID, update)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = master.Update(ctx, "master", v.ID, update); !errors.Is(e, masterDomain.VendorStale) {
		t.Fatal("stale master write accepted", e)
	}
	rows, readErr := master.List(ctx, "reader")
	if readErr != nil {
		t.Fatal(readErr)
	}
	termsPersisted := false
	for _, row := range rows {
		if row.ID == v.ID {
			termsPersisted = row.Data["indent_terms"] == "Revised vendor condition" && row.Data["indent_notes"] == "Vendor-specific note" && row.Data["delivery_terms"] == "Vendor delivery basis"
		}
	}
	if !termsPersisted {
		t.Fatal("central vendor terms did not round-trip")
	}
	var recordedTerms, recordedActor string
	if e = pool.QueryRow(ctx, `SELECT COALESCE(snapshot->'Data',snapshot->'data')->>'indent_terms',created_by_id::text FROM supplier_changes WHERE supplier_id=$1 AND version=$2`, v.ID, v.Version).Scan(&recordedTerms, &recordedActor); e != nil || recordedTerms != "Revised vendor condition" || recordedActor != actor {
		t.Fatal("vendor term change audit missing", e)
	}
	st, e = finance.Status(ctx, "finance", v.ID)
	if e != nil || st.CanOrder || !st.CanInvite {
		t.Fatal("profile change failed to invalidate", e)
	}
	e = manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return masterRepository.NewVendorVerificationGate(c, directory(c), evidence(c)).RequirePurchaseOrder(ctx, v.ID)
	})
	if e == nil {
		t.Fatal("unverified PO gate accepted")
	}
	cmd.Version = 2
	cmd.Fingerprint = st.Fingerprint
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := finance.Review(ctx, "finance", v.ID, cmd); results <- e }()
	}
	wg.Wait()
	close(results)
	success, stale := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, masterDomain.VendorVerificationStale) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatal("concurrent approvals not serialized")
	}
	if _, e = docs.Archive(ctx, "master", v.ID, stored[0].ID, stored[0].Version, "Fixture obsolete"); e != nil {
		t.Fatal(e)
	}
	st, e = finance.Status(ctx, "finance", v.ID)
	if e != nil || st.CanOrder || len(st.MissingTypes) != 1 {
		t.Fatal("archived required evidence ignored", e)
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM supplier_verification_history").Scan(&count); e != nil || count != 3 {
		t.Fatal("review history incomplete", e)
	}
	if _, e = pool.Exec(ctx, "UPDATE supplier_verification_history SET version=99"); e == nil {
		t.Fatal("review history mutable")
	}
	// Deferred journal failure rolls back the review and its immutable history together.
	in.DocType = "pan"
	if _, e = docs.Upload(ctx, "master", v.ID, in, "Fixture replacement"); e != nil {
		t.Fatal(e)
	}
	st, e = finance.Status(ctx, "finance", v.ID)
	if e != nil {
		t.Fatal(e)
	}
	listed, e := docs.List(ctx, "finance", v.ID)
	if e != nil {
		t.Fatal(e)
	}
	cmd.Version = 3
	cmd.Fingerprint = st.Fingerprint
	cmd.Decisions = nil
	for _, d := range listed {
		if d.Active {
			cmd.Decisions = append(cmd.Decisions, masterDomain.VendorVerificationDecision{DocumentID: d.ID, Decision: "accept", Comment: "Fixture inspected"})
		}
	}
	execFixture(t, pool, `CREATE FUNCTION reject_review_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.source_module='supplier_verification' THEN RAISE EXCEPTION 'fixture'; END IF; RETURN NEW; END; $$; CREATE CONSTRAINT TRIGGER reject_review_event AFTER INSERT ON business_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_review_event();`)
	failed, commitErr := finance.Review(ctx, "finance", v.ID, cmd)
	var commitPG *pgconn.PgError
	if !errors.As(commitErr, &commitPG) || commitPG.Code != "P0001" || commitPG.Message != "fixture" || failed.Review != nil || failed.CanOrder || failed.Fingerprint != "" {
		t.Fatal("failed commit returned review success", failed, commitErr)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM supplier_verification_history").Scan(&count); e != nil || count != 3 {
		t.Fatal("failed review history leaked", e)
	}
	execFixture(t, pool, "DROP TRIGGER reject_review_event ON business_events")
	execFixture(t, pool, "DROP FUNCTION reject_review_event()")
	persisted, e := finance.Status(ctx, "finance", v.ID)
	if e != nil || persisted.Review == nil || persisted.Review.Version != 3 || persisted.CanOrder {
		t.Fatal("failed commit changed saved review", persisted, e)
	}
	if _, e = finance.Review(ctx, "admin", v.ID, cmd); !errors.Is(e, masterDomain.VendorVerificationForbidden) {
		t.Fatal("spoofed recovery administrator trusted", e)
	}
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id IN(SELECT id FROM auth.permissions WHERE code='accounts.vendor_verification.write')", role)
	if denied, e := finance.Review(ctx, "finance", v.ID, cmd); !errors.Is(e, masterDomain.VendorVerificationForbidden) || denied.Review != nil || denied.Fingerprint != "" {
		t.Fatal("revoked live write grant trusted", denied, e)
	}
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id IN(SELECT id FROM auth.permissions WHERE code='accounts.vendor_verification.read')", role)
	if _, e = finance.Status(ctx, "finance", v.ID); !errors.Is(e, masterDomain.VendorVerificationForbidden) {
		t.Fatal("revoked read grant trusted", e)
	}
	ordinary, e = finance.Status(ctx, "reader", v.ID)
	if e != nil || ordinary.Review != nil || ordinary.CanOrder {
		t.Fatal("master readiness leaked confidential review", ordinary, e)
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", actor)
	if _, e = finance.Status(ctx, "reader", v.ID); !errors.Is(e, masterDomain.VendorVerificationUnauthorized) {
		t.Fatal("inactive actor trusted", e)
	}

}
