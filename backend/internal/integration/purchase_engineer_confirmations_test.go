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

func verifyNativePurchaseEngineerConfirmation(t *testing.T, f purchaseFixture, ids projectIdentities, ws *purchaseApp.WorkflowService, pkg string, gp func(database.Connection) purchaseRepository.GRNsProviders, grn *purchaseApp.GRNsService) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_engineer_confirmations.read", "projects.purchase_engineer_confirmations.write"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native engineer confirmation',true)", role, "Native engineer "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native engineer confirmation')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	actor := ids["grn-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), codes...)
	ids["engineer-writer"] = actor
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign actual dispatch doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 19, Team: "Project engineer", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.EngineerConfirmationsToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	providers := func(c database.Connection) purchaseRepository.EngineerConfirmationsProviders {
		x := gp(c)
		return purchaseRepository.EngineerConfirmationsProviders{Projects: x.Projects, Purchase: x.Purchase, Access: x.Access, Users: x.Users, Work: x.Work, GRN: purchaseRepository.NewGRNsIssuedReader(c, x)}
	}
	factory := func(c database.Connection) purchasePorts.EngineerConfirmationsContext {
		return purchaseRepository.NewEngineerConfirmationsContext(providers(c))
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.EngineerConfirmationsService {
		return purchaseApp.NewEngineerConfirmations(purchaseRepository.NewEngineerConfirmationsUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	purchaseHTTP.EngineerConfirmationsRegister(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/engineer-confirmation"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r := httptest.NewRecorder()
		api.ServeHTTP(r, req)
		return r
	}
	initial, e := service.Workspace(ctx, "engineer-writer", f.project, pkg)
	if e != nil || initial.Record != nil || initial.Source.GRNVersion != 6 || initial.Source.ReportVersion != 8 || initial.Source.Quantity != "1.000001" || initial.Source.Guard() != nil {
		t.Fatal("actual verified-quality source", e, initial.Source)
	}
	if request("GET", "", "").Code != 401 || request("POST", "engineer-writer", `{"version":0,"action":"record","reason":"r","confirmation":{"status":"pending","cleared_quantity":"forged"}}`).Code != 422 {
		t.Fatal("dispatch HTTP protection")
	}
	q := purchaseDomain.EngineerConfirmationsCommand{Action: "record", Reason: "Prepare actual engineer confirmation", Confirmation: &purchaseDomain.EngineerConfirmationsConfirmationInput{Status: "pending"}}
	if leaked, e := failed.Execute(ctx, "engineer-writer", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure leaked dispatch")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_engineer_confirmations").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed dispatch persisted", e)
	}
	raw, _ := json.Marshal(q)
	r := request("POST", "engineer-writer", string(raw))
	var current purchaseDomain.EngineerConfirmationsWorkspace
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &current) != nil || current.Record == nil || current.Record.Confirmation.Status != "pending" || len(current.History) != 1 {
		t.Fatal("native engineer confirmation HTTP", r.Code, r.Body.String())
	}
	checkProgress := func(want string) {
		t.Helper()
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			p, err := purchaseRepository.NewEngineerConfirmationsProgressReader(c).Progress(txctx, f.project, pkg, current.Source.EngineerConfirmationsSourceFacts, true)
			if err != nil || p.State != want || !p.Current {
				t.Fatal("actual purchase_engineer_confirmations progress", err, p)
			}
			if want == "completed" && (p.Snapshot == nil || p.Snapshot.ID != current.Record.ID) {
				t.Fatal("completion lacks actual owned evidence", p)
			}
			changed := current.Source.EngineerConfirmationsSourceFacts
			changed.GRNVersion++
			stale, err := purchaseRepository.NewEngineerConfirmationsProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
			if err != nil || stale.State != "blocked" || stale.Current {
				t.Fatal("changed source earned completion", err, stale)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := purchaseRepository.NewEngineerConfirmationsProgressReader(f.pool).Progress(ctx, f.project, pkg, current.Source.EngineerConfirmationsSourceFacts, true); err == nil {
			t.Fatal("unbound progress reader accepted pool")
		}
	}
	checkProgress("not_started")
	q.Version = 1
	q.Confirmation = &purchaseDomain.EngineerConfirmationsConfirmationInput{Status: "confirmed", EngineerName: "Actual engineer", Date: purchaseDomain.EngineerConfirmationsToday(v.Now()), Designation: "Project engineer", MBookReference: "MB-actual", WorkPackageReference: "Actual work package", Remarks: "Actual review"}
	for _, field := range []string{"engineer_name", "confirmation_date"} {
		invalid := q
		input := *q.Confirmation
		invalid.Confirmation = &input
		if field == "engineer_name" {
			input.EngineerName = " "
		} else {
			input.Date = ""
		}
		raw, _ := json.Marshal(invalid)
		response := request("POST", "engineer-writer", string(raw))
		if response.Code != 422 || !strings.Contains(response.Body.String(), field) {
			t.Fatal("server required field", field, response.Code, response.Body.String())
		}
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_engineer_confirmations.write')", role)
	if _, e = service.Execute(ctx, "engineer-writer", f.project, pkg, q); e == nil {
		t.Fatal("revoked dispatch permission accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_engineer_confirmations.write'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "engineer-writer", f.project, pkg)
	if e != nil || held.Record == nil || len(held.Actions) > 0 {
		t.Fatal("held dispatch lost history or permits actions", e)
	}
	if _, e = service.Execute(ctx, "engineer-writer", f.project, pkg, q); e == nil {
		t.Fatal("held client clearance saved")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := service.Execute(ctx, "engineer-writer", f.project, pkg, q)
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
	current, e = service.Workspace(ctx, "engineer-writer", f.project, pkg)
	if e != nil || current.Record.Version != 2 || current.Record.Confirmation.EngineerName != "Actual engineer" {
		t.Fatal(e)
	}
	checkProgress("completed")
	id := current.Record.ID
	if _, e = purchaseRepository.NewEngineerConfirmationsConfirmedReader(f.pool, providers(f.pool)).Confirmed(ctx, f.project, pkg); e == nil {
		t.Fatal("unbound dispatch consumer accepted")
	}
	fact := func(want bool) {
		t.Helper()
		if e := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			r, e := purchaseRepository.NewEngineerConfirmationsConfirmedReader(c, providers(c)).Confirmed(txctx, f.project, pkg)
			if want && (e != nil || r == nil || r.ID != id) || !want && e == nil {
				t.Fatal("current physical receipt fact", want, e)
			}
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	fact(true)
	if _, e = service.Execute(ctx, "engineer-writer", f.project, pkg, q); e == nil {
		t.Fatal("final dispatch overwritten")
	}
	if leaked, e := failed.Execute(ctx, "engineer-writer", f.project, pkg, purchaseDomain.EngineerConfirmationsCommand{Version: 2, Action: "revise", Reason: "Must roll back actual correction"}); e == nil || leaked.Record != nil {
		t.Fatal("failed revision escaped")
	}
	fact(true)
	current, e = service.Execute(ctx, "engineer-writer", f.project, pkg, purchaseDomain.EngineerConfirmationsCommand{Version: 2, Action: "revise", Reason: "Actual GRN correction"})
	if e != nil || current.Record.Confirmation.EngineerName != "" || current.Record.Confirmation.Status != "pending" || current.Record.Revision != 2 {
		t.Fatal("dispatch revision reset", e)
	}
	checkProgress("not_started")
	fact(false)
	q.Version = 3
	if _, e = service.Execute(ctx, "engineer-writer", f.project, pkg, q); e != nil {
		t.Fatal(e)
	}
	fact(true)
	// Revise and re-clear the actual engineer confirmation through its owning application.
	old, e := grn.Workspace(ctx, "grn-writer", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = grn.Execute(ctx, "grn-writer", f.project, pkg, purchaseDomain.GRNsCommand{Version: old.Record.Version, Action: "revise", Reason: "Actual GRN rework"}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e := service.Workspace(ctx, "engineer-writer", f.project, pkg)
	if e != nil || blocked.Record == nil || len(blocked.History) != 4 || len(blocked.Actions) != 0 {
		t.Fatal("GRN rework lost engineer history", e)
	}
	next := old.Record.GRN.GRNsGRNInput
	next.Number = "Corrected actual GRN"
	if _, e = grn.Execute(ctx, "grn-writer", f.project, pkg, purchaseDomain.GRNsCommand{Version: old.Record.Version + 1, Action: "record", Reason: "Actual corrected GRN", GRN: &next}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e = service.Workspace(ctx, "engineer-writer", f.project, pkg)
	if e != nil || len(blocked.Actions) != 1 || blocked.Actions[0] != "revise" {
		t.Fatal("renewed GRN reused obsolete confirmation", e)
	}
	if _, e = service.Execute(ctx, "engineer-writer", f.project, pkg, purchaseDomain.EngineerConfirmationsCommand{Version: 4, Action: "revise", Reason: "Review logistics against corrected dispatch"}); e != nil {
		t.Fatal(e)
	}
	q.Version = 5
	current, e = service.Execute(ctx, "engineer-writer", f.project, pkg, q)
	if e != nil || current.Record.Source.GRNVersion != 8 {
		t.Fatal(e)
	}
	fact(true)
	for _, sql := range []string{"UPDATE purchase_engineer_confirmations SET version=version+1 WHERE id=$1", "DELETE FROM purchase_engineer_confirmations WHERE id=$1", "DELETE FROM purchase_engineer_confirmations_events WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("unaudited dispatch/history mutation accepted")
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_engineer_confirmations' AND entity_id=$1", id).Scan(&count); e != nil || count != 6 || len(current.History) != 6 {
		t.Fatal("atomic dispatch history", e, count, len(current.History))
	}
	verifyNativePurchaseAccountsAcceptance(t, f, ids, ws, pkg, providers, service)
}
