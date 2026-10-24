package integration

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"

	materialout "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"testing"
	"time"
)

func TestPurchaseHoldOpenProjectScopeReleaseAuditAndEvaluation(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	permissions := []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", "projects.purchase_holds.read", "projects.purchase_holds.write", "projects.purchase_holds.release"}
	ids := projectIdentities{"writer": {ID: f.actor, Active: true, Permissions: permissions}, "outsider": {ID: f.outsider, Active: true, Permissions: permissions}}
	factory := func(c database.Connection) purchaseRepository.HoldsProviders {
		p := f.materialProviders(c, &materialCommitments{quantity: "0"})
		return purchaseRepository.HoldsProviders{Scope: p.Scope.(*purchaseRepository.WorkspaceAccessGate), Categories: masterRepository.NewCategoryDirectory(c), Demands: materialout.NewDemandPort(c, p, purchaseJournal(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))}
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.HoldsService {
		return purchaseApp.NewHolds(purchaseRepository.NewHoldsUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	}
	service := build(purchaseJournal)
	input := purchaseDomain.HoldsCreate{Scope: "project", DemandIDs: []string{}, Reason: "Client purchase suspension", Evidence: "Reviewed instruction", StartedAt: time.Now().UTC().Add(-time.Hour), DoerID: f.actor, ResolverID: f.outsider}
	// A manager may assign an active resolver from outside the project, as requested by the owner.
	if _, e := service.Create(ctx, "outsider", f.project, input); !errors.Is(e, purchaseDomain.HoldsForbidden) {
		t.Fatal("outsider writes", e)
	}
	execFixture(t, f.pool, "UPDATE projects SET lifecycle_status='on_hold' WHERE id=$1", f.project)
	if _, e := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Create(ctx, "writer", f.project, input); e == nil {
		t.Fatal("hold audit failure committed")
	}
	var count int
	if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_holds").Scan(&count); e != nil || count != 0 {
		t.Fatal("hold rollback", count, e)
	}
	record, e := service.Create(ctx, "writer", f.project, input)
	if e != nil || record.Version != 1 || record.CreatedBy == nil || *record.CreatedBy != f.actor {
		t.Fatal("on-hold project recording", e)
	}
	evaluation, e := purchaseRepository.NewHoldsEvaluator(f.pool).Evaluate(ctx, f.project, "", "CAB")
	if e != nil || len(evaluation.Holds) != 1 || evaluation.HoldMinutes < 59 {
		t.Fatal("active hold", evaluation, e)
	}
	decision := purchaseDomain.HoldsDecision{Version: 1, Reason: "Client released suspension", Evidence: "Client release instruction"}
	released, e := service.Release(ctx, "writer", f.project, record.ID, decision)
	if e != nil || released.Active || released.Version != 2 || released.Data.ReleasedAt == nil {
		t.Fatal("hold release", e)
	}
	if _, e = service.Release(ctx, "writer", f.project, record.ID, decision); !errors.Is(e, purchaseDomain.HoldsStale) {
		t.Fatal("stale release", e)
	}
	decision.Version = 2
	if _, e = service.Release(ctx, "writer", f.project, record.ID, decision); e == nil {
		t.Fatal("double release")
	}
	evaluation, e = purchaseRepository.NewHoldsEvaluator(f.pool).Evaluate(ctx, f.project, "", "CAB")
	if e != nil || len(evaluation.Holds) != 0 || evaluation.HoldMinutes < 59 {
		t.Fatal("released elapsed evidence", e)
	}
	history, e := service.History(ctx, "writer", f.project, record.ID)
	if e != nil || len(history) != 2 || history[0].Before == nil || !history[0].Before.Active {
		t.Fatal("hold history", e)
	}
	if _, e = f.pool.Exec(ctx, "UPDATE purchase_hold_events SET reason='rewrite'"); e == nil {
		t.Fatal("mutable hold evidence")
	}
	execFixture(t, f.pool, "UPDATE projects SET lifecycle_status='closed' WHERE id=$1", f.project)
	if _, e = service.Create(ctx, "writer", f.project, input); !errors.Is(e, purchaseDomain.HoldsForbidden) {
		t.Fatal("closed project hold", e)
	}
}
