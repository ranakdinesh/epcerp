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

func verifyNativePurchaseAcceptance(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.IndentsFactory, wp purchaseRepository.WorkflowFactory, ws *purchaseApp.WorkflowService, pkg string, financeFactory purchaseRepository.QuotationApprovalsFactory) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_acceptance.read", "projects.purchase_acceptance.write"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native acceptance',true)", role, "Native acceptance "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native acceptance')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	actor := ids["po-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), codes...)
	ids["acceptance-writer"] = actor
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign actual acceptance doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 10, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.AcceptanceToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	providers := func(c database.Connection) purchaseRepository.AcceptanceProviders {
		x := p(c)
		return purchaseRepository.AcceptanceProviders{Projects: x.Projects, Purchase: x.Scope, Access: accessout.NewGate(c, accessProviders(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Work: purchaseRepository.NewWorkflowSourceReader(c, wp(c)), Orders: purchaseRepository.NewOrdersReleasedReader(c)}
	}
	factory := func(c database.Connection) purchasePorts.AcceptanceContext {
		return purchaseRepository.NewAcceptanceContext(providers(c))
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.AcceptanceService {
		return purchaseApp.NewAcceptance(purchaseRepository.NewAcceptanceUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	purchaseHTTP.AcceptanceRegister(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/acceptance"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		return rec
	}
	initial, e := service.Workspace(ctx, "acceptance-writer", f.project, pkg)
	if e != nil || initial.Record != nil || initial.Source.Guard() != nil {
		t.Fatal("actual released PO source", e, initial.Source)
	}
	if request("GET", "", "").Code != 401 {
		t.Fatal("unauthenticated acceptance exposed")
	}
	if request("POST", "acceptance-writer", `{"version":0,"action":"record","reason":"r","response":{"status":"pending"},"supplier_id":"forged"}`).Code != 422 {
		t.Fatal("forged source accepted")
	}
	q := purchaseDomain.AcceptanceCommand{Action: "record", Reason: "Received actual vendor acknowledgement", Response: purchaseDomain.AcceptanceResponse{Status: "accepted", AcceptedDate: purchaseDomain.AcceptanceToday(v.Now()), SignatoryName: "Actual vendor signatory", DocumentReference: "Actual vendor reference", Remarks: "Confirmed unchanged terms"}}
	if leaked, e := failed.Execute(ctx, "acceptance-writer", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure leaked acceptance")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_acceptances").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed transaction persisted acceptance", e)
	}
	raw, _ := json.Marshal(q)
	response := request("POST", "acceptance-writer", string(raw))
	var current purchaseDomain.AcceptanceWorkspace
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.Record == nil || current.Record.Response.Status != "accepted" || current.Record.Order.ID != initial.Source.Order.ID || len(current.History) != 1 {
		t.Fatal("owned acceptance HTTP", response.Code, response.Body.String())
	}
	id := current.Record.ID
	checkProgress := func(withoutExceptions bool) {
		t.Helper()
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			fact, err := purchaseRepository.NewAcceptanceProgressReader(c, factory(c)).Progress(txctx, f.project, pkg)
			if err != nil || fact.State != "completed" || !fact.Current || fact.WithoutExceptions != withoutExceptions || fact.Snapshot == nil || fact.Snapshot.ID != id {
				t.Fatal("current vendor acknowledgement progress", err, fact)
			}
			foreign, err := purchaseRepository.NewAcceptanceProgressReader(c, factory(c)).Progress(txctx, f.otherProject, pkg)
			if err != nil || foreign.State != "not_started" || foreign.Snapshot != nil || foreign.Current {
				t.Fatal("foreign project leaked acknowledgement progress", err, foreign)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	checkProgress(true)
	if _, err := purchaseRepository.NewAcceptanceProgressReader(f.pool, factory(f.pool)).Progress(ctx, f.project, pkg); err == nil {
		t.Fatal("unbound acknowledgement progress reader accepted pool")
	}
	if _, e = purchaseRepository.NewAcceptanceAcceptedReader(f.pool, providers(f.pool)).Accepted(ctx, f.project, pkg); e == nil {
		t.Fatal("unbound accepted reader accepted pool")
	}
	if e = database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		fact, err := purchaseRepository.NewAcceptanceAcceptedReader(c, providers(c)).Accepted(txctx, f.project, pkg)
		if err != nil || fact == nil || fact.ID != id || fact.Order.ID != initial.Source.Order.ID {
			t.Fatal("current unconditional acceptance fact", err)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}

	verifyNativePurchaseAdvance(t, f, ids, p, wp, ws, pkg, financeFactory)
	verifyNativePurchaseInspection(t, f, ids, p, wp, ws, pkg, financeFactory)
	q.Version = 1
	q.Response.Status = "accepted_with_exceptions"
	q.Response.Remarks = "Actual unresolved delivery exception"
	q.Reason = "Vendor clarified response"
	if _, e = service.Execute(ctx, "acceptance-writer", f.otherProject, pkg, q); e == nil {
		t.Fatal("foreign response saved")
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_acceptance.write')", role)
	if _, e = service.Execute(ctx, "acceptance-writer", f.project, pkg, q); e == nil {
		t.Fatal("revoked permission accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_acceptance.write'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "acceptance-writer", f.project, pkg)
	if e != nil || held.Record == nil || len(held.Actions) > 0 {
		t.Fatal("held response lost history or retained actions", e)
	}
	if _, e = service.Execute(ctx, "acceptance-writer", f.project, pkg, q); e == nil {
		t.Fatal("held project accepted write")
	}
	checkProgress(true)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, e := service.Execute(ctx, "acceptance-writer", f.project, pkg, q)
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
		t.Fatal("concurrent corrections", wins)
	}
	current, e = service.Workspace(ctx, "acceptance-writer", f.project, pkg)
	if e != nil || current.Record.Version != 2 || len(current.History) != 2 || current.Record.Response.Status != "accepted_with_exceptions" {
		t.Fatal("corrected acceptance", e)
	}
	checkProgress(false)
	if _, e = purchaseDomain.AcceptanceAcceptedFact(current.Record, current.Source); e == nil {
		t.Fatal("exceptions passed acceptance gate")
	}

	if e = database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		fact, err := purchaseRepository.NewAcceptanceAcceptedReader(c, providers(c)).Accepted(txctx, f.project, pkg)
		if err == nil || fact != nil {
			t.Fatal("exception response published as unconditional acceptance")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	verifyInspectionAfterAcceptanceChange(t, f, ids, p, wp, pkg)
	for _, sql := range []string{"UPDATE purchase_acceptances SET version=version+1 WHERE id=$1", "DELETE FROM purchase_acceptances WHERE id=$1", "DELETE FROM purchase_acceptance_events WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("unaudited acceptance/history mutation allowed")
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_acceptance' AND entity_id=$1", id).Scan(&count); e != nil || count != 2 {
		t.Fatal("atomic project acceptance history", e, count)
	}
}
