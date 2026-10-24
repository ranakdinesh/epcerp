package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	authOut "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authApp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"

	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	projectIn "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/inbound"
	projectOut "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	projectApp "github.com/ranakdinesh/epcERP/internal/modules/projects/application"
	projectContracts "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

type projectIdentities map[string]auth.Identity

func (p projectIdentities) Resolve(_ context.Context, token string) (auth.Identity, error) {
	return p[token], nil
}
func TestProjectOwnershipTransactionAndScope(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	values := projectOut.SystemValues{}
	admin, first, second, clientID, roleID := values.ID(), values.ID(), values.ID(), values.ID(), values.ID()
	for i, id := range []string{admin, first, second} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,'fixture',true,$4)", id, []string{"admin", "manager_a", "manager_b"}[i], []string{"a@example.invalid", "b@example.invalid", "c@example.invalid"}[i], i == 0)
	}
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,'manager','fixture',true)", roleID)
	for _, code := range []string{"projects.master.read", "projects.master.write"} {
		id := values.ID()
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description) VALUES($1,$2,'fixture')", id, code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id) VALUES($1,$2)", roleID, id)
	}
	for _, id := range []string{first, second} {
		execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", id, roleID)
	}
	execFixture(t, pool, `INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,legacy_source,legacy_source_id,is_active)
 VALUES($1,'CLIENT','Fixture client','fixture client','Fixture client','','','','','','','','','','','','','India','','',true)`, clientID)
	readWrite := []string{"projects.master.read", "projects.master.write"}
	identities := projectIdentities{"admin": {ID: admin, Active: true, Superuser: true}, "first": {ID: first, Active: true, Permissions: readWrite}, "second": {ID: second, Active: true, Permissions: readWrite}, "viewer": {ID: values.ID(), Active: true, Permissions: []string{"projects.master.read", "projects.portfolio.read"}}}

	buyer, buyerRole := values.ID(), values.ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'buyer','buyer@example.invalid','fixture',true,false)", buyer)
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,'procurement.executive','fixture',true)", buyerRole)
	purchaseCodes := []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read"}
	for _, code := range purchaseCodes {
		pid := values.ID()
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description) VALUES($1,$2,'fixture')", pid, code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id) VALUES($1,$2)", buyerRole, pid)
	}
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", buyer, buyerRole)
	identities["buyer"] = auth.Identity{ID: buyer, Active: true, Permissions: purchaseCodes}
	service := projectApp.New(projectOut.NewUnitOfWork(pool,
		func(c database.Connection) masterContracts.ClientDirectory {
			return masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))
		},
		func(c database.Connection) auth.AssignmentDirectory {
			return authApp.NewAssignmentDirectory(authOut.NewAssignmentReader(c))
		},
		func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }).WithPurchasing(projectPurchaseOwnership),
		projectOut.NewAuthBridge(identities), values)
	create := func(name string) *domain.Project {
		p, e := service.Create(ctx, "admin", domain.Create{ProcurementExecutiveID: buyer, Name: name, ClientID: clientID, ManagerID: first, Capture: domain.Capture{ProjectType: "electrical", SiteState: "Delhi", SiteCity: "New Delhi", OperationsManagerID: second, ContractValue: "99999999999999.99", Currency: "INR", LOIDate: "2026-10-01", ScheduleStartBasis: "loi_date", PlannedDurationDays: "31", ScopeSummary: "Electrical works", Remarks: "Fixture capture", SafetyBufferPercent: "20.25", SafetyBufferDays: "5"}})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	p := create("EPC one")
	if p.ContractValue != "99999999999999.99" || p.PlannedCompletionDate != "2026-10-31" || p.OperationsManagerID != second || p.Remarks != "Fixture capture" {
		t.Fatalf("capture did not round trip: version %d", p.Version)
	}
	loaded, err := service.Get(ctx, "admin", p.ID)
	if err != nil || !reflect.DeepEqual(loaded.Capture, p.Capture) {
		t.Fatal("capture read mismatch", err)
	}

	options, e := service.Options(ctx, "admin")
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, c := range options.Procurement {
		if c.ID == buyer {
			found = true
		}
		if c.ID == first || c.ID == second {
			t.Fatal("project-only manager offered for procurement")
		}
	}
	if !found {
		t.Fatal("Procurement Executive missing")
	}
	if loaded.ProcurementExecutiveID != buyer {
		t.Fatal("initial purchase owner missing from project read")
	}
	if _, e = service.Create(ctx, "admin", domain.Create{Name: "Ineligible procurement", ClientID: clientID, ManagerID: first, ProcurementExecutiveID: second, Capture: p.Capture}); e == nil {
		t.Fatal("ineligible procurement accepted")
	}
	if countRows(t, pool, "projects") != 1 || countRows(t, pool, "purchase_fms_workspaces") != 1 {
		t.Fatal("invalid procurement leaked project")
	}
	create("EPC two")
	purchases := purchaseApp.NewWorkspace(purchaseRepository.NewWorkspaceUnitOfWork(pool, func(c database.Connection) projectContracts.ContextReader { return projectOut.NewContextReader(c) }, func(c database.Connection) auth.AssignmentDirectory {
		return authApp.NewAssignmentDirectory(authOut.NewAssignmentReader(c))
	}, func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }), purchaseRepository.NewAuthBridge(identities), purchaseRepository.Values{})
	dashboard, e := purchases.Dashboard(ctx, "buyer", purchaseDomain.WorkspaceFilter{Page: 1, Size: 50})
	if e != nil || dashboard.Total != 2 {
		t.Fatal("executive purchase dashboard scope", e)
	}
	execFixture(t, pool, `CREATE FUNCTION initial_purchase_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$;CREATE CONSTRAINT TRIGGER initial_purchase_failure AFTER INSERT ON purchase_fms_owner_history DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION initial_purchase_failure();`)
	if _, e = service.Create(ctx, "admin", domain.Create{Name: "Failed initial purchase", ClientID: clientID, ManagerID: first, ProcurementExecutiveID: buyer, Capture: p.Capture}); e == nil {
		t.Fatal("purchase audit failure accepted")
	}
	if countRows(t, pool, "projects") != 2 || countRows(t, pool, "purchase_fms_owner_history") != 2 || countRows(t, pool, "project_manager_history") != 2 || countRows(t, pool, "business_events") != 4 {
		t.Fatal("failed purchase assignment leaked source/history")
	}
	execFixture(t, pool, "DROP TRIGGER initial_purchase_failure ON purchase_fms_owner_history;DROP FUNCTION initial_purchase_failure()")

	if page, e := service.List(ctx, "first", domain.Filter{Page: 1, PageSize: 50}); e != nil || page.Total != 2 {
		t.Fatal("one manager cannot own multiple projects", e)
	}
	if _, e := service.Get(ctx, "second", p.ID); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("unassigned reader allowed", e)
	}
	if _, e := service.Create(ctx, "first", domain.Create{ProcurementExecutiveID: admin, Name: "No", ClientID: clientID, ManagerID: first, Capture: domain.Capture{ProjectType: "electrical", SiteState: "Delhi", SiteCity: "New Delhi", OperationsManagerID: second, ContractValue: "99999999999999.99", Currency: "INR", LOIDate: "2026-10-01", ScheduleStartBasis: "loi_date", PlannedDurationDays: "31", ScopeSummary: "Electrical works", Remarks: "Fixture capture", SafetyBufferPercent: "20.25", SafetyBufferDays: "5"}}); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("ordinary manager gained portfolio creation")
	}
	if _, e := service.Create(ctx, "admin", domain.Create{ProcurementExecutiveID: admin, Name: "ePc ONE", ClientID: clientID, ManagerID: first, Capture: domain.Capture{ProjectType: "electrical", SiteState: "Delhi", SiteCity: "New Delhi", OperationsManagerID: second, ContractValue: "99999999999999.99", Currency: "INR", LOIDate: "2026-10-01", ScheduleStartBasis: "loi_date", PlannedDurationDays: "31", ScopeSummary: "Electrical works", Remarks: "Fixture capture", SafetyBufferPercent: "20.25", SafetyBufferDays: "5"}}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("duplicate accepted", e)
	}
	if _, e := service.Transfer(ctx, "viewer", p.ID, domain.Transfer{ManagerID: second, Reason: "handover", ExpectedVersion: 1}); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("portfolio reader gained transfer")
	}
	statuses := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := service.Transfer(ctx, "admin", p.ID, domain.Transfer{ManagerID: second, Reason: "Governed handover", ExpectedVersion: 1})
			statuses <- e
		}()
	}
	wg.Wait()
	close(statuses)
	success, stale := 0, 0
	for e := range statuses {
		if e == nil {
			success++
		} else if errors.Is(e, domain.ErrStale) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatal("concurrent handovers overwrote", success, stale)
	}
	if _, e := service.Get(ctx, "first", p.ID); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("former manager retained scope", e)
	}
	current, e := service.Get(ctx, "second", p.ID)
	if e != nil || current.ManagerID != second || current.Version != 2 || current.ProcurementExecutiveID != buyer || !reflect.DeepEqual(current.Capture, p.Capture) {
		t.Fatal("replacement did not gain scope", e)
	}
	history, e := service.History(ctx, "admin", p.ID)
	if e != nil || len(history) != 2 || history[0].BeforeManagerID != first || history[0].ActorID != admin {
		t.Fatal("history incomplete", e)
	}
	var count int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM project_manager_history h JOIN business_events e ON e.id=h.id AND e.entity_id=h.project_id AND e.entity_version=h.project_version AND e.scope_id=h.project_id AND e.actor_id=h.created_by_id WHERE h.project_id=$1`, p.ID).Scan(&count); e != nil || count != 2 {
		t.Fatal("event/source correlation failed", e)
	}
	for _, sql := range []string{"UPDATE project_manager_history SET reason='changed'", "DELETE FROM project_manager_history", "TRUNCATE project_manager_history"} {
		if _, e = pool.Exec(ctx, sql); e == nil {
			t.Fatal("history mutable")
		}
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", first)
	if _, e = service.Transfer(ctx, "admin", p.ID, domain.Transfer{ManagerID: first, Reason: "Inactive", ExpectedVersion: 2}); e == nil {
		t.Fatal("inactive manager accepted")
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=true WHERE id=$1", first)
	execFixture(t, pool, "DELETE FROM auth.user_roles WHERE user_id=$1", first)
	if _, e = service.Transfer(ctx, "admin", p.ID, domain.Transfer{ManagerID: first, Reason: "Permission revoked", ExpectedVersion: 2}); e == nil {
		t.Fatal("revoked candidate accepted")
	}
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", first, roleID)
	execFixture(t, pool, "UPDATE clients SET is_active=false WHERE id=$1", clientID)
	if _, e = service.Create(ctx, "admin", domain.Create{ProcurementExecutiveID: admin, Name: "Inactive client", ClientID: clientID, ManagerID: first, Capture: domain.Capture{ProjectType: "electrical", SiteState: "Delhi", SiteCity: "New Delhi", OperationsManagerID: second, ContractValue: "99999999999999.99", Currency: "INR", LOIDate: "2026-10-01", ScheduleStartBasis: "loi_date", PlannedDurationDays: "31", ScopeSummary: "Electrical works", Remarks: "Fixture capture", SafetyBufferPercent: "20.25", SafetyBufferDays: "5"}}); e == nil {
		t.Fatal("inactive client accepted")
	}
	execFixture(t, pool, "UPDATE clients SET is_active=true WHERE id=$1", clientID)
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", second)
	if _, e = service.Create(ctx, "admin", domain.Create{ProcurementExecutiveID: second, Name: "Inactive procurement fixture", ClientID: clientID, ManagerID: first, Capture: p.Capture}); e == nil {
		t.Fatal("inactive operations user accepted")
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=true WHERE id=$1", second)
	// A deferred commit failure must roll back source, history and central event.
	execFixture(t, pool, `CREATE FUNCTION project_fixture_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture' USING ERRCODE='23514'; END; $$;
 CREATE CONSTRAINT TRIGGER project_fixture_failure AFTER INSERT ON project_manager_history DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.project_version=3) EXECUTE FUNCTION project_fixture_failure();`)
	if _, e = service.Transfer(ctx, "admin", p.ID, domain.Transfer{ManagerID: first, Reason: "Commit failure", ExpectedVersion: 2}); e == nil {
		t.Fatal("commit failure accepted")
	}
	current, e = service.Get(ctx, "admin", p.ID)
	if e != nil || current.Version != 2 || current.ManagerID != second {
		t.Fatal("commit failure changed current manager", e)
	}
	if countRows(t, pool, "project_manager_history") != 3 || countRows(t, pool, "business_events") != 5 {
		t.Fatal("rollback leaked journal")
	}
	execFixture(t, pool, `CREATE CONSTRAINT TRIGGER project_capture_fixture_failure AFTER INSERT ON project_manager_history DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.project_version=1) EXECUTE FUNCTION project_fixture_failure();`)
	if _, e = service.Create(ctx, "admin", domain.Create{ProcurementExecutiveID: admin, Name: "Rollback capture fixture", ClientID: clientID, ManagerID: first, Capture: p.Capture}); e == nil {
		t.Fatal("failed capture event accepted")
	}
	if countRows(t, pool, "projects") != 2 || countRows(t, pool, "project_manager_history") != 3 || countRows(t, pool, "business_events") != 5 {
		t.Fatal("capture commit failure leaked source or event")
	}
	router := chi.NewRouter()
	projectIn.Register(router, service)
	for _, tc := range []struct {
		method, path, token, body string
		status                    int
	}{
		{"POST", "/api/v1/projects", "admin", `{"name":"Invalid","client_id":"bad","project_manager_id":"bad","unexpected":true}`, 422},
		{"POST", "/api/v1/projects", "admin", `{"name":"Invalid","client_id":"bad","project_manager_id":"bad"} {}`, 422},
		{"PATCH", "/api/v1/projects/" + p.ID + "/manager", "admin", `{"project_manager_id":"bad","reason":"","expected_version":"2"}`, 422},
		{"GET", "/api/v1/projects/" + p.ID, "first", "", 403},
		{"GET", "/api/v1/projects/" + p.ID, "", "", 401},
		{"GET", "/api/v1/projects?page_size=101", "admin", "", 422},
		{"GET", "/api/v1/projects/assignment-options", "admin", "", 200},
	} {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("HTTP %s %s: %d", tc.method, tc.path, w.Code)
		}
		if tc.path == "/api/v1/projects/assignment-options" {
			var body map[string]any
			if json.Unmarshal(w.Body.Bytes(), &body) != nil {
				t.Fatal("invalid options")
			}
			for _, m := range body["managers"].([]any) {
				if len(m.(map[string]any)) != 2 {
					t.Fatal("candidate secrets leaked")
				}
			}
		}
	}
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(22); e == nil {
		t.Fatal("populated project capture rollback accepted")
	}
	var retained string
	if e = pool.QueryRow(ctx, "SELECT contract_value::text FROM projects WHERE id=$1", p.ID).Scan(&retained); e != nil || retained != "99999999999999.99" {
		t.Fatal("rollback guard lost project metadata", e)
	}

}
