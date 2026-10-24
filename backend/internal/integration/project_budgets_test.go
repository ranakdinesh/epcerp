package integration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"

	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	accessapp "github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	in "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	"net/http/httptest"
	"strings"
	"testing"
)

func budgetProviders(c database.Connection) out.Providers {
	return out.Providers{Scope: accessout.NewGate(c, accessProviders(c)), Projects: projectout.NewContextReader(c), Clients: masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c)), BO: purchaseRepository.NewBOReader(c)}
}
func TestNativeBudgetDraftPricingScopeAuditAndMigration(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	v := out.Values{}
	bo, line, role := v.ID(), v.ID(), v.ID()
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.manager)
	execFixture(t, f.pool, "UPDATE projects SET contract_value=1000 WHERE id=$1", f.project)
	execFixture(t, f.pool, "INSERT INTO purchase_bo_lists(id,project_id,preparer_id,name,state,is_active,version)VALUES($1,$2,$3,'Budget BO','draft',true,1)", bo, f.project, f.actor)
	execFixture(t, f.pool, `INSERT INTO purchase_bo_lines(id,bo_id,quantity,reference,notes,item_snapshot,sort_order,is_active)VALUES($1,$2,0.005,'','','{"sku":"FIX-CAB","description":"Budget cable","uom":"M","manufacturer":"Fixture make"}',0,true)`, line, bo)
	codes := []string{"projects.budgets.read", "projects.budgets.write", "projects.budgets.approve", "projects.team.read"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'budget specialist','fixture',true)", role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.outsider, role)
	ids := projectIdentities{"manager": auth.Identity{ID: f.manager, Active: true, Superuser: true}, "finance": auth.Identity{ID: f.outsider, Active: true, Permissions: codes}}
	build := func(j func(database.Connection) events.Writer) *app.Service {
		return app.New(out.NewUnitOfWork(f.pool, budgetProviders, j), out.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	router := chi.NewRouter()
	in.Register(router, service)
	request := func(method, project, token, body string) int {
		req := httptest.NewRequest(method, "/api/v1/projects/"+project+"/budget", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		result := httptest.NewRecorder()
		router.ServeHTTP(result, req)
		return result.Code
	}
	if status := request("GET", f.project, "manager", ""); status != 200 {
		t.Fatal("native HTTP read", status)
	}
	if status := request("GET", f.project, "", ""); status != 401 {
		t.Fatal("HTTP authentication", status)
	}
	if status := request("GET", f.otherProject, "finance", ""); status != 403 {
		t.Fatal("HTTP project privacy", status)
	}
	if status := request("POST", f.project, "manager", `{"action":"save","version":0,"reason":"Forged","created_by_id":"forged"}`); status != 422 {
		t.Fatal("HTTP audit rejected", status)
	}
	if status := request("POST", f.project, "manager", `{"action":"approve","version":0,"reason":"Premature approval"}`); status != 422 {
		t.Fatal("HTTP draft-only contract", status)
	}

	team := accessapp.New(accessout.NewUnitOfWork(f.pool, accessProviders, purchaseJournal), accessout.NewAuthBridge(ids), accessout.Values{})
	profile, e := team.Save(ctx, "manager", f.project, "", access.Input{UserID: f.outsider, BundleID: role, Responsibility: "Finance", Permissions: []string{"projects.budgets.read"}, Active: true, Reason: "Read-only budget review"})
	if e != nil {
		t.Fatal(e)
	}
	w, e := service.Workspace(ctx, "finance", f.project)
	if e != nil || w.CanWrite || w.CanApprove || len(w.Items) != 1 || w.Items[0].Snapshot.SKU != "FIX-CAB" {
		t.Fatal("read-only Finance and source snapshot", e)
	}
	if _, e = service.Workspace(ctx, "finance", f.otherProject); !errors.Is(e, domain.Forbidden) {
		t.Fatal("other project budget leaked", e)
	}
	price := "1.00"
	cmd := domain.Command{Action: "price_items", Reason: "Prepare unit budgets", Prices: []domain.Price{{BOID: bo, LineID: line, BOVersion: 1, Quantity: "0.005", UnitPrice: &price, Changed: true}}}
	if _, e = service.Execute(ctx, "finance", f.project, cmd); !errors.Is(e, domain.Forbidden) {
		t.Fatal("global write bypassed read-only profile", e)
	}
	if _, e = build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Execute(ctx, "manager", f.project, cmd); e == nil {
		t.Fatal("budget journal rejection ignored")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM project_budgets").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed draft retained", count, e)
	}
	payload, marshalError := json.Marshal(cmd)
	if marshalError != nil {
		t.Fatal(marshalError)
	}
	if status := request("POST", f.project, "manager", string(payload)); status != 200 {
		t.Fatal("native HTTP price save", status)
	}
	w, e = service.Workspace(ctx, "manager", f.project)
	if e != nil || w.Record.Version != 1 || w.Record.Approved != nil || w.CanApprove || w.Record.Data.Allocations[0].Amount != "0.01" || w.Report.PurchaseTotal != "0.01" {
		t.Fatal("native HALF_UP draft", w.Record, e)
	}
	initial := w.Record.Data.Allocations[0]
	if initial.CreatedBy == nil || *initial.CreatedBy != f.manager || w.Record.Data.Heads[0].CreatedBy == nil {
		t.Fatal("missing normalized child attribution")
	}
	if _, e = service.Execute(ctx, "manager", f.project, domain.Command{Action: "approve", Version: 1, Reason: "Premature approval"}); e == nil {
		t.Fatal("draft approved without Finance/Director")
	}
	execFixture(t, f.pool, "UPDATE purchase_bo_lines SET quantity=10 WHERE id=$1", line)
	execFixture(t, f.pool, "UPDATE purchase_bo_lists SET version=version+1 WHERE id=$1", bo)
	cmd.Version = 1
	if _, e = service.Execute(ctx, "manager", f.project, cmd); !errors.Is(e, domain.Stale) {
		t.Fatal("stale source quantity/version accepted", e)
	}
	cmd.Prices[0].BOVersion = 2
	cmd.Prices[0].Quantity = "10"
	cmd.Prices[0].Changed = false
	w, e = service.Execute(ctx, "manager", f.project, cmd)
	if e != nil || w.Record.Version != 2 || w.Record.Data.Allocations[0].Amount != "10.00" || w.Record.Data.Allocations[0].CreatedAt != initial.CreatedAt {
		t.Fatal("reconciled unit rate and audit identity", e)
	}
	if _, e = service.Execute(ctx, "manager", f.project, cmd); !errors.Is(e, domain.Stale) {
		t.Fatal("stale budget version accepted", e)
	}
	// Upgrade the scoped profile, never rely on the user's other global role.
	_, e = team.Save(ctx, "manager", f.project, profile.ID, access.Input{UserID: f.outsider, BundleID: role, Responsibility: "Finance", Permissions: codes, Active: true, Version: 1, Reason: "Allow budget preparation"})
	if e != nil {
		t.Fatal(e)
	}
	w, e = service.Workspace(ctx, "finance", f.project)
	if e != nil || !w.CanWrite || w.CanApprove {
		t.Fatal("independent scoped write/approve capabilities", e)
	}
	execFixture(t, f.pool, `CREATE FUNCTION reject_budget_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced budget commit rejection';END $$`)
	execFixture(t, f.pool, `CREATE CONSTRAINT TRIGGER reject_budget_commit AFTER UPDATE ON project_budgets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_budget_commit()`)
	cmd.Version = 2
	cmd.Reason = "Commit failure test"
	failed, e := service.Execute(ctx, "finance", f.project, cmd)
	if e == nil || failed.Record != nil {
		t.Fatal("failed commit returned budget data", e)
	}
	execFixture(t, f.pool, "DROP TRIGGER reject_budget_commit ON project_budgets")
	w, e = service.Workspace(ctx, "manager", f.project)
	if e != nil || w.Record.Version != 2 || len(w.History) != 2 {
		t.Fatal("native commit rollback", e)
	}
	if _, e = f.pool.Exec(ctx, "UPDATE project_budget_events SET reason='rewrite'"); e == nil {
		t.Fatal("mutable budget evidence")
	}
	if _, e = f.pool.Exec(ctx, "DELETE FROM project_budget_allocations"); e == nil {
		t.Fatal("allocation history deleted")
	}
	if _, e = f.pool.Exec(ctx, "UPDATE project_budget_allocations SET bo_id=$1", v.ID()); e == nil {
		t.Fatal("allocation source retargeted")
	}
	if _, e = f.pool.Exec(ctx, "UPDATE project_budgets SET state='approved',version=version+1"); e == nil {
		t.Fatal("SQL invented an approved baseline")
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='project_budgets'").Scan(&count); e != nil || count != 2 {
		t.Fatal("local and central budget events", count, e)
	}
	m, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Steps(-(schema.LatestVersion - 37)); e == nil || !strings.Contains(e.Error(), "populated budget draft") {
		t.Fatal("populated budget migration discarded evidence", e)
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM project_budgets").Scan(&count); e != nil || count != 1 {
		t.Fatal("rollback guard lost draft", count, e)
	}
}
