package integration

import (
	"context"
	"errors"
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
	"strings"
	"testing"
	"time"
)

func TestSupplierImportDocumentsAndRollback(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actor := "135bcaf6-e4ec-4809-b7e9-42b94c332891"
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'supplier_fixture','fixture@example.invalid','fixture',true,false)", actor)
	identities := projectIdentities{"importer": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.suppliers.import", "settings.suppliers.read", "settings.supplier_documents.import", "settings.supplier_documents.read"}}, "reader": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.suppliers.read", "settings.supplier_documents.read"}}, "denied": auth.Identity{ID: actor, Active: true}}
	journal := func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }
	master := masterApplication.NewVendorService(masterRepository.NewVendorUnitOfWork(pool, journal), documentInfra.NewAuthBridge(identities), masterRepository.VendorValues{})
	at := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	active := true
	in := masterDomain.VendorInput{Source: masterDomain.VendorSource{ID: "c222ad10-b1c6-4f14-b249-f5f08f351c54", Version: 3, CreatedAt: at, UpdatedAt: at}, Code: "FIXTURE", Name: "Supplier fixture", Data: map[string]any{"category_ids": []any{}, "item_ids": []any{}, "qualification": "unverified", "payment_terms": map[string]any{"advance_pct": 25, "delivery_pct": 75}}, Active: &active}
	if _, e := master.Import(ctx, "reader", in); !errors.Is(e, masterDomain.VendorForbidden) {
		t.Fatal("reader imported supplier", e)
	}
	if _, e := master.List(ctx, ""); !errors.Is(e, masterDomain.VendorUnauthorized) {
		t.Fatal("anonymous read", e)
	}
	v, e := master.Import(ctx, "importer", in)
	if e != nil {
		t.Fatal(e)
	}
	if v.ID == in.Source.ID || v.Source.Version != 3 || *v.CreatedBy != actor {
		t.Fatal("native identity/provenance/actor lost")
	}
	again, e := master.Import(ctx, "importer", in)
	if e != nil || again.ID != v.ID {
		t.Fatal("supplier retry duplicated", e)
	}
	in.Name = "Different fixture"
	if _, e = master.Import(ctx, "importer", in); !errors.Is(e, masterDomain.VendorConflict) {
		t.Fatal("changed source accepted", e)
	}
	in.Name = "Supplier fixture"
	store := &objects{data: map[string][]byte{}}
	cipher, e := documentInfra.NewDocumentCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	docs := masterApplication.NewVendorDocumentService(masterRepository.NewVendorDocumentUnitOfWork(pool, func(c database.Connection) masterContracts.VendorDirectory {
		return masterRepository.NewVendorDirectory(c)
	}, journal), documentInfra.NewAuthBridge(identities), cipher, masterRepository.VendorDocumentValues{}).WithObjects(store, "fixture-documents")
	input := masterDomain.VendorDocumentInput{Source: masterDomain.VendorDocumentSource{ID: "59532c4d-74a2-4c6c-bef6-67ce3cdaaa4b", CreatedAt: at, UpdatedAt: at}, DocType: "gst", Title: "Fixture", Filename: "fixture.pdf", ContentType: "application/pdf", Content: []byte("Fixture evidence"), Active: &active}
	if _, e := docs.Import(ctx, "reader", v.ID, input); !errors.Is(e, masterDomain.VendorDocumentForbidden) {
		t.Fatal("reader imported document", e)
	}
	saved, e := docs.Import(ctx, "importer", v.ID, input)
	if e != nil {
		t.Fatal(e)
	}
	if saved.ID == input.Source.ID || !saved.Encrypted || string(saved.Ciphertext) == string(input.Content) || *saved.CreatedBy != actor {
		t.Fatal("document storage mismatch")
	}
	retry, e := docs.Import(ctx, "importer", v.ID, input)
	if e != nil || retry.ID != saved.ID {
		t.Fatal("document retry duplicated", e)
	}
	_, bytes, e := docs.Download(ctx, "reader", v.ID, saved.ID)
	if e != nil || string(bytes) != string(input.Content) {
		t.Fatal("download integrity mismatch", e)
	}
	if _, _, e = docs.Download(ctx, "denied", v.ID, saved.ID); !errors.Is(e, masterDomain.VendorDocumentForbidden) {
		t.Fatal("download not protected", e)
	}
	if _, _, e = docs.Download(ctx, "reader", "11111111-1111-4111-8111-111111111111", saved.ID); !errors.Is(e, masterDomain.VendorDocumentMissing) {
		t.Fatal("cross-supplier download", e)
	}
	input.Source.ID = "8f6c2757-cb1a-4772-a63f-873b146477a3"
	inactive := false
	input.Active = &inactive
	archived, e := docs.Import(ctx, "importer", v.ID, input)
	if e != nil || archived.Active {
		t.Fatal("archival status lost", e)
	}
	if _, _, e = docs.Download(ctx, "reader", v.ID, archived.ID); !errors.Is(e, masterDomain.VendorDocumentMissing) {
		t.Fatal("ordinary reader accessed archive", e)
	}
	if _, _, e = docs.Download(ctx, "importer", v.ID, archived.ID); e != nil {
		t.Fatal("operator archive integrity check", e)
	}
	listed, e := docs.List(ctx, "reader", v.ID)
	if e != nil || len(listed) != 2 || len(listed[0].Ciphertext) != 0 {
		t.Fatal("metadata list leaked bytes", e)
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module IN ('suppliers','supplier_documents')").Scan(&count); e != nil || count != 3 {
		t.Fatal("central event or retry mismatch", e)
	}
	// A deferred journal failure must roll back both evidence and source event.
	execFixture(t, pool, `CREATE FUNCTION reject_supplier_document_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END; $$; CREATE CONSTRAINT TRIGGER reject_supplier_document_event AFTER INSERT ON business_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_supplier_document_event();`)
	input.Source.ID = "d222ad10-b1c6-4f14-b249-f5f08f351c54"
	if _, e = docs.Import(ctx, "importer", v.ID, input); e == nil {
		t.Fatal("deferred failure accepted")
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM supplier_documents").Scan(&count); e != nil || count != 2 {
		t.Fatal("failed event leaked evidence", e)
	}
}
