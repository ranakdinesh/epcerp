package integration

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	repository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"testing"
)

// Exercise the new handoff against actual approved fixture data, then roll back
// the test-only outer transaction so the original sourcing regression can proceed.
func verifyNativeAwardHandoff(t *testing.T, f purchaseFixture, ids projectIdentities, p repository.IndentsFactory, wp repository.WorkflowFactory, finance repository.QuotationApprovalsFactory, attempt string) {
	t.Helper()
	ctx := t.Context()
	tx, e := f.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	providers := func(c database.Connection) repository.AwardsProviders {
		x := p(c)
		return repository.AwardsProviders{Indents: x, Workflow: wp(c), Approvals: finance(c), Sourcing: repository.SourcingProviders{Projects: x.Projects, Purchase: x.Scope, Access: accessout.NewGate(c, accessProviders(c)), Users: x.Users, Work: repository.NewWorkflowSourceReader(c, wp(c)), Indents: repository.NewIndentsReader(c), Items: x.Items, Suppliers: masterRepository.NewVendorDirectory(c), BO: repository.NewBODemandReader(c), Quotes: repository.NewQuotationApprovalsApprovedReader(c, finance(c))}}
	}
	verdict, e := repository.NewClientRulesEvaluator(tx).Evaluate(ctx, domain.ClientRulesEvaluation{ClientID: f.client, ProjectID: f.project, Category: "CAB", Manufacturer: f.make, Today: domain.OrdersToday((repository.Values{}).Now())})
	if e != nil || verdict.Status != "eligible" || len(verdict.Rules) != 0 || len(verdict.Reasons) != 0 {
		t.Fatal("unconfigured make policy must not block or invent review evidence", e, verdict)
	}
	s := application.NewAwards(repository.NewAwardsUnitOfWork(awardTestBeginner{tx}, providers, purchaseJournal), repository.NewAuthBridge(ids), repository.Values{})
	w, e := s.Workspace(ctx, "loi-manager", f.project)
	if e != nil || len(w.Rows) == 0 {
		t.Fatal("approved award listing", e, w)
	}
	q := domain.AwardCommand{AttemptID: attempt, DoerID: f.actor, ResolverID: f.manager, PlannedDate: domain.OrdersToday((repository.Values{}).Now()), Reason: "Continue actual Finance-approved group"}
	if _, e = s.Execute(ctx, "loi-writer", f.project, q); e == nil {
		t.Fatal("ungranted award permission accepted")
	}
	h, e := s.Execute(ctx, "loi-manager", f.project, q)
	if e != nil || len(h.Packages) == 0 {
		t.Fatal("atomic approved award handoff", e, h)
	}
	// The exact approval links only its selected vendor to this project item;
	// an active, qualified unrelated vendor with no master links stays ineligible.
	unrelated := (repository.Values{}).ID()
	if _, e = tx.Exec(ctx, `INSERT INTO suppliers(id,code,name,data,is_active)VALUES($1,$2,'Unrelated vendor','{"qualification":"approved","item_ids":[],"category_ids":[]}',true)`, unrelated, "UNLINKED-"+unrelated[:12]); e != nil {
		t.Fatal(e)
	}
	var choices []domain.SourcingSupplier
	e = database.NewManager(awardTestBeginner{tx}).Within(ctx, func(ctx context.Context, c database.Connection) error {
		var err error
		choices, err = repository.NewSourcingContext(providers(c).Sourcing).Suppliers(ctx, f.project, h.Packages[0].ID)
		return err
	})
	if e != nil {
		t.Fatal(e)
	}
	var approvedVendor string
	if e = tx.QueryRow(ctx, `SELECT data->'offers'->0->>'supplier_id' FROM purchase_sourcing_cases WHERE project_id=$1 AND package_id=$2`, f.project, h.Packages[0].ID).Scan(&approvedVendor); e != nil {
		t.Fatal(e)
	}
	foundEligible := false
	for _, choice := range choices {
		if choice.ID == approvedVendor {
			foundEligible = choice.Eligible
		}
		if choice.ID == unrelated && choice.Eligible {
			t.Fatal("Finance approval linked an unrelated vendor")
		}
	}
	if !foundEligible {
		t.Fatal("exact approved vendor remained unlinked")
	}
	verifyNativeLOIDocuments(t, f, ids, awardTestBeginner{tx}, providers, attempt)
	if _, e = tx.Exec(ctx, "SET CONSTRAINTS ALL IMMEDIATE"); e != nil {
		t.Fatal("award constraints", e)
	}
	again, e := s.Execute(ctx, "loi-manager", f.project, q)
	if e != nil || again.ID != h.ID || len(again.Packages) != len(h.Packages) {
		t.Fatal("duplicate handoff", e)
	}
	current, e := s.Workspace(ctx, "loi-manager", f.project)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, r := range current.Rows {
		if r.ID == attempt {
			found = r.State == "loi_pending" && len(r.Packages) == len(h.Packages)
		}
	}
	if !found {
		t.Fatal("saved award state missing")
	}
	var count int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM purchase_award_handoffs WHERE project_id=$1 AND attempt_id=$2", f.project, attempt).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate award records", e, count)
	}
}

type awardTestBeginner struct{ tx pgx.Tx }

func (b awardTestBeginner) BeginTx(ctx context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return b.tx.Begin(ctx)
}
