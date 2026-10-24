package integration

import (
	"bytes"
	infra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	repo "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	app "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"strings"
	"testing"
)

func verifyNativeLOIDocuments(t *testing.T, f purchaseFixture, ids projectIdentities, txBeginner awardTestBeginner, providers func(database.Connection) repo.AwardsProviders, attempt string) {
	t.Helper()
	ctx := t.Context()
	objects := &nativePOObjects{data: map[string][]byte{}}
	cipher, e := infra.NewDocumentCipher(strings.Repeat("12", 32))
	if e != nil {
		t.Fatal(e)
	}
	u := repo.NewAwardsUnitOfWork(txBeginner, providers, purchaseJournal)
	s := app.NewLOIDocuments(u, repo.NewAuthBridge(ids), repo.Values{}, repo.LOIPDFPrinter{}, repo.LOIFingerprint{}, cipher, objects, "private-test")
	w, e := s.Workspace(ctx, "loi-manager", f.project, attempt)
	if e != nil || len(w.Documents) != 0 || !w.CanGenerate {
		t.Fatal("LOI empty state", e, w)
	}
	q := domain.LOICommand{Reference: "LOI/26-27/VTPL/NATIVE", Date: domain.SourcingToday((repo.Values{}).Now()), Format: "cable"}
	if _, e = s.Generate(ctx, "loi-writer", f.project, attempt, q); e == nil {
		t.Fatal("ungranted LOI generation")
	}
	d, e := s.Generate(ctx, "loi-manager", f.project, attempt, q)
	if e != nil || d.Version != 1 {
		t.Fatal("native LOI generation", e)
	}
	data, _, e := s.Download(ctx, "loi-manager", f.project, attempt, d.ID)
	if e != nil || !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatal("native encrypted LOI download", e)
	}
	for _, stored := range objects.data {
		if bytes.HasPrefix(stored, []byte("%PDF")) {
			t.Fatal("unencrypted LOI object")
		}
	}
	again, e := s.Generate(ctx, "loi-manager", f.project, attempt, q)
	if e != nil || again.ID != d.ID {
		t.Fatal("LOI retry duplicated document", e)
	}
	confirm := domain.LOIReviewCommand{DetailsCorrect: true, PricesCorrect: true, FormatCorrect: true}
	if _, e = s.CompleteReview(ctx, "loi-writer", f.project, attempt, d.ID, confirm); e == nil {
		t.Fatal("ungranted user completed LOI review")
	}
	if _, e = s.CompleteReview(ctx, "loi-manager", f.otherProject, attempt, d.ID, confirm); e == nil {
		t.Fatal("review crossed project scope")
	}
	if _, e = s.CompleteReview(ctx, "loi-manager", f.project, attempt, d.ID, domain.LOIReviewCommand{}); e == nil {
		t.Fatal("unchecked review completed")
	}
	review, e := s.CompleteReview(ctx, "loi-manager", f.project, attempt, d.ID, confirm)
	if e != nil || review == nil || review.DocumentID != d.ID || review.CreatedBy == "" {
		t.Fatal("persisted LOI review", e, review)
	}
	retry, e := s.CompleteReview(ctx, "loi-manager", f.project, attempt, d.ID, confirm)
	if e != nil || retry.ID != review.ID {
		t.Fatal("LOI review retry duplicated audit", e)
	}
	awards := app.NewAwards(u, repo.NewAuthBridge(ids), repo.Values{})
	grid, e := awards.Workspace(ctx, "loi-manager", f.project)
	if e != nil {
		t.Fatal(e)
	}
	visible := false
	for _, row := range grid.Rows {
		if row.ID == attempt {
			visible = row.LOIReviewComplete
			if row.State != "loi_pending" {
				t.Fatal("PDF review fabricated LOI dispatch")
			}
		}
	}
	if !visible {
		t.Fatal("completed review absent from award grid")
	}
	q.Notes = "Client-specific inspection requirements"
	newer, e := s.Generate(ctx, "loi-manager", f.project, attempt, q)
	if e != nil || newer.Version != 2 || newer.ID == d.ID {
		t.Fatal("immutable LOI version", e)
	}
	if newer.Review != nil {
		t.Fatal("new PDF inherited prior review")
	}
	if _, e = s.CompleteReview(ctx, "loi-manager", f.project, attempt, d.ID, confirm); e == nil {
		t.Fatal("superseded PDF review accepted")
	}
	old, _, e := s.Download(ctx, "loi-manager", f.project, attempt, d.ID)
	if e != nil || !bytes.Equal(old, data) {
		t.Fatal("historical LOI changed", e)
	}
	if _, _, e = s.Download(ctx, "loi-manager", f.otherProject, attempt, d.ID); e == nil {
		t.Fatal("LOI crossed project scope")
	}
	badJournal := repo.NewAwardsUnitOfWork(txBeginner, providers, func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	bad := app.NewLOIDocuments(badJournal, repo.NewAuthBridge(ids), repo.Values{}, repo.LOIPDFPrinter{}, repo.LOIFingerprint{}, cipher, objects, "private-test")
	if _, e = bad.CompleteReview(ctx, "loi-manager", f.project, attempt, newer.ID, confirm); e == nil {
		t.Fatal("unaudited review committed")
	}
	q.Notes = "Must roll back if immutable audit fails"
	if _, e = bad.Generate(ctx, "loi-manager", f.project, attempt, q); e == nil {
		t.Fatal("unaudited LOI generation committed")
	}
	w, e = s.Workspace(ctx, "loi-manager", f.project, attempt)
	if e != nil || len(w.Documents) != 2 {
		t.Fatal("LOI generation rollback", e)
	}
	if w.Documents[0].Review != nil || w.Documents[1].Review == nil || w.Documents[1].Review.ID != review.ID {
		t.Fatal("review rollback or historical review lost")
	}
	// Immutable metadata and generation/read audits are checked at commit.
	if _, e = txBeginner.tx.Exec(ctx, "SET CONSTRAINTS ALL IMMEDIATE"); e != nil {
		t.Fatal("LOI audit constraints", e)
	}
	if _, e = txBeginner.tx.Exec(ctx, "SET CONSTRAINTS ALL DEFERRED"); e != nil {
		t.Fatal(e)
	}
}
