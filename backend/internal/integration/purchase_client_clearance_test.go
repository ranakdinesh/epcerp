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

func verifyNativeClientClearance(t *testing.T, f purchaseFixture, ids projectIdentities, ws *purchaseApp.WorkflowService, pkg string, qp func(database.Connection) purchaseRepository.QualityProviders, quality *purchaseApp.QualityService) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_client_clearance.read", "projects.purchase_client_clearance.write"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native MDCC',true)", role, "Native MDCC "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native MDCC')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	actor := ids["quality-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), codes...)
	ids["client-clearance-writer"] = actor
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign actual MDCC doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 15, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.ClientClearanceToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	providers := func(c database.Connection) purchaseRepository.ClientClearanceProviders {
		x := qp(c)
		return purchaseRepository.ClientClearanceProviders{Projects: x.Projects, Purchase: x.Purchase, Access: x.Access, Users: x.Users, Work: x.Work, Quality: purchaseRepository.NewQualityVerifiedReader(c, x)}
	}
	factory := func(c database.Connection) purchasePorts.ClientClearanceContext {
		return purchaseRepository.NewClientClearanceContext(providers(c))
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.ClientClearanceService {
		return purchaseApp.NewClientClearance(purchaseRepository.NewClientClearanceUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	purchaseHTTP.ClientClearanceRegister(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/client-clearance"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r := httptest.NewRecorder()
		api.ServeHTTP(r, req)
		return r
	}
	initial, e := service.Workspace(ctx, "client-clearance-writer", f.project, pkg)
	if e != nil || initial.Record != nil || initial.Source.QualityVersion != 23 || initial.Source.ReportVersion != 8 || initial.Source.Quantity != "1.000001" || initial.Source.Guard() != nil {
		t.Fatal("actual verified-quality source", e, initial.Source)
	}
	if request("GET", "", "").Code != 401 || request("POST", "client-clearance-writer", `{"version":0,"action":"record","reason":"r","clearance":{"status":"pending","cleared_quantity":"forged"}}`).Code != 422 {
		t.Fatal("MDCC HTTP protection")
	}
	q := purchaseDomain.ClientClearanceCommand{Action: "record", Reason: "Prepare actual client MDCC", Clearance: &purchaseDomain.ClientClearanceClearanceInput{Status: "pending"}}
	if leaked, e := failed.Execute(ctx, "client-clearance-writer", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure leaked MDCC")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_client_clearances").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed MDCC persisted", e)
	}
	raw, _ := json.Marshal(q)
	r := request("POST", "client-clearance-writer", string(raw))
	var current purchaseDomain.ClientClearanceWorkspace
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &current) != nil || current.Record == nil || current.Record.Clearance.Status != "pending" || len(current.History) != 1 {
		t.Fatal("native MDCC HTTP", r.Code, r.Body.String())
	}
	checkProgress := func(want string) {
		t.Helper()
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			p, err := purchaseRepository.NewClientClearanceProgressReader(c).Progress(txctx, f.project, pkg, current.Source.ClientClearanceSourceFacts, true)
			if err != nil || p.State != want || !p.Current {
				t.Fatal("actual purchase_client_clearance progress", err, p)
			}
			if want == "completed" && (p.Snapshot == nil || p.Snapshot.ID != current.Record.ID) {
				t.Fatal("completion lacks actual owned evidence", p)
			}
			changed := current.Source.ClientClearanceSourceFacts
			changed.QualityVersion++
			stale, err := purchaseRepository.NewClientClearanceProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
			if err != nil || stale.State != "blocked" || stale.Current {
				t.Fatal("changed source earned completion", err, stale)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := purchaseRepository.NewClientClearanceProgressReader(f.pool).Progress(ctx, f.project, pkg, current.Source.ClientClearanceSourceFacts, true); err == nil {
			t.Fatal("unbound progress reader accepted pool")
		}
	}
	checkProgress("not_started")
	q.Version = 1
	q.Clearance = &purchaseDomain.ClientClearanceClearanceInput{Status: "cleared", Reference: "Actual MDCC", Date: purchaseDomain.ClientClearanceToday(v.Now()), Authority: "Actual client consultant", Remarks: "Actual client instructions", DocumentReference: "Actual MDCC reference"}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_client_clearance.write')", role)
	if _, e = service.Execute(ctx, "client-clearance-writer", f.project, pkg, q); e == nil {
		t.Fatal("revoked MDCC permission accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_client_clearance.write'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "client-clearance-writer", f.project, pkg)
	if e != nil || held.Record == nil || len(held.Actions) > 0 {
		t.Fatal("held MDCC lost history or permits actions", e)
	}
	if _, e = service.Execute(ctx, "client-clearance-writer", f.project, pkg, q); e == nil {
		t.Fatal("held client clearance saved")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := service.Execute(ctx, "client-clearance-writer", f.project, pkg, q)
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
		t.Fatal("MDCC concurrent clearance winners", wins)
	}
	current, e = service.Workspace(ctx, "client-clearance-writer", f.project, pkg)
	if e != nil || current.Record.Version != 2 || current.Record.Clearance.Reference != "Actual MDCC" {
		t.Fatal(e)
	}
	checkProgress("completed")
	id := current.Record.ID
	if _, e = purchaseRepository.NewClientClearanceClearedReader(f.pool, providers(f.pool)).Cleared(ctx, f.project, pkg); e == nil {
		t.Fatal("unbound MDCC consumer accepted")
	}
	fact := func(want bool) {
		t.Helper()
		if e := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			r, e := purchaseRepository.NewClientClearanceClearedReader(c, providers(c)).Cleared(txctx, f.project, pkg)
			if want && (e != nil || r == nil || r.ID != id) || !want && e == nil {
				t.Fatal("current MDCC fact", want, e)
			}
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	fact(true)
	if _, e = service.Execute(ctx, "client-clearance-writer", f.project, pkg, q); e == nil {
		t.Fatal("final MDCC overwritten")
	}
	if leaked, e := failed.Execute(ctx, "client-clearance-writer", f.project, pkg, purchaseDomain.ClientClearanceCommand{Version: 2, Action: "revise", Reason: "Must roll back actual correction"}); e == nil || leaked.Record != nil {
		t.Fatal("failed revision escaped")
	}
	fact(true)
	current, e = service.Execute(ctx, "client-clearance-writer", f.project, pkg, purchaseDomain.ClientClearanceCommand{Version: 2, Action: "revise", Reason: "Actual client MDCC correction"})
	if e != nil || current.Record.Clearance.Reference != "" || current.Record.Clearance.Status != "pending" || current.Record.Revision != 2 {
		t.Fatal("MDCC revision reset", e)
	}
	checkProgress("not_started")
	fact(false)
	q.Version = 3
	if _, e = service.Execute(ctx, "client-clearance-writer", f.project, pkg, q); e != nil {
		t.Fatal(e)
	}
	fact(true)
	// Change current quality through its owning application, never raw SQL.
	old, e := quality.Workspace(ctx, "quality-writer", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = quality.Execute(ctx, "quality-writer", f.project, pkg, purchaseDomain.QualityCommand{Version: old.Record.Version, Action: "revoke", Code: "TC", Reason: "Actual certificate re-review"}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e := service.Workspace(ctx, "client-clearance-writer", f.project, pkg)
	if e != nil || blocked.Record == nil || len(blocked.History) != 4 || len(blocked.Actions) != 0 {
		t.Fatal("invalid quality lost MDCC history", e)
	}
	if _, e = quality.Execute(ctx, "quality-writer", f.project, pkg, purchaseDomain.QualityCommand{Version: old.Record.Version + 1, Action: "verify", Code: "TC", Reason: "Actual certificate reverified"}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e = service.Workspace(ctx, "client-clearance-writer", f.project, pkg)
	if e != nil || len(blocked.Actions) != 1 || blocked.Actions[0] != "revise" {
		t.Fatal("changed quality reused old MDCC", e)
	}
	if _, e = service.Execute(ctx, "client-clearance-writer", f.project, pkg, purchaseDomain.ClientClearanceCommand{Version: 4, Action: "revise", Reason: "Review client MDCC against renewed quality"}); e != nil {
		t.Fatal(e)
	}
	q.Version = 5
	current, e = service.Execute(ctx, "client-clearance-writer", f.project, pkg, q)
	if e != nil || current.Record.Source.QualityVersion != 25 {
		t.Fatal(e)
	}
	fact(true)
	for _, sql := range []string{"UPDATE purchase_client_clearances SET version=version+1 WHERE id=$1", "DELETE FROM purchase_client_clearances WHERE id=$1", "DELETE FROM purchase_client_clearance_events WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("unaudited MDCC/history mutation accepted")
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_client_clearance' AND entity_id=$1", id).Scan(&count); e != nil || count != 6 || len(current.History) != 6 {
		t.Fatal("atomic MDCC history", e, count, len(current.History))
	}
	verifyNativePurchaseDispatch(t, f, ids, ws, pkg, providers, service)
}
