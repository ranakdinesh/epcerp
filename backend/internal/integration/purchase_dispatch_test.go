package integration

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
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

func verifyNativePurchaseDispatch(t *testing.T, f purchaseFixture, ids projectIdentities, ws *purchaseApp.WorkflowService, pkg string, cp func(database.Connection) purchaseRepository.ClientClearanceProviders, clearance *purchaseApp.ClientClearanceService) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_dispatch.read", "projects.purchase_dispatch.write"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native dispatch',true)", role, "Native dispatch "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native dispatch')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	actor := ids["client-clearance-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), codes...)
	ids["dispatch-writer"] = actor
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign actual dispatch doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 16, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.DispatchToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	providers := func(c database.Connection) purchaseRepository.DispatchProviders {
		x := cp(c)
		return purchaseRepository.DispatchProviders{Projects: x.Projects, Purchase: x.Purchase, Access: x.Access, Users: x.Users, Work: x.Work, Clearance: purchaseRepository.NewClientClearanceClearedReader(c, x)}
	}
	factory := func(c database.Connection) purchasePorts.DispatchContext {
		return purchaseRepository.NewDispatchContext(providers(c))
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.DispatchService {
		return purchaseApp.NewDispatch(purchaseRepository.NewDispatchUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	purchaseHTTP.DispatchRegister(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/dispatch"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r := httptest.NewRecorder()
		api.ServeHTTP(r, req)
		return r
	}
	initial, e := service.Workspace(ctx, "dispatch-writer", f.project, pkg)
	if e != nil || initial.Record != nil || initial.Source.ClearanceVersion != 6 || initial.Source.ReportVersion != 8 || initial.Source.Quantity != "1.000001" || initial.Source.Guard() != nil {
		t.Fatal("actual verified-quality source", e, initial.Source)
	}
	if request("GET", "", "").Code != 401 || request("POST", "dispatch-writer", `{"version":0,"action":"record","reason":"r","dispatch":{"status":"pending","cleared_quantity":"forged"}}`).Code != 422 {
		t.Fatal("dispatch HTTP protection")
	}
	q := purchaseDomain.DispatchCommand{Action: "record", Reason: "Prepare actual client dispatch", Dispatch: &purchaseDomain.DispatchInput{Status: "pending"}}
	if leaked, e := failed.Execute(ctx, "dispatch-writer", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure leaked dispatch")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_dispatches").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed dispatch persisted", e)
	}
	raw, _ := json.Marshal(q)
	r := request("POST", "dispatch-writer", string(raw))
	var current purchaseDomain.DispatchWorkspace
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &current) != nil || current.Record == nil || current.Record.Dispatch.Status != "pending" || len(current.History) != 1 {
		t.Fatal("native dispatch HTTP", r.Code, r.Body.String())
	}
	checkProgress := func(want string) {
		t.Helper()
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			p, err := purchaseRepository.NewDispatchProgressReader(c).Progress(txctx, f.project, pkg, current.Source.DispatchSourceFacts, true)
			if err != nil || p.State != want || !p.Current {
				t.Fatal("actual purchase_dispatch progress", err, p)
			}
			if want == "completed" && (p.Snapshot == nil || p.Snapshot.ID != current.Record.ID) {
				t.Fatal("completion lacks actual owned evidence", p)
			}
			changed := current.Source.DispatchSourceFacts
			changed.ClearanceVersion++
			stale, err := purchaseRepository.NewDispatchProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
			if err != nil || stale.State != "blocked" || stale.Current {
				t.Fatal("changed source earned completion", err, stale)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := purchaseRepository.NewDispatchProgressReader(f.pool).Progress(ctx, f.project, pkg, current.Source.DispatchSourceFacts, true); err == nil {
			t.Fatal("unbound progress reader accepted pool")
		}
	}
	checkProgress("not_started")
	q.Version = 1
	q.Dispatch = &purchaseDomain.DispatchInput{Status: "dispatched", LRNumber: "Actual LR", Transporter: "Actual transporter", Date: purchaseDomain.DispatchToday(v.Now()), Vehicle: "Actual vehicle reference", EwayNumber: "Actual e-way reference", EwayValidUntil: purchaseDomain.DispatchToday(v.Now().AddDate(0, 0, 1)), InvoiceNumber: "Actual invoice", InvoiceDate: purchaseDomain.DispatchToday(v.Now()), DriverContact: "Actual contact reference", DocumentReference: "Actual dispatch reference"}
	for _, field := range []string{"dispatch_date", "transporter_name", "lr_number"} {
		invalid := q
		input := *q.Dispatch
		invalid.Dispatch = &input
		switch field {
		case "dispatch_date":
			input.Date = ""
		case "transporter_name":
			input.Transporter = " "
		case "lr_number":
			input.LRNumber = ""
		}
		payload, _ := json.Marshal(invalid)
		response := request("POST", "dispatch-writer", string(payload))
		if response.Code != 422 || !strings.Contains(response.Body.String(), field) {
			t.Fatal("missing dispatch field did not produce server validation", field, response.Code)
		}
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_dispatch.write')", role)
	if _, e = service.Execute(ctx, "dispatch-writer", f.project, pkg, q); e == nil {
		t.Fatal("revoked dispatch permission accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_dispatch.write'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "dispatch-writer", f.project, pkg)
	if e != nil || held.Record == nil || len(held.Actions) > 0 {
		t.Fatal("held dispatch lost history or permits actions", e)
	}
	if _, e = service.Execute(ctx, "dispatch-writer", f.project, pkg, q); e == nil {
		t.Fatal("held client clearance saved")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := service.Execute(ctx, "dispatch-writer", f.project, pkg, q)
			results <- err
		}()
	}
	group.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("dispatch concurrent clearance winners", wins)
	}
	current, e = service.Workspace(ctx, "dispatch-writer", f.project, pkg)
	if e != nil || current.Record.Version != 2 || current.Record.Dispatch.LRNumber != "Actual LR" {
		t.Fatal(e)
	}
	checkProgress("completed")
	id := current.Record.ID
	if _, e = purchaseRepository.NewDispatchDispatchedReader(f.pool, providers(f.pool)).Dispatched(ctx, f.project, pkg); e == nil {
		t.Fatal("unbound dispatch consumer accepted")
	}
	fact := func(want bool) {
		t.Helper()
		if e := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			r, e := purchaseRepository.NewDispatchDispatchedReader(c, providers(c)).Dispatched(txctx, f.project, pkg)
			if want && (e != nil || r == nil || r.ID != id) || !want && e == nil {
				t.Fatal("current dispatch fact", want, e)
			}
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	fact(true)
	if _, e = service.Execute(ctx, "dispatch-writer", f.project, pkg, q); e == nil {
		t.Fatal("final dispatch overwritten")
	}
	if leaked, e := failed.Execute(ctx, "dispatch-writer", f.project, pkg, purchaseDomain.DispatchCommand{Version: 2, Action: "revise", Reason: "Must roll back actual correction"}); e == nil || leaked.Record != nil {
		t.Fatal("failed revision escaped")
	}
	fact(true)
	current, e = service.Execute(ctx, "dispatch-writer", f.project, pkg, purchaseDomain.DispatchCommand{Version: 2, Action: "revise", Reason: "Actual client dispatch correction"})
	if e != nil || current.Record.Dispatch.LRNumber != "" || current.Record.Dispatch.Status != "pending" || current.Record.Revision != 2 {
		t.Fatal("dispatch revision reset", e)
	}
	checkProgress("not_started")
	fact(false)
	q.Version = 3
	if _, e = service.Execute(ctx, "dispatch-writer", f.project, pkg, q); e != nil {
		t.Fatal(e)
	}
	fact(true)
	// Revise and re-clear the actual client dispatch through its owning application.
	old, e := clearance.Workspace(ctx, "client-clearance-writer", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = clearance.Execute(ctx, "client-clearance-writer", f.project, pkg, purchaseDomain.ClientClearanceCommand{Version: old.Record.Version, Action: "revise", Reason: "Actual client dispatch rework"}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e := service.Workspace(ctx, "dispatch-writer", f.project, pkg)
	if e != nil || blocked.Record == nil || len(blocked.History) != 4 || len(blocked.Actions) != 0 {
		t.Fatal("dispatch rework lost dispatch history", e)
	}
	next := old.Record.Clearance
	next.Reference = "Corrected actual client dispatch"
	if _, e = clearance.Execute(ctx, "client-clearance-writer", f.project, pkg, purchaseDomain.ClientClearanceCommand{Version: old.Record.Version + 1, Action: "record", Reason: "Actual corrected client clearance", Clearance: &next}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e = service.Workspace(ctx, "dispatch-writer", f.project, pkg)
	if e != nil || len(blocked.Actions) != 1 || blocked.Actions[0] != "revise" {
		t.Fatal("renewed dispatch reused obsolete dispatch", e)
	}
	if _, e = service.Execute(ctx, "dispatch-writer", f.project, pkg, purchaseDomain.DispatchCommand{Version: 4, Action: "revise", Reason: "Review logistics against corrected dispatch"}); e != nil {
		t.Fatal(e)
	}
	q.Version = 5
	current, e = service.Execute(ctx, "dispatch-writer", f.project, pkg, q)
	if e != nil || current.Record.Source.ClearanceVersion != 8 {
		t.Fatal(e)
	}
	fact(true)
	for _, sql := range []string{"UPDATE purchase_dispatches SET version=version+1 WHERE id=$1", "DELETE FROM purchase_dispatches WHERE id=$1", "DELETE FROM purchase_dispatch_events WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("unaudited dispatch/history mutation accepted")
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_dispatch' AND entity_id=$1", id).Scan(&count); e != nil || count != 6 || len(current.History) != 6 {
		t.Fatal("atomic dispatch history", e, count, len(current.History))
	}
	verifyNativePurchaseSiteReceipt(t, f, ids, ws, pkg, providers, service)
}
