package integration

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func verifyNativePurchaseInspection(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.IndentsFactory, wp purchaseRepository.WorkflowFactory, ws *purchaseApp.WorkflowService, pkg string, financeFactory purchaseRepository.QuotationApprovalsFactory) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_inspection.read", "projects.purchase_inspection.write"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native inspection',true)", role, "Native inspection "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native inspection')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	actor := ids["acceptance-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), codes...)
	ids["inspection-writer"] = actor
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign actual inspection doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 12, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.InspectionsToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	providers := func(c database.Connection) purchaseRepository.InspectionsProviders {
		x := p(c)
		scope := accessout.NewGate(c, accessProviders(c))
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		workSource := purchaseRepository.NewWorkflowSourceReader(c, wp(c))
		orders := purchaseRepository.NewOrdersReleasedReader(c)
		accepted := purchaseRepository.NewAcceptanceAcceptedReader(c, purchaseRepository.AcceptanceProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Orders: orders})
		technical := purchaseRepository.NewTechnicalClearanceReader(c, purchaseRepository.TechnicalProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Released: purchaseRepository.NewSourcingReleasedReader(c)})
		return purchaseRepository.InspectionsProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Orders: orders, Accepted: accepted, Technical: technical}
	}
	factory := func(c database.Connection) purchasePorts.InspectionsContext {
		return purchaseRepository.NewInspectionsContext(providers(c))
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.InspectionsService {
		return purchaseApp.NewInspections(purchaseRepository.NewInspectionsUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	purchaseHTTP.InspectionsRegister(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/inspection"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		return rec
	}

	initial, e := service.Workspace(ctx, "inspection-writer", f.project, pkg)
	if e != nil || initial.Record != nil || initial.Source.Guard() != nil || initial.Source.QAPFileID == initial.Source.SignedApprovalFileID {
		t.Fatal("actual distinct QAP and approval evidence", e, initial.Source)
	}
	if request("GET", "", "").Code != 401 {
		t.Fatal("unauthenticated inspection exposed")
	}
	if request("POST", "inspection-writer", `{"version":0,"action":"record","reason":"r","inspection":{"status":"not_scheduled"},"source_snapshot":{}}`).Code != 422 {
		t.Fatal("forged source accepted")
	}
	q := purchaseDomain.InspectionsCommand{Action: "record", Reason: "Actual factory inspection scheduled", Inspection: &purchaseDomain.InspectionsInspectionInput{Status: "scheduled", ScheduledDate: purchaseDomain.InspectionsToday(v.Now()), Factory: "Actual manufacturer factory", Agency: "client_consultant", Points: []purchaseDomain.InspectionsWitnessInput{}}}
	if leaked, e := failed.Execute(ctx, "inspection-writer", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure leaked inspection")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_inspections").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed inspection persisted", e)
	}
	raw, _ := json.Marshal(q)
	response := request("POST", "inspection-writer", string(raw))
	var current purchaseDomain.InspectionsWorkspace
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.Record == nil || current.Record.Inspection.Status != "scheduled" || len(current.History) != 1 {
		t.Fatal("owned inspection HTTP", response.Code, response.Body.String())
	}
	checkProgress := func(expected purchaseDomain.InspectionsSourceFacts, want string) {
		t.Helper()
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			p, err := purchaseRepository.NewInspectionsProgressReader(c).Progress(txctx, f.project, pkg, expected, true)
			if err != nil || p.State != want || !p.Current {
				t.Fatal("actual inspection progress", err, p)
			}
			if want == "completed" && (p.Snapshot == nil || p.Snapshot.Inspection.Status != "cleared") {
				t.Fatal("inspection completion lacks clearance", p)
			}
			changed := expected
			changed.TechnicalRevision++
			stale, err := purchaseRepository.NewInspectionsProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
			if err != nil || stale.State != "blocked" || stale.Current {
				t.Fatal("obsolete technical revision earned inspection completion", err, stale)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := purchaseRepository.NewInspectionsProgressReader(f.pool).Progress(ctx, f.project, pkg, expected, true); err == nil {
			t.Fatal("unbound inspection progress reader accepted pool")
		}
	}
	checkProgress(current.Source.InspectionsSourceFacts, "in_progress")
	q.Version = current.Record.Version
	q.Inspection.Status = "cleared"
	q.Inspection.ActualDate = purchaseDomain.InspectionsToday(v.Now())
	q.Inspection.Inspector = "Actual inspecting engineer"
	q.Inspection.Findings = "Actual inspection passed approved QAP"
	yes, no := true, false
	q.Inspection.Points = []purchaseDomain.InspectionsWitnessInput{{Code: "WP-01", Title: "Actual QAP test", Mandatory: &yes, Cleared: &no}}
	if _, e = service.Execute(ctx, "inspection-writer", f.project, pkg, q); e == nil {
		t.Fatal("uncleared mandatory point passed")
	}
	q.Inspection.Points[0].Cleared = &yes
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_inspection.write')", role)
	if _, e = service.Execute(ctx, "inspection-writer", f.project, pkg, q); e == nil {
		t.Fatal("revoked inspection permission accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_inspection.write'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "inspection-writer", f.project, pkg)
	if e != nil || held.Record == nil || len(held.Actions) > 0 {
		t.Fatal("held inspection lost history or allows actions", e)
	}
	if _, e = service.Execute(ctx, "inspection-writer", f.project, pkg, q); e == nil {
		t.Fatal("held inspection saved")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, e := service.Execute(ctx, "inspection-writer", f.project, pkg, q)
			results <- e
		}()
	}
	group.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("concurrent clearance winners", wins)
	}
	current, e = service.Workspace(ctx, "inspection-writer", f.project, pkg)
	if e != nil || current.Record.Version != 2 || len(current.History) != 2 {
		t.Fatal("cleared inspection", e)
	}
	checkProgress(current.Source.InspectionsSourceFacts, "completed")
	id := current.Record.ID
	if _, e = purchaseRepository.NewInspectionsClearedReader(f.pool, providers(f.pool)).Cleared(ctx, f.project, pkg); e == nil {
		t.Fatal("unbound clearance reader accepted pool")
	}
	fact := func(want bool) {
		t.Helper()
		if e := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			v, e := purchaseRepository.NewInspectionsClearedReader(c, providers(c)).Cleared(txctx, f.project, pkg)
			if want && (e != nil || v == nil || v.ID != id) || !want && e == nil {
				t.Fatal("current inspection fact", want, e)
			}
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	fact(true)
	if _, e = service.Execute(ctx, "inspection-writer", f.project, pkg, purchaseDomain.InspectionsCommand{Version: 2, Action: "record", Reason: "Overwrite final", Inspection: q.Inspection}); e == nil {
		t.Fatal("final inspection overwritten")
	}
	current, e = service.Execute(ctx, "inspection-writer", f.project, pkg, purchaseDomain.InspectionsCommand{Version: 2, Action: "revise", Reason: "Actual corrected QAP inspection"})
	if e != nil || current.Record.Revision != 2 || current.Record.Inspection.Status != "not_scheduled" || len(current.Record.Inspection.Points) != 0 || len(current.History) != 3 {
		t.Fatal("revision did not retain/reset evidence", e)
	}
	checkProgress(current.Source.InspectionsSourceFacts, "not_started")
	fact(false)
	q.Version = 3
	q.Inspection.Status = "rejected"
	q.Inspection.Points[0].Cleared = &no
	q.Inspection.Findings = "Actual failed QAP test"
	current, e = service.Execute(ctx, "inspection-writer", f.project, pkg, q)
	if e != nil || current.Record.Inspection.Status != "rejected" {
		t.Fatal("truthful failed inspection", e)
	}
	checkProgress(current.Source.InspectionsSourceFacts, "blocked")
	fact(false)
	current, e = service.Execute(ctx, "inspection-writer", f.project, pkg, purchaseDomain.InspectionsCommand{Version: 4, Action: "revise", Reason: "Actual retest after failed inspection"})
	if e != nil {
		t.Fatal(e)
	}
	q.Version = 5
	q.Inspection.Status = "cleared"
	q.Inspection.Points[0].Cleared = &yes
	q.Inspection.Findings = "Actual retest passed"
	if _, e = service.Execute(ctx, "inspection-writer", f.project, pkg, q); e != nil {
		t.Fatal(e)
	}
	fact(true)
	verifyNativePurchaseInspectionReport(t, f, ids, p, wp, ws, pkg, financeFactory)
	for _, sql := range []string{"UPDATE purchase_inspections SET version=version+1 WHERE id=$1", "DELETE FROM purchase_inspections WHERE id=$1", "DELETE FROM purchase_inspection_events WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("unaudited inspection/history mutation allowed")
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_inspections' AND entity_id=$1", id).Scan(&count); e != nil || count != 8 {
		t.Fatal("atomic inspection project history", e, count)
	}
}

func verifyInspectionAfterAcceptanceChange(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.IndentsFactory, wp purchaseRepository.WorkflowFactory, pkg string) {
	t.Helper()
	ctx := t.Context()
	providers := func(c database.Connection) purchaseRepository.InspectionsProviders {
		x := p(c)
		scope := accessout.NewGate(c, accessProviders(c))
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		workSource := purchaseRepository.NewWorkflowSourceReader(c, wp(c))
		orders := purchaseRepository.NewOrdersReleasedReader(c)
		return purchaseRepository.InspectionsProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Orders: orders, Accepted: purchaseRepository.NewAcceptanceAcceptedReader(c, purchaseRepository.AcceptanceProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Orders: orders}), Technical: purchaseRepository.NewTechnicalClearanceReader(c, purchaseRepository.TechnicalProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Released: purchaseRepository.NewSourcingReleasedReader(c)})}
	}
	service := purchaseApp.NewInspections(purchaseRepository.NewInspectionsUnitOfWork(f.pool, func(c database.Connection) purchasePorts.InspectionsContext {
		return purchaseRepository.NewInspectionsContext(providers(c))
	}, purchaseJournal), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	w, e := service.Workspace(ctx, "inspection-writer", f.project, pkg)
	if e != nil || w.Record == nil || w.Record.Inspection.Status != "cleared" || len(w.History) != 8 || len(w.Actions) != 0 || w.Source.Guard() == nil {
		t.Fatal("changed acceptance must retain obsolete inspection history without authority", e)
	}
	if e = database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		v, e := purchaseRepository.NewInspectionsClearedReader(c, providers(c)).Cleared(txctx, f.project, pkg)
		if e == nil || v != nil {
			t.Fatal("conditional acceptance reused prior inspection clearance")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
