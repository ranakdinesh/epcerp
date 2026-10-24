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

func verifyNativePurchaseInspectionReport(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.IndentsFactory, wp purchaseRepository.WorkflowFactory, ws *purchaseApp.WorkflowService, pkg string, financeFactory purchaseRepository.QuotationApprovalsFactory) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_inspection_reports.read", "projects.purchase_inspection_reports.write"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native report',true)", role, "Native report "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native report')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	actor := ids["inspection-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), codes...)
	ids["report-writer"] = actor
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign actual report doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 13, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.InspectionReportsToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	providers := func(c database.Connection) purchaseRepository.InspectionReportsProviders {
		x := p(c)
		scope := accessout.NewGate(c, accessProviders(c))
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		workSource := purchaseRepository.NewWorkflowSourceReader(c, wp(c))
		orders := purchaseRepository.NewOrdersReleasedReader(c)
		accepted := purchaseRepository.NewAcceptanceAcceptedReader(c, purchaseRepository.AcceptanceProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Orders: orders})
		technical := purchaseRepository.NewTechnicalClearanceReader(c, purchaseRepository.TechnicalProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Released: purchaseRepository.NewSourcingReleasedReader(c)})
		return purchaseRepository.InspectionReportsProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Inspection: purchaseRepository.NewInspectionsClearedReader(c, purchaseRepository.InspectionsProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Orders: orders, Accepted: accepted, Technical: technical})}
	}
	factory := func(c database.Connection) purchasePorts.InspectionReportsContext {
		return purchaseRepository.NewInspectionReportsContext(providers(c))
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.InspectionReportsService {
		return purchaseApp.NewInspectionReports(purchaseRepository.NewInspectionReportsUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	purchaseHTTP.InspectionReportsRegister(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/inspection-report"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		return rec
	}

	initial, e := service.Workspace(ctx, "report-writer", f.project, pkg)
	if e != nil || initial.Record != nil || initial.Source.Guard() != nil || initial.Source.InspectionVersion != 6 {
		t.Fatal("actual cleared inspection source", e, initial.Source)
	}
	if request("GET", "", "").Code != 401 {
		t.Fatal("unauthenticated report exposed")
	}
	if request("POST", "report-writer", `{"version":0,"action":"record","reason":"r","report":{"status":"pending","uom":"forged"}}`).Code != 422 {
		t.Fatal("forged report unit accepted")
	}
	q := purchaseDomain.InspectionReportsCommand{Action: "record", Reason: "Prepare actual IR / IRN", Report: &purchaseDomain.InspectionReportsReportInput{Status: "pending", Quantity: "0"}}
	if leaked, e := failed.Execute(ctx, "report-writer", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure leaked report")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_inspection_reports").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed report persisted", e)
	}
	raw, _ := json.Marshal(q)
	response := request("POST", "report-writer", string(raw))
	var current purchaseDomain.InspectionReportsWorkspace
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.Record == nil || current.Record.Report.Status != "pending" || len(current.History) != 1 {
		t.Fatal("owned report HTTP", response.Code, response.Body.String())
	}
	checkProgress := func(want string) {
		t.Helper()
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			p, err := purchaseRepository.NewInspectionReportsProgressReader(c).Progress(txctx, f.project, pkg, current.Source.InspectionReportsSourceFacts, true)
			if err != nil || p.State != want || !p.Current {
				t.Fatal("actual IRN progress", err, p)
			}
			if want == "completed" && (p.Snapshot == nil || p.Snapshot.Report.Quantity != current.Record.Report.Quantity) {
				t.Fatal("report lost actual issued quantity", p)
			}
			changed := current.Source.InspectionReportsSourceFacts
			changed.InspectionVersion++
			stale, err := purchaseRepository.NewInspectionReportsProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
			if err != nil || stale.State != "blocked" || stale.Current {
				t.Fatal("changed inspection earned issued report", err, stale)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := purchaseRepository.NewInspectionReportsProgressReader(f.pool).Progress(ctx, f.project, pkg, current.Source.InspectionReportsSourceFacts, true); err == nil {
			t.Fatal("unbound IRN progress reader accepted pool")
		}
	}
	checkProgress("not_started")
	q.Version = 1
	q.Report.Status = "issued"
	q.Report.IRNumber = "Actual IR"
	q.Report.IRNNumber = "Actual IRN"
	q.Report.IssueDate = purchaseDomain.InspectionReportsToday(v.Now())
	q.Report.Quantity = "999999.000001"
	if _, e = service.Execute(ctx, "report-writer", f.project, pkg, q); e == nil {
		t.Fatal("over-cleared quantity passed")
	}
	q.Report.Quantity = "1.000001"
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_inspection_reports.write')", role)
	if _, e = service.Execute(ctx, "report-writer", f.project, pkg, q); e == nil {
		t.Fatal("revoked report permission accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_inspection_reports.write'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "report-writer", f.project, pkg)
	if e != nil || held.Record == nil || len(held.Actions) > 0 {
		t.Fatal("held report lost history or allows actions", e)
	}
	if _, e = service.Execute(ctx, "report-writer", f.project, pkg, q); e == nil {
		t.Fatal("held report saved")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, e := service.Execute(ctx, "report-writer", f.project, pkg, q)
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
		t.Fatal("concurrent issuance winners", wins)
	}
	current, e = service.Workspace(ctx, "report-writer", f.project, pkg)
	if e != nil || current.Record.Version != 2 || current.Record.Report.Quantity != "1.000001" || current.Record.Report.UOM != initial.Source.UOM {
		t.Fatal("exact partial issued report", e)
	}
	checkProgress("completed")
	id := current.Record.ID
	if _, e = purchaseRepository.NewInspectionReportsIssuedReader(f.pool, providers(f.pool)).Issued(ctx, f.project, pkg); e == nil {
		t.Fatal("unbound issued reader accepted pool")
	}
	fact := func(want bool) {
		t.Helper()
		if e := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			v, e := purchaseRepository.NewInspectionReportsIssuedReader(c, providers(c)).Issued(txctx, f.project, pkg)
			if want && (e != nil || v == nil || v.ID != id) || !want && e == nil {
				t.Fatal("current issued report fact", want, e)
			}
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	fact(true)
	if _, e = service.Execute(ctx, "report-writer", f.project, pkg, q); e == nil {
		t.Fatal("issued report overwritten")
	}
	current, e = service.Execute(ctx, "report-writer", f.project, pkg, purchaseDomain.InspectionReportsCommand{Version: 2, Action: "revise", Reason: "Actual report correction"})
	if e != nil || current.Record.Revision != 2 || current.Record.Report.Status != "pending" || current.Record.Report.IRNNumber != "" {
		t.Fatal("report revision failed", e)
	}
	checkProgress("not_started")
	fact(false)
	q.Version = 3
	if _, e = service.Execute(ctx, "report-writer", f.project, pkg, q); e != nil {
		t.Fatal(e)
	}
	fact(true)
	// Reopen and re-clear the actual owning inspection, never mutate its table.
	ip := func(c database.Connection) purchaseRepository.InspectionsProviders {
		x := p(c)
		scope := accessout.NewGate(c, accessProviders(c))
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		workSource := purchaseRepository.NewWorkflowSourceReader(c, wp(c))
		orders := purchaseRepository.NewOrdersReleasedReader(c)
		return purchaseRepository.InspectionsProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Orders: orders, Accepted: purchaseRepository.NewAcceptanceAcceptedReader(c, purchaseRepository.AcceptanceProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Orders: orders}), Technical: purchaseRepository.NewTechnicalClearanceReader(c, purchaseRepository.TechnicalProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Released: purchaseRepository.NewSourcingReleasedReader(c)})}
	}
	inspect := purchaseApp.NewInspections(purchaseRepository.NewInspectionsUnitOfWork(f.pool, func(c database.Connection) purchasePorts.InspectionsContext {
		return purchaseRepository.NewInspectionsContext(ip(c))
	}, purchaseJournal), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	old, e := inspect.Workspace(ctx, "inspection-writer", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = inspect.Execute(ctx, "inspection-writer", f.project, pkg, purchaseDomain.InspectionsCommand{Version: old.Record.Version, Action: "revise", Reason: "Actual inspection rework after issued IRN"}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e := service.Workspace(ctx, "report-writer", f.project, pkg)
	if e != nil || blocked.Record == nil || len(blocked.Actions) != 0 || len(blocked.History) != 4 {
		t.Fatal("obsolete report history", e)
	}
	i := old.Record.Inspection
	input := purchaseDomain.InspectionsInspectionInput{Status: i.Status, CallReference: i.CallReference, CallDate: i.CallDate, ScheduledDate: i.ScheduledDate, ActualDate: i.ActualDate, Agency: i.Agency, Inspector: i.Inspector, Factory: i.Factory, NoPointsReason: i.NoPointsReason, Findings: i.Findings}
	for _, point := range i.Points {
		mandatory, cleared := point.Mandatory, point.Cleared
		input.Points = append(input.Points, purchaseDomain.InspectionsWitnessInput{Code: point.Code, Title: point.Title, Mandatory: &mandatory, Cleared: &cleared, Remarks: point.Remarks})
	}
	if _, e = inspect.Execute(ctx, "inspection-writer", f.project, pkg, purchaseDomain.InspectionsCommand{Version: old.Record.Version + 1, Action: "record", Reason: "Actual inspection re-clearance", Inspection: &input}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	q.Version = 4
	if _, e = service.Execute(ctx, "report-writer", f.project, pkg, q); e == nil {
		t.Fatal("old report source overwritten")
	}
	if _, e = service.Execute(ctx, "report-writer", f.project, pkg, purchaseDomain.InspectionReportsCommand{Version: 4, Action: "revise", Reason: "Re-issue against current inspection"}); e != nil {
		t.Fatal(e)
	}
	q.Version = 5
	if _, e = service.Execute(ctx, "report-writer", f.project, pkg, q); e != nil {
		t.Fatal(e)
	}
	fact(true)
	for _, sql := range []string{"UPDATE purchase_inspection_reports SET version=version+1 WHERE id=$1", "DELETE FROM purchase_inspection_reports WHERE id=$1", "DELETE FROM purchase_inspection_report_events WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("unaudited report/history mutation allowed")
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_inspection_reports' AND entity_id=$1", id).Scan(&count); e != nil || count != 6 {
		t.Fatal("atomic report project history", e, count)
	}
	verifyNativePurchaseQuality(t, f, ids, wp, ws, pkg, providers, service)
}
