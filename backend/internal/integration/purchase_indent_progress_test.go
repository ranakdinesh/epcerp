package integration

import (
	"context"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	master "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	material "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	repo "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	app "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"testing"
)

func TestPurchaseIndentSummaryCreditsOnlyCompleteCurrentDocuments(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	v := repo.Values{}
	perms := []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", "settings.items.read", "settings.items.write", "projects.items.write", "projects.purchase_bo.read", "projects.purchase_bo.write", "projects.purchase_bo.complete", "projects.purchase_indent_groups.view", "projects.purchase_indent_groups.read", "projects.purchase_indent_groups.write", "projects.purchase_group_vendors.write", "projects.purchase_indent_documents.view", "projects.purchase_indent_documents.read", "projects.purchase_indent_documents.generate"}
	ids := projectIdentities{"writer": {ID: f.actor, Active: true, Permissions: perms}}
	item, e := f.itemService(ids).Create(ctx, "writer", f.template, f.make, map[string]string{"C": "3C"}, "MODEL")
	if e != nil {
		t.Fatal(e)
	}
	bf := func(c database.Connection) repo.BOProviders {
		p := f.materialProviders(c, repo.NewIndentsReader(c))
		return repo.BOProviders{Scope: p.Scope, Items: p.Items, Rules: p.Rules, Projects: p.Projects, Demands: material.NewDemandPort(c, p, purchaseJournal(c))}
	}
	bos := app.NewBO(repo.NewBOUnitOfWork(f.pool, bf, purchaseJournal), repo.NewAuthBridge(ids), v)
	bID, lID, gID := v.ID(), v.ID(), v.ID()
	q, date, name := "10", "2026-10-20", "Fixture items"
	b, e := bos.Save(ctx, "writer", f.project, bID, domain.BOSave{Name: &name, Reason: "Prepare items", Lines: []domain.BOInputLine{{ID: lID, ItemID: &item.ID, Quantity: &q, RequiredDate: &date}}})
	if e != nil {
		t.Fatal(e)
	}
	b, e = bos.Change(ctx, "writer", f.project, bID, "complete", domain.BOChange{Version: b.Version, Reason: "Finalise items"})
	if e != nil {
		t.Fatal(e)
	}
	vendors := []string{}
	for i := 0; i < 3; i++ {
		id := v.ID()
		vendors = append(vendors, id)
		execFixture(t, f.pool, `INSERT INTO suppliers(id,code,name,data,is_active)VALUES($1,$2,$3,'{"indent_notes":"Vendor-specific inspection note","indent_terms":"Agreed vendor conditions","delivery_terms":"Agreed delivery location"}',true)`, id, fmt.Sprintf("P%d", i), fmt.Sprintf("Vendor %d", i))
	}
	gf := func(c database.Connection) repo.IndentGroupsProviders {
		p := bf(c)
		return repo.IndentGroupsProviders{Scope: p.Scope, BO: repo.NewBOReader(c), Suppliers: master.NewVendorDirectory(c), Projects: p.Projects, Clients: f.providers(c).Clients}
	}
	groups := app.NewIndentGroups(repo.NewIndentGroupsUnitOfWork(f.pool, gf, purchaseJournal), repo.NewAuthBridge(ids), v)
	w, e := groups.Execute(ctx, "writer", f.project, bID, domain.IndentGroupsCommand{Action: "save", Version: 0, BOVersion: b.Version, Reason: "Group fixture", Groups: []domain.IndentGroupsGroupInput{{ID: gID, Name: "Fixture", LineIDs: []string{lID}, VendorIDs: vendors}}})
	if e != nil {
		t.Fatal(e)
	}
	df := func(c database.Connection) repo.IndentDocumentsProviders {
		p := gf(c)
		return repo.IndentDocumentsProviders{Scope: p.Scope, BO: p.BO, Groups: repo.NewIndentGroupsReader(c), Commands: repo.NewIndentGroupsCommander(c, p, purchaseJournal(c)), Suppliers: p.Suppliers, Projects: p.Projects, Clients: p.Clients, Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))}
	}
	docs := app.NewIndentDocuments(repo.NewIndentDocumentsUnitOfWork(f.pool, df, purchaseJournal), repo.NewAuthBridge(ids), v)
	summary := app.NewWorkspace(repo.NewWorkspaceUnitOfWork(f.pool, func(c database.Connection) projects.ContextReader { return bf(c).Projects }, func(c database.Connection) auth.AssignmentDirectory {
		return authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
	}, purchaseJournal), repo.NewAuthBridge(ids), v)
	check := func(want float64, next string) {
		t.Helper()
		c, e := summary.Context(ctx, "writer", f.project)
		if e != nil || c.Row.Progress.Percent == nil || *c.Row.Progress.Percent != want || c.Row.Progress.Next != next {
			t.Fatal("summary", c, e)
		}
	}
	check(4.1, "PUR-02")
	for i, id := range vendors {
		_, e = docs.Execute(ctx, "writer", f.project, bID, domain.IndentDocumentsCommand{Action: "generate", GroupID: &gID, VendorID: &id, GroupVersion: w.Record.Version, BOVersion: b.Version})
		if e != nil {
			t.Fatal(e)
		}
		w, e = groups.Workspace(ctx, "writer", f.project, bID)
		if e != nil {
			t.Fatal(e)
		}
		if i < 2 {
			check(4.1, "PUR-02")
		}
	}
	check(8.2, "PUR-03")
	b, e = bos.Change(ctx, "writer", f.project, bID, "reopen", domain.BOChange{Version: b.Version, Reason: "Revise material scope"})
	if e != nil {
		t.Fatal(e)
	}
	check(0, "PUR-01")
}
