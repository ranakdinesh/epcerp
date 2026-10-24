package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type purchaseRejectJournal struct{}

func (purchaseRejectJournal) Record(context.Context, events.Event, []events.Target) error {
	return errors.New("fixture publication failed")
}
func TestPurchaseDashboardScopeOwnershipAndAtomicHistory(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	v := purchaseRepository.Values{}
	admin, pm, a, b, outsider, client, role, p1, p2 := v.ID(), v.ID(), v.ID(), v.ID(), v.ID(), v.ID(), v.ID(), v.ID(), v.ID()
	for i, id := range []string{admin, pm, a, b, outsider} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,'fixture',true,$4)", id, []string{"admin", "pm", "a", "b", "outsider"}[i], []string{"admin@fixture.invalid", "pm@fixture.invalid", "a@fixture.invalid", "b@fixture.invalid", "x@fixture.invalid"}[i], i == 0)
	}
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'purchaser','fixture',true)", role)
	perms := []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read"}
	for _, code := range perms {
		pid := v.ID()
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", pid, code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)VALUES($1,$2)", role, pid)
	}
	for _, id := range []string{a, b} {
		execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", id, role)
	}
	execFixture(t, pool, `INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,legacy_source,legacy_source_id,is_active)VALUES($1,'FIX','Fixture','fixture','Fixture','','','','','','','','','','','','','India','','',true)`, client)
	for i, id := range []string{p1, p2} {
		execFixture(t, pool, "INSERT INTO projects(id,project_number,name,normalized_name,client_id,project_manager_id)VALUES($1,$2,$3,$4,$5,$6)", id, []string{"PRJ-FIX-1", "PRJ-FIX-2"}[i], []string{"First project", "Second project"}[i], []string{"first project", "second project"}[i], client, pm)
	}
	ids := projectIdentities{"admin": {ID: admin, Active: true, Superuser: true}, "pm": {ID: pm, Active: true, Permissions: append(append([]string{}, perms...), "projects.purchase_assignments.write")}, "a": {ID: a, Active: true, Permissions: perms}, "b": {ID: b, Active: true, Permissions: perms}, "outsider": {ID: outsider, Active: true, Permissions: perms}, "portfolio": {ID: outsider, Active: true, Permissions: append(append([]string{}, perms...), "projects.purchase_portfolio.read")}, "view": {ID: a, Active: true, Permissions: perms[:1]}}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	build := func(j func(database.Connection) events.Writer) *purchaseApp.WorkspaceService {
		return purchaseApp.NewWorkspace(purchaseRepository.NewWorkspaceUnitOfWork(pool, func(c database.Connection) projects.ContextReader { return projectout.NewContextReader(c) }, func(c database.Connection) auth.AssignmentDirectory {
			return authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		}, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	s := build(journal)
	filter := purchaseDomain.WorkspaceFilter{Page: 1, Size: 20, Sort: "name", Direction: "desc"}
	if page, e := s.Dashboard(ctx, "outsider", filter); e != nil || page.Total != 0 {
		t.Fatal("unassigned global reader saw projects", page.Total, e)
	}
	if _, e := s.Context(ctx, "outsider", p1); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("unrelated URL allowed", e)
	}
	if _, e := s.Dashboard(ctx, "view", filter); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("view without read accepted", e)
	}
	for _, id := range []string{p1, p2} {
		if _, e := s.Assign(ctx, "pm", id, purchaseDomain.WorkspaceChange{ManagerID: a, Version: 0, Reason: "Named purchasing responsibility"}); e != nil {
			t.Fatal(e)
		}
	}
	if page, e := s.Dashboard(ctx, "a", filter); e != nil || page.Total != 2 || page.Rows[0].Project.Name != "Second project" {
		t.Fatal("multiple assignments/sort", page, e)
	}
	if page, e := s.Dashboard(ctx, "a", purchaseDomain.WorkspaceFilter{Page: 1, Size: 20, Search: "%"}); e != nil || page.Total != 0 {
		t.Fatal("wildcard search not literal", e)
	}
	if _, e := s.Assign(ctx, "a", p1, purchaseDomain.WorkspaceChange{ManagerID: b, Version: 1, Reason: "Forbidden"}); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("process owner changed project assignment", e)
	}
	if _, e := s.Options(ctx, "portfolio", p1); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("portfolio read granted assignment", e)
	}
	if _, e := s.Assign(ctx, "pm", p1, purchaseDomain.WorkspaceChange{ManagerID: outsider, Version: 1, Reason: "Invalid candidate"}); e == nil {
		t.Fatal("ineligible candidate accepted")
	}
	if _, e := s.Assign(ctx, "admin", p1, purchaseDomain.WorkspaceChange{ManagerID: b, Version: 1, Reason: "Responsibility handover"}); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Context(ctx, "a", p1); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("prior owner retained access", e)
	}
	if c, e := s.Context(ctx, "b", p1); e != nil || c.Row.Project.ManagerID != pm || len(c.History) != 2 || c.History[0].PreviousID != a {
		t.Fatal("handover altered PM/history", e)
	}
	if _, e := pool.Exec(ctx, "UPDATE purchase_fms_owner_history SET reason='changed' WHERE project_id=$1", p1); e == nil {
		t.Fatal("mutable history")
	}
	if _, e := pool.Exec(ctx, "TRUNCATE purchase_fms_owner_history"); e == nil {
		t.Fatal("history truncated")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Assign(ctx, "admin", p2, purchaseDomain.WorkspaceChange{ManagerID: b, Version: 1, Reason: "Concurrent handover"})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success, stale := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, purchaseDomain.WorkspaceStale) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatal("version race", success, stale)
	}
	broken := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	if _, e := broken.Assign(ctx, "admin", p1, purchaseDomain.WorkspaceChange{ManagerID: a, Version: 2, Reason: "Must roll back"}); e == nil {
		t.Fatal("event failure accepted")
	}
	if c, e := s.Context(ctx, "admin", p1); e != nil || c.Row.Workspace.Version != 2 || c.Row.Workspace.ManagerID != b || len(c.History) != 2 {
		t.Fatal("failed audit changed source", e)
	}

	// Summary is scoped like the workspace, derives current active register state,
	// and revokes the contribution when one preparation is reopened.
	boID, lineID := v.ID(), v.ID()
	execFixture(t, pool, "INSERT INTO purchase_bo_lists(id,project_id,preparer_id,name,state,is_active,version,completed_at,completed_by_id,created_by_id,updated_by_id) VALUES($1,$2,$3,'Fixture register','completed',true,1,clock_timestamp(),$3,$3,$3)", boID, p1, b)
	execFixture(t, pool, "INSERT INTO purchase_bo_lines(id,bo_id,item_snapshot,reference,notes,sort_order,is_active,created_by_id,updated_by_id) VALUES($1,$2,'{}','','',0,true,$3,$3)", lineID, boID, b)
	for _, actor := range []string{"admin", "b"} {
		c, e := s.Context(ctx, actor, p1)
		if e != nil || (c.Row.Progress.Percent == nil || *c.Row.Progress.Percent != 4.1) || c.Row.Progress.Next != "PUR-02" || len(c.Row.Progress.Completed) != 1 {
			t.Fatal("finalised context credit", actor, c, e)
		}
		page, e := s.Dashboard(ctx, actor, filter)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, r := range page.Rows {
			if r.Project.ID == p1 {
				found = true
				if r.Progress.Percent == nil || *r.Progress.Percent != 4.1 {
					t.Fatal("dashboard credit", r)
				}
			}
		}
		if !found {
			t.Fatal("missing scoped project")
		}
	}
	execFixture(t, pool, "UPDATE purchase_bo_lists SET state='draft',completed_at=NULL,completed_by_id=NULL WHERE id=$1", boID)
	if c, e := s.Context(ctx, "b", p1); e != nil || (c.Row.Progress.Percent == nil || *c.Row.Progress.Percent != 0) || len(c.Row.Progress.Completed) != 0 || c.Row.Progress.Next != "PUR-01" {
		t.Fatal("reopened credit", c, e)
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", a)
	if _, e := s.Assign(ctx, "pm", p1, purchaseDomain.WorkspaceChange{ManagerID: a, Version: 2, Reason: "Inactive"}); e == nil {
		t.Fatal("inactive assignee accepted")
	}
	execFixture(t, pool, "UPDATE projects SET lifecycle_status='on_hold' WHERE id=$1", p1)
	if _, e := s.Assign(ctx, "pm", p1, purchaseDomain.WorkspaceChange{ManagerID: admin, Version: 2, Reason: "On hold"}); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("held project change", e)
	}
	mux := chi.NewRouter()
	purchaseHTTP.WorkspaceRegister(mux, s)
	req := httptest.NewRequest("GET", "/api/v1/purchase-fms", nil)
	req.Header.Set("Authorization", "Bearer admin")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"progress_percent":null`) {
		t.Fatal("empty register credit", rec.Code)
	}
	req = httptest.NewRequest("GET", "/api/v1/purchase-fms/projects/"+p2, nil)
	req.Header.Set("Authorization", "Bearer outsider")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatal("HTTP scope", rec.Code)
	}
	for _, payload := range []string{`{"purchase_manager_id":"` + b + `","reason":"bad","unexpected":true}`, `{"purchase_manager_id":"` + b + `","reason":"missing version"}`} {
		req = httptest.NewRequest("PATCH", "/api/v1/purchase-fms/projects/"+p2+"/owner", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer admin")
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != 422 {
			t.Fatal("strict JSON", rec.Code)
		}
	}
	var eventsCount int
	if e := pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_fms'").Scan(&eventsCount); e != nil {
		t.Fatal(e)
	}
	if eventsCount != 4 {
		t.Fatal("atomic event count", eventsCount)
	}
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(21); e == nil {
		t.Fatal("populated purchasing rollback accepted")
	}
}
