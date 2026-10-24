package integration

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	materialout "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	materialapp "github.com/ranakdinesh/epcERP/internal/modules/project_items/application"
	material "github.com/ranakdinesh/epcERP/internal/modules/project_items/contracts"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"testing"
)

func TestPurchaseBODraftCompletionReopeningAndAtomicPublication(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	permissions := []string{"settings.items.read", "settings.items.write", "projects.items.read", "projects.items.write", "projects.purchase_bo.read", "projects.purchase_bo.write", "projects.purchase_bo.complete", "projects.purchase_workflow.view", "projects.purchase_workflow.read"}
	ids := projectIdentities{"writer": {ID: f.actor, Active: true, Permissions: permissions}, "manager": {ID: f.manager, Active: true, Permissions: permissions}, "outsider": {ID: f.outsider, Active: true, Permissions: permissions}}
	item, e := f.itemService(ids).Create(ctx, "writer", f.template, f.make, map[string]string{"C": "3C"}, "MODEL")
	if e != nil {
		t.Fatal(e)
	}
	factory := func(c database.Connection) purchaseRepository.BOProviders {
		p := f.materialProviders(c, purchaseRepository.NewIndentsReader(c))
		return purchaseRepository.BOProviders{Scope: p.Scope, Items: p.Items, Rules: p.Rules, Projects: p.Projects, Demands: materialout.NewDemandPort(c, p, purchaseJournal(c))}
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.BOService {
		return purchaseApp.NewBO(purchaseRepository.NewBOUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{}, purchaseRepository.BOPrinter{})
	}
	service := build(purchaseJournal)
	id := (purchaseRepository.Values{}).ID()
	lineID := (purchaseRepository.Values{}).ID()
	name := "Cable BO"
	input := purchaseDomain.BOSave{Name: &name, Version: 0, Reason: "Prepare BO", Lines: []purchaseDomain.BOInputLine{{ID: lineID}}}
	if _, e = service.Save(ctx, "outsider", f.project, id, input); !errors.Is(e, purchaseDomain.BOForbidden) {
		t.Fatal("project scope", e)
	}
	record, e := service.Save(ctx, "writer", f.project, id, input)
	if e != nil || record.Version != 1 || record.Lines[0].Quantity != nil || record.CreatedBy == nil || *record.CreatedBy != f.actor {
		t.Fatal("incomplete draft", record, e)
	}
	if _, e = service.Save(ctx, "manager", f.project, id, purchaseDomain.BOSave{Version: 1, Reason: "Attempt other preparer edit", Lines: input.Lines}); !errors.Is(e, purchaseDomain.BOForbidden) {
		t.Fatal("preparer restriction", e)
	}
	if _, e = service.Change(ctx, "writer", f.project, id, "complete", purchaseDomain.BOChange{Version: 1, Reason: "Complete missing fields"}); e == nil {
		t.Fatal("incomplete BO completed")
	}
	q, date := "10.125", "2026-10-20"
	input.Version = 1
	input.Lines[0].ItemID = &item.ID
	input.Lines[0].Quantity = &q
	input.Lines[0].RequiredDate = &date
	record, e = service.Save(ctx, "writer", f.project, id, input)
	if e != nil {
		t.Fatal(e)
	}
	second := (purchaseRepository.Values{}).ID()
	duplicate := input
	duplicate.Version = 0
	duplicate.Lines[0].ID = (purchaseRepository.Values{}).ID()
	if _, e = service.Save(ctx, "writer", f.project, second, duplicate); e == nil {
		t.Fatal("duplicate engineering BO")
	}
	if pdf, _, err := service.Print(ctx, "writer", f.project); err != nil || len(pdf) < 1000 {
		t.Fatal("scoped PDF", err)
	}
	if _, _, err := service.Print(ctx, "outsider", f.project); !errors.Is(err, purchaseDomain.BOForbidden) {
		t.Fatal("PDF project scope", err)
	}
	req := purchaseDomain.BORegisterChange{Records: []purchaseDomain.BOExpected{{ID: id, Version: record.Version}}, Reason: "Publish reviewed requirements"}
	if _, err := service.ChangeRegister(ctx, "manager", f.project, "complete", req); !errors.Is(err, purchaseDomain.BOForbidden) {
		t.Fatal("register preparer", err)
	}
	stale := req
	stale.Records = []purchaseDomain.BOExpected{{ID: id, Version: record.Version - 1}}
	if _, err := service.ChangeRegister(ctx, "writer", f.project, "complete", stale); !errors.Is(err, purchaseDomain.BOStale) {
		t.Fatal("register version", err)
	}
	blank := req
	blank.Reason = " "
	if _, err := service.ChangeRegister(ctx, "writer", f.project, "complete", blank); err == nil {
		t.Fatal("blank register comment")
	}
	transition := purchaseDomain.BOChange{Version: record.Version, Reason: "Publish reviewed requirements"}
	if _, e = build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).ChangeRegister(ctx, "writer", f.project, "complete", req); e == nil {
		t.Fatal("publication event failure committed")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM project_material_requirements").Scan(&count); e != nil || count != 0 {
		t.Fatal("demand publication rollback", count, e)
	}
	completedRegister, e := service.ChangeRegister(ctx, "writer", f.project, "complete", req)
	var completed purchaseDomain.BORecord
	if len(completedRegister.Lists) == 1 {
		completed = completedRegister.Lists[0]
	}
	if e != nil || completed.State != "completed" || completed.Version != 3 || completed.Lines[0].DemandID == nil {
		t.Fatal("BO publication", completed, e)
	}

	// Exact linkage is published under the consuming transaction, never by description or SKU.
	if _, err := purchaseRepository.NewBODemandReader(f.pool).Demand(ctx, f.project, *completed.Lines[0].DemandID); err == nil {
		t.Fatal("pool demand lookup accepted")
	}
	m := database.NewManager(f.pool)
	var captured database.Connection
	if err := m.Within(ctx, func(bound context.Context, c database.Connection) error {
		captured = c
		reader := purchaseRepository.NewBODemandReader(c)
		link, err := reader.Demand(bound, f.project, *completed.Lines[0].DemandID)
		if err != nil || link == nil || link.BOID != id || link.LineID != lineID || link.ItemID != item.ID || link.Quantity != q || link.BOVersion != completed.Version {
			t.Fatal("exact current demand linkage", link, err)
		}
		if wrong, err := reader.Demand(bound, (purchaseRepository.Values{}).ID(), *completed.Lines[0].DemandID); err != nil || wrong != nil {
			t.Fatal("cross-project demand linkage", wrong, err)
		}
		if _, err = reader.Demand(ctx, f.project, *completed.Lines[0].DemandID); err == nil {
			t.Fatal("unbound context accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := m.Within(ctx, func(bound context.Context, c database.Connection) error {
		if _, err := purchaseRepository.NewBODemandReader(captured).Demand(bound, f.project, *completed.Lines[0].DemandID); err == nil {
			t.Fatal("different transaction accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	retry, e := service.Change(ctx, "writer", f.project, id, "complete", transition)
	if e != nil || retry.Version != completed.Version {
		t.Fatal("safe retry", e)
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM project_material_requirements").Scan(&count); e != nil || count != 1 {
		t.Fatal("retry republished", count, e)
	}
	reopenedRegister, e := service.ChangeRegister(ctx, "writer", f.project, "reopen", purchaseDomain.BORegisterChange{Records: []purchaseDomain.BOExpected{{ID: id, Version: completed.Version}}, Reason: "Revise required quantities"})
	var reopened purchaseDomain.BORecord
	if len(reopenedRegister.Lists) == 1 {
		reopened = reopenedRegister.Lists[0]
	}
	if e != nil || reopened.State != "draft" || reopened.Lines[0].Snapshot.DemandVersion == nil {
		t.Fatal("reopen", e)
	}

	if err := m.Within(ctx, func(bound context.Context, c database.Connection) error {
		if link, err := purchaseRepository.NewBODemandReader(c).Demand(bound, f.project, *completed.Lines[0].DemandID); err != nil || link != nil {
			t.Fatal("reopened BO remained current linkage", link, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, e = service.Save(ctx, "writer", f.project, id, purchaseDomain.BOSave{Version: reopened.Version, Reason: "Remove published item", Lines: []purchaseDomain.BOInputLine{}}); e == nil {
		t.Fatal("published line removed")
	}
	// A separate material revision invalidates the captured BO publication version.
	materialService := materialapp.New(materialout.NewUnitOfWork(f.pool, func(c database.Connection) materialout.Providers {
		return f.materialProviders(c, purchaseRepository.NewIndentsReader(c))
	}, purchaseJournal), materialout.NewAuthBridge(ids), materialout.Values{})
	demandID := *reopened.Lines[0].DemandID
	demands, e := materialService.List(ctx, "writer", f.project)
	if e != nil || len(demands) != 1 {
		t.Fatal("demands", e)
	}
	demand := demands[0]
	_, e = materialService.Save(ctx, "writer", f.project, demandID, material.Save{ItemID: item.ID, Quantity: "11", RequiredDate: date, Active: true, Version: demand.Version, Reason: "Separate material revision"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.Change(ctx, "writer", f.project, id, "complete", purchaseDomain.BOChange{Version: reopened.Version, Reason: "Complete stale BO"}); !errors.Is(e, purchaseDomain.BOStale) {
		t.Fatal("stale publication source", e)
	}
	history, e := service.History(ctx, "writer", f.project, id)
	if e != nil || len(history) != 4 {
		t.Fatal("BO immutable history", len(history), e)
	}
	for _, sql := range []string{"DELETE FROM purchase_bo_lines", "UPDATE purchase_bo_events SET reason='rewrite'", "TRUNCATE purchase_bo_lists"} {
		if _, e = f.pool.Exec(ctx, sql); e == nil {
			t.Fatal("immutable BO evidence", sql)
		}
	}
}

func TestPurchaseRegisterRollsBackEarlierRecordWhenLaterRecordInvalid(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	perms := []string{"settings.items.read", "settings.items.write", "projects.items.read", "projects.items.write", "projects.purchase_bo.read", "projects.purchase_bo.write", "projects.purchase_bo.complete", "projects.purchase_workflow.view", "projects.purchase_workflow.read"}
	ids := projectIdentities{"writer": {ID: f.actor, Active: true, Permissions: perms}}
	item, e := f.itemService(ids).Create(ctx, "writer", f.template, f.make, map[string]string{"C": "3C"}, "")
	if e != nil {
		t.Fatal(e)
	}
	factory := func(c database.Connection) purchaseRepository.BOProviders {
		p := f.materialProviders(c, purchaseRepository.NewIndentsReader(c))
		return purchaseRepository.BOProviders{Scope: p.Scope, Items: p.Items, Rules: p.Rules, Projects: p.Projects, Demands: materialout.NewDemandPort(c, p, purchaseJournal(c))}
	}
	s := purchaseApp.NewBO(purchaseRepository.NewBOUnitOfWork(f.pool, factory, purchaseJournal), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	v := purchaseRepository.Values{}
	q, date, name := "10", "2026-10-20", "Requirements"
	first, e := s.Save(ctx, "writer", f.project, v.ID(), purchaseDomain.BOSave{Name: &name, Reason: "Prepare first", Lines: []purchaseDomain.BOInputLine{{ID: v.ID(), ItemID: &item.ID, Quantity: &q, RequiredDate: &date}}})
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.Save(ctx, "writer", f.project, v.ID(), purchaseDomain.BOSave{Name: &name, Reason: "Prepare incomplete second", Lines: []purchaseDomain.BOInputLine{{ID: v.ID()}}})
	if e != nil {
		t.Fatal(e)
	}
	req := purchaseDomain.BORegisterChange{Records: []purchaseDomain.BOExpected{{ID: first.ID, Version: first.Version}, {ID: second.ID, Version: second.Version}}, Reason: "Review both"}
	if _, e = s.ChangeRegister(ctx, "writer", f.project, "complete", req); e == nil {
		t.Fatal("incomplete second completed")
	}
	listing, e := s.List(ctx, "writer", f.project)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range listing.Lists {
		if r.State != "draft" || r.Version != 1 || r.Lines[0].DemandID != nil {
			t.Fatal("partial register write", r)
		}
	}
	var n int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM project_material_requirements").Scan(&n); e != nil || n != 0 {
		t.Fatal("partial demands", n, e)
	}
	req.Records = req.Records[:1]
	if _, e = s.ChangeRegister(ctx, "writer", f.project, "complete", req); !errors.Is(e, purchaseDomain.BOStale) {
		t.Fatal("missing record scope", e)
	}
}
