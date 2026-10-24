package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Runs in the owned native purchasing fixture, never an operational database.
func verifyLegacyIndentTerms(t *testing.T, f purchaseFixture, documents *purchaseApp.IndentDocumentsService, original purchaseDomain.IndentDocumentsDocument) {
	t.Helper()
	ctx := context.Background()
	migration, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer migration.Close()
	if e = migration.Migrate(65); e != nil {
		t.Fatal("prepare pre-terms schema", e)
	}
	vendorID, documentID := (purchaseRepository.Values{}).ID(), (purchaseRepository.Values{}).ID()
	execFixture(t, f.pool, `INSERT INTO suppliers(id,code,name,data,is_active)VALUES($1,'LEGACYTERM','Historical vendor','{}',true)`, vendorID)
	data := original.Data
	data.Terms = nil
	data.IsCurrent = false
	data.Vendor.ID = vendorID
	data.Vendor.Name = "Historical vendor"
	data.Number = purchaseDomain.IndentDocumentsNumber(documentID)
	data.PrintRequests = []purchaseDomain.IndentDocumentsPrintRequest{}
	data.SentDate = nil
	data.SentByID = nil
	data.SentRecordedAt = nil
	body, e := json.Marshal(data)
	if e != nil {
		t.Fatal(e)
	}
	execFixture(t, f.pool, `INSERT INTO purchase_indent_documents(id,project_id,bo_id,group_id,vendor_id,scope_revision,version,data,is_active,created_by_id,updated_by_id) VALUES($1,$2,$3,$4,$5,$6,1,$7,true,$8,$8)`, documentID, original.ProjectID, original.BOID, original.GroupID, vendorID, data.ScopeRevision, body, f.actor)
	var before, after string
	if e = f.pool.QueryRow(ctx, `SELECT data::text FROM purchase_indent_documents WHERE id=$1`, documentID).Scan(&before); e != nil {
		t.Fatal(e)
	}
	if e = migration.Up(); e != nil {
		t.Fatal("legacy terms migration", e)
	}
	if e = f.pool.QueryRow(ctx, `SELECT data::text FROM purchase_indent_documents WHERE id=$1`, documentID).Scan(&after); e != nil || before != after {
		t.Fatal("migration rewrote issued source JSON", e)
	}
	workspace, e := documents.Workspace(ctx, "writer", original.ProjectID, original.BOID)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, doc := range workspace.Documents {
		if doc.ID == documentID {
			found = doc.Data.Terms != nil && doc.Data.Terms.Source == "legacy_template_v1" && len(doc.Data.Terms.Notes) == 5 && len(doc.Data.Terms.Conditions) == 16
		}
	}
	if !found {
		t.Fatal("historical terms not supplied from frozen migration evidence")
	}
	if _, e = documents.Execute(ctx, "writer", original.ProjectID, original.BOID, purchaseDomain.IndentDocumentsCommand{Action: "print", DocumentID: &documentID, Version: 1}); e != nil {
		t.Fatal("legacy reprint failed", e)
	}
	var unchanged bool
	if e = f.pool.QueryRow(ctx, `SELECT data->'terms' IS NULL FROM purchase_indent_documents WHERE id=$1`, documentID).Scan(&unchanged); e != nil || !unchanged {
		t.Fatal("reprint rewrote source terms", e)
	}
	for _, query := range []string{`UPDATE purchase_indent_legacy_terms SET terms='{}'`, `DELETE FROM purchase_indent_legacy_terms`, `TRUNCATE purchase_indent_legacy_terms`} {
		if _, e = f.pool.Exec(ctx, query); e == nil {
			t.Fatal("legacy evidence was mutable")
		}
	}
}
