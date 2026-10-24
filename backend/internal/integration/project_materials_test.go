package integration

import (
	"context"
	"errors"
	authInfrastructure "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"

	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	out "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/project_items/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/domain"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"

	"testing"
)

type materialCommitments struct{ quantity string }

func (c *materialCommitments) Committed(context.Context, string, string) (string, error) {
	return c.quantity, nil
}
func purchaseJournal(c database.Connection) events.Writer {
	return eventapp.NewRecorder(eventout.NewJournal(c))
}
func (f purchaseFixture) itemService(ids projectIdentities) *masterApplication.ItemService {
	return masterApplication.NewItemService(masterRepository.NewItemUnitOfWork(f.pool, func(c database.Connection) masterContracts.CategoryItemReader {
		return masterRepository.NewCategoryDirectory(c)
	}, func(c database.Connection) masterContracts.TemplateItemReader {
		return masterRepository.NewTemplateItemReader(c)
	}, func(c database.Connection) masterContracts.ManufacturerItemReader {
		return masterRepository.NewManufacturerItemReader(c)
	}, purchaseJournal), authInfrastructure.NewAuthBridge(ids), masterRepository.ItemValues{})
}
func (f purchaseFixture) materialProviders(c database.Connection, committed out.Commitments) out.Providers {
	return out.Providers{Scope: purchaseRepository.NewWorkspaceAccessGate(c, projectout.NewContextReader(c), masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))), Items: masterRepository.NewItemReader(c, masterRepository.NewCategoryDirectory(c), masterRepository.NewTemplateItemReader(c), masterRepository.NewManufacturerItemReader(c)), Rules: purchaseRepository.NewClientRulesEvaluator(c), Projects: projectout.NewContextReader(c), Commitments: committed}
}
func TestProjectMaterialsPublicationAlternativesVersionsAndCommitments(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	permissions := []string{"settings.items.read", "settings.items.write", "projects.items.read", "projects.items.write", "projects.purchase_workflow.view", "projects.purchase_workflow.read"}
	ids := projectIdentities{"writer": {ID: f.actor, Active: true, Permissions: permissions}, "outsider": {ID: f.outsider, Active: true, Permissions: permissions}, "reader": {ID: f.manager, Active: true, Permissions: []string{"projects.items.read", "projects.purchase_workflow.view", "projects.purchase_workflow.read"}}}
	bank := f.itemService(ids)
	item, e := bank.Create(ctx, "writer", f.template, f.make, map[string]string{"C": "3C"}, "MODEL1")
	if e != nil {
		t.Fatal(e)
	}
	second := (out.Values{}).ID()
	execFixture(t, f.pool, `INSERT INTO manufacturers(id,code,name,data,is_active)VALUES($1,'SECOND','Second make','{"eligible":["CAB"],"notes":""}',true)`, second)
	alternate, e := bank.Create(ctx, "writer", f.template, second, map[string]string{"C": "3C"}, "MODEL2")
	if e != nil {
		t.Fatal(e)
	}
	committed := &materialCommitments{quantity: "0"}
	factory := func(c database.Connection) out.Providers { return f.materialProviders(c, committed) }
	build := func(j func(database.Connection) events.Writer) *app.Service {
		return app.New(out.NewUnitOfWork(f.pool, factory, j), out.NewAuthBridge(ids), out.Values{})
	}
	service := build(purchaseJournal)

	choices, e := service.Catalogue(ctx, "writer", f.project)
	if e != nil || len(choices) != 2 || choices[0].SKU == "" || choices[0].Category != "CAB" || choices[0].Configuration != "LTPC" {
		t.Fatal("project catalogue selection", e, choices)
	}
	if _, e = service.Catalogue(ctx, "outsider", f.project); !errors.Is(e, domain.Forbidden) {
		t.Fatal("outside project catalogue access", e)
	}
	execFixture(t, f.pool, "UPDATE manufacturers SET is_active=false WHERE id=$1", second)
	choices, e = service.Catalogue(ctx, "writer", f.project)
	if e != nil || len(choices) != 1 || choices[0].ID != item.ID {
		t.Fatal("retired make selectable", e)
	}
	execFixture(t, f.pool, "UPDATE manufacturers SET is_active=true WHERE id=$1", second)
	input := domain.Save{ItemID: item.ID, Quantity: "10.125", RequiredDate: "2026-10-12", Allocations: []domain.Allocation{{Location: "Site A", Quantity: "6"}, {Location: "Site B", Quantity: "4.125"}}, Alternatives: []string{alternate.ID}, Active: true, Version: 1, Reason: "Reviewed BO requirement"}
	if _, e := service.Save(ctx, "outsider", f.project, "", input); !errors.Is(e, domain.Forbidden) {
		t.Fatal("global permission escaped project scope", e)
	}
	if _, e := service.Save(ctx, "reader", f.project, "", input); !errors.Is(e, domain.Forbidden) {
		t.Fatal("reader mutated material", e)
	}
	if _, e := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Save(ctx, "writer", f.project, "", input); e == nil {
		t.Fatal("event failure committed requirement")
	}
	var count int
	if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM project_material_requirements").Scan(&count); e != nil || count != 0 {
		t.Fatal("material rollback", e)
	}
	record, e := service.Save(ctx, "writer", f.project, "", input)
	if e != nil || record.Quantity != "10.125" || record.Version != 1 || record.Snapshot.SKU != item.SKU || record.CreatedBy == nil || *record.CreatedBy != f.actor {
		t.Fatal("exact demand publication", record, e)
	}
	if _, e := service.Save(ctx, "writer", f.project, "", input); e == nil {
		t.Fatal("duplicate SKU requirement")
	}
	engineering := input
	engineering.ItemID = alternate.ID
	engineering.Alternatives = []string{}
	if _, e := service.Save(ctx, "writer", f.project, "", engineering); e == nil {
		t.Fatal("duplicate engineering requirement")
	}
	replacement := input
	replacement.ItemID = alternate.ID
	replacement.Alternatives = []string{}
	if _, e := service.Save(ctx, "writer", f.project, record.ID, replacement); e == nil {
		t.Fatal("immutable material SKU replaced")
	}
	committed.quantity = "10"
	input.Quantity = "9.999"
	input.Allocations = []domain.Allocation{}
	if _, e := service.Save(ctx, "writer", f.project, record.ID, input); e == nil {
		t.Fatal("below committed quantity")
	}
	input.Quantity = "10.125"
	input.Active = false
	if _, e := service.Save(ctx, "writer", f.project, record.ID, input); e == nil {
		t.Fatal("committed requirement deactivated")
	}
	input.Active = true
	input.Quantity = "12"
	input.Version = record.Version
	revised, e := service.Save(ctx, "writer", f.project, record.ID, input)
	if e != nil || revised.Version != 2 || revised.Quantity != "12.000" {
		t.Fatal("material revision", e)
	}
	if _, e := service.Save(ctx, "writer", f.project, record.ID, input); !errors.Is(e, domain.Stale) {
		t.Fatal("stale material revision", e)
	}
	history, e := service.History(ctx, "reader", f.project, record.ID)
	if e != nil || len(history) != 2 || history[0].Before == nil || history[0].Before.Quantity != "10.125" {
		t.Fatal("immutable snapshots", e)
	}
	for _, sql := range []string{"UPDATE project_material_events SET reason='rewrite'", "DELETE FROM project_material_events", "TRUNCATE project_material_events", "DELETE FROM project_material_requirements", "UPDATE project_material_requirements SET item_id='00000000-0000-0000-0000-000000000000'"} {
		if _, e := f.pool.Exec(ctx, sql); e == nil {
			t.Fatal("immutable material evidence", sql)
		}
	}
	if _, e := service.History(ctx, "reader", f.otherProject, record.ID); !errors.Is(e, domain.Missing) {
		t.Fatal("history crossed projects", e)
	}
	// Published writer refuses a pool or independent connection outside the consumer transaction.
	if _, e := out.NewDemandPort(f.pool, f.materialProviders(f.pool, committed), purchaseJournal(f.pool)).Save(ctx, f.project, record.ID, input, f.actor); e == nil {
		t.Fatal("unbound publication accepted")
	}
	execFixture(t, f.pool, "UPDATE catalogue_items SET status='retired',is_active=false WHERE id=$1", item.ID)
	input.Version = revised.Version
	input.Quantity = "13"
	if _, e := service.Save(ctx, "writer", f.project, record.ID, input); e == nil {
		t.Fatal("retired item gained new demand")
	}
	input.Quantity = "11"
	if _, e := service.Save(ctx, "writer", f.project, record.ID, input); e != nil {
		t.Fatal("existing retired snapshot could not be reduced above commitments", e)
	}
	m, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(28); e == nil {
		t.Fatal("populated demand rollback erased evidence")
	}
}
