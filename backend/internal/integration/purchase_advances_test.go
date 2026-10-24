package integration

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	inboxout "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	financeout "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/adapters/outbound"
	finance "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	govout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	accessapp "github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
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

// This runs after actual supplier verification, budget and quotation Finance
// approval, LOI, technical clearance, PO release and unconditional acceptance.
func verifyNativePurchaseAdvance(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.IndentsFactory, wp purchaseRepository.WorkflowFactory, ws *purchaseApp.WorkflowService, pkg string, source purchaseRepository.QuotationApprovalsFactory) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	grant := func(user string, codes []string) string {
		t.Helper()
		role := v.ID()
		execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native advance',true)", role, "Native advance "+role)
		for _, code := range codes {
			execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native advance')ON CONFLICT(code)DO NOTHING", v.ID(), code)
			execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
		}
		execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", user, role)
		return role
	}
	procurement := []string{"projects.purchase_advances.read", "projects.purchase_advances.write"}
	grant(f.actor, procurement)
	actor := ids["acceptance-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), procurement...)
	ids["advance-writer"] = actor
	codes := []string{"accounts.purchase_advances.view", "accounts.purchase_advances.read", "accounts.purchase_advances.write", "accounts.purchase_advances.disburse"}
	role := grant(f.outsider, codes)
	reviewer := ids["approval-accounts"]
	reviewer.Permissions = append(append([]string{}, reviewer.Permissions...), codes...)
	ids["advance-finance"] = reviewer
	team := accessapp.New(accessout.NewUnitOfWork(f.pool, accessProviders, purchaseJournal), accessout.NewAuthBridge(ids), accessout.Values{})
	if _, e := team.Save(ctx, "budget-manager", f.project, "", access.Input{UserID: f.outsider, BundleID: role, Responsibility: "Finance advance review and payment", Permissions: codes, Active: true, Reason: "Delegated live advance authority"}); e != nil {
		t.Fatal(e)
	}
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign advance request doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 11, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.AdvancesToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	factory := func(c database.Connection) purchasePorts.AdvancesContext {
		x := p(c)
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		scope := accessout.NewGate(c, accessProviders(c))
		work := purchaseRepository.NewWorkflowSourceReader(c, wp(c))
		orders := purchaseRepository.NewOrdersReleasedReader(c)
		accepted := purchaseRepository.NewAcceptanceAcceptedReader(c, purchaseRepository.AcceptanceProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: work, Orders: orders})
		verified := source(c).Verification
		gate, ok := verified.(masterContracts.VendorVerificationGate)
		if !ok {
			t.Fatal("real supplier verification gate required")
		}
		return purchaseRepository.NewAdvancesContext(purchaseRepository.AdvancesProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: work, Orders: orders, Accepted: accepted, Verification: gate, Verified: verified})
	}
	financeFactory := func(c database.Connection) finance.AdvanceReviewer {
		users := directorUsers(c)
		notices := inboxout.NewWriter(c, govout.NewDirectory(c, users), purchaseJournal(c), inboxout.Values{})
		return financeout.NewAdvanceReviewer(c, users, accessout.NewGate(c, accessProviders(c)), purchaseJournal(c), notices, financeout.Values{})
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.AdvancesService {
		return purchaseApp.NewAdvances(purchaseRepository.NewAdvancesUnitOfWork(f.pool, factory, func(c database.Connection) purchasePorts.AdvancesFinance {
			return purchaseRepository.NewAdvancesFinanceBridge(financeFactory(c))
		}, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	purchaseHTTP.AdvancesRegister(api, service)
	path := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/advances"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		return rec
	}
	current, e := service.Workspace(ctx, "advance-writer", f.project, pkg)
	if e != nil || current.Record != nil || current.Source.Guard() != nil || current.Source.Total != "145.74" {
		t.Fatal("actual accepted advance source", e, current.Source.Guard())
	}
	if request("GET", "", "").Code != 401 {
		t.Fatal("unauthenticated advance exposed")
	}
	if request("POST", "advance-writer", `{"version":0,"action":"prepare","reason":"r","amount":"999"}`).Code != 422 {
		t.Fatal("caller amount accepted")
	}
	details := purchaseDomain.AdvancesDetails{Kind: "payment", RequestNo: "ACTUAL-ADV-1", RequestDate: purchaseDomain.AdvancesToday(v.Now()), Percentage: "30", InvoiceReference: "Actual vendor PI reference", InvoiceDate: current.Source.Date, BankReference: "Actual verified bank reference"}
	checkProgress := func() {
		t.Helper()
		if current.Record == nil {
			t.Fatal("missing actual advance record")
		}
		want := "completed"
		switch current.Record.State {
		case "draft":
			want = "not_started"
		case "rejected":
			want = "blocked"
		case "not_applicable":
			want = "not_applicable"
		}
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			fact, err := purchaseRepository.NewAdvancesProgressReader(c).Progress(txctx, f.project, pkg, current.Source.AdvancesSourceFacts, true)
			if err != nil || fact.State != want || !fact.Current {
				t.Fatal("actual advance reporting", err, fact)
			}
			if want == "completed" && (fact.Snapshot == nil || fact.Snapshot.State != current.Record.State) {
				t.Fatal("report invented advance state", fact)
			}
			changed := current.Source.AdvancesSourceFacts
			changed.OrderVersion++
			stale, err := purchaseRepository.NewAdvancesProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
			if err != nil || stale.State != "blocked" || stale.Current {
				t.Fatal("changed order earned progress", err, stale)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := purchaseRepository.NewAdvancesProgressReader(f.pool).Progress(ctx, f.project, pkg, current.Source.AdvancesSourceFacts, true); err == nil {
			t.Fatal("unbound advance progress reader accepted pool")
		}
	}
	run := func(token string, q purchaseDomain.AdvancesCommand) {
		t.Helper()
		raw, _ := json.Marshal(q)
		resp := request("POST", token, string(raw))
		if resp.Code != 200 {
			t.Fatal("advance command", q.Action, resp.Code, resp.Body.String())
		}
		if e = json.Unmarshal(resp.Body.Bytes(), &current); e != nil {
			t.Fatal(e)
		}
		checkProgress()
	}
	run("advance-writer", purchaseDomain.AdvancesCommand{Action: "prepare", Reason: "Prepare agreed actual advance", Details: &details})
	run("advance-writer", purchaseDomain.AdvancesCommand{Version: 1, Action: "submit", Reason: "Submit exact PO-based advance"})
	if current.Record.State != "requested" || current.Record.Amount != "43.72" || len(current.History) != 2 {
		t.Fatal("exact saved advance submission")
	}

	queue, e := service.Queue(ctx, "advance-finance", 0)
	if e != nil || len(queue.Rows) != 1 || queue.Rows[0].ID != current.Record.ID || queue.Rows[0].Amount != "43.72" {
		t.Fatal("Finance-owned advance queue", e)
	}
	if _, e = service.Queue(ctx, "advance-writer", 0); e == nil {
		t.Fatal("Procurement entered Finance queue")
	}
	if _, e = service.Queue(ctx, "advance-finance", -1); e == nil {
		t.Fatal("Invalid queue offset accepted")
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='accounts.purchase_advances.read')", role)
	queue, e = service.Queue(ctx, "advance-finance", 0)
	if e != nil || len(queue.Rows) != 0 {
		t.Fatal("Revoked live Finance queue leaked record", e)
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='accounts.purchase_advances.read'", role)
	id := current.Record.ID
	if _, e = service.Execute(ctx, "advance-writer", f.project, pkg, purchaseDomain.AdvancesCommand{Version: 2, Action: "approve", Reason: "Self review"}); e == nil {
		t.Fatal("procurement self approved")
	}
	if _, e = service.Workspace(ctx, "advance-finance", f.otherProject, pkg); e == nil {
		t.Fatal("Finance read foreign request")
	}
	if _, e = financeFactory(f.pool).ReviewAdvance(ctx, finance.Actor{ID: f.outsider, Active: true, Permissions: codes}, finance.AdvanceRequest{}); e == nil {
		t.Fatal("Finance accepted unbound pool")
	}
	assertState := func(state string, version int) {
		t.Helper()
		var actual string
		var n int
		if e = f.pool.QueryRow(ctx, "SELECT state,version FROM purchase_advances WHERE id=$1", id).Scan(&actual, &n); e != nil || actual != state || n != version {
			t.Fatal("transaction state", e, actual, n)
		}
	}
	assertDecisionCount := func(want int) {
		t.Helper()
		var n int
		if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM finance_advance_decisions WHERE request_id=$1", id).Scan(&n); e != nil || n != want {
			t.Fatal("decision count", e, n)
		}
	}
	q := purchaseDomain.AdvancesCommand{Version: 2, Action: "approve", Reason: "Independent exact terms review"}
	if leaked, e := failed.Execute(ctx, "advance-finance", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure exposed approval")
	}
	assertState("requested", 2)
	assertDecisionCount(0)
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=false WHERE id=$1", f.manager)
	if _, e = service.Execute(ctx, "advance-finance", f.project, pkg, q); e == nil {
		t.Fatal("missing effective Director approved")
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=true WHERE id=$1", f.manager)
	assertState("requested", 2)
	assertDecisionCount(0)
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='accounts.purchase_advances.write')", role)
	if _, e = service.Execute(ctx, "advance-finance", f.project, pkg, q); e == nil {
		t.Fatal("revoked live Finance authority accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='accounts.purchase_advances.write'", role)
	// A rejected submission remains in immutable Finance/history; revising requires a new decision.
	run("advance-finance", purchaseDomain.AdvancesCommand{Version: 2, Action: "reject", Reason: "Request needs an actual clarification"})
	run("advance-writer", purchaseDomain.AdvancesCommand{Version: 3, Action: "revise", Reason: "Address Finance clarification"})
	noAdvance := purchaseDomain.AdvancesDetails{Kind: "not_applicable", RequestNo: "ACTUAL-NO-ADV", RequestDate: purchaseDomain.AdvancesToday(v.Now()), Percentage: "0", Notes: "Actual PO has no agreed advance"}
	run("advance-writer", purchaseDomain.AdvancesCommand{Version: 4, Action: "prepare", Reason: "Record no advance disposition", Details: &noAdvance})
	run("advance-writer", purchaseDomain.AdvancesCommand{Version: 5, Action: "submit", Reason: "Finance must confirm no advance"})
	run("advance-finance", purchaseDomain.AdvancesCommand{Version: 6, Action: "approve", Reason: "Finance confirms the zero advance disposition"})
	if current.Record.State != "not_applicable" || current.Record.Amount != "0.00" || current.Record.Payment != nil {
		t.Fatal("No advance confirmation fabricated payment")
	}
	if _, e = service.Execute(ctx, "advance-finance", f.project, pkg, purchaseDomain.AdvancesCommand{Version: 7, Action: "disburse", Reason: "Try to pay no advance", Payment: &purchaseDomain.AdvancesPayment{Voucher: "invalid", Date: purchaseDomain.AdvancesToday(v.Now())}}); e == nil {
		t.Fatal("No advance was paid")
	}
	run("advance-writer", purchaseDomain.AdvancesCommand{Version: 7, Action: "revise", Reason: "Correct agreed payment terms before payment"})
	run("advance-writer", purchaseDomain.AdvancesCommand{Version: 8, Action: "prepare", Reason: "Correct actual request evidence", Details: &details})
	run("advance-writer", purchaseDomain.AdvancesCommand{Version: 9, Action: "submit", Reason: "Submit corrected request"})
	run("advance-finance", purchaseDomain.AdvancesCommand{Version: 10, Action: "approve", Reason: "Finance approves agreed exact amount"})
	decision := current.Record.DecisionID
	if current.Record.Amount != "43.72" || current.Record.State != "approved" || decision == "" || current.Record.ReviewedBy != f.outsider {
		t.Fatal("independent Finance proof")
	}
	assertDecisionCount(3)
	var notices int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM communication_notices WHERE decision_id IN(SELECT id FROM finance_advance_decisions WHERE request_id=$1) AND recipient_id=$2", id, f.manager).Scan(&notices); e != nil || notices != 3 {
		t.Fatal("mandatory Director notifications", e, notices)
	}
	// Tampering and completed payment never bypass exact owned evidence.
	for _, sql := range []string{"UPDATE purchase_advances SET data=jsonb_set(data,'{amount}','\"999.00\"') WHERE id=$1", "DELETE FROM purchase_advances WHERE id=$1", "DELETE FROM purchase_advance_events WHERE subject_id=$1", "UPDATE finance_advance_decisions SET amount=999 WHERE request_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("immutable exact audit bypass", sql)
		}
	}
	pay := purchaseDomain.AdvancesCommand{Version: 11, Action: "disburse", Reason: "Record actual authorised payment", Payment: &purchaseDomain.AdvancesPayment{Voucher: "ACTUAL-UTR-1", Date: purchaseDomain.AdvancesToday(v.Now())}}
	if leaked, e := failed.Execute(ctx, "advance-finance", f.project, pkg, pay); e == nil || leaked.Record != nil {
		t.Fatal("late payment event leaked success")
	}
	assertState("approved", 11)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	if _, e = service.Execute(ctx, "advance-finance", f.project, pkg, pay); e == nil {
		t.Fatal("held project payment accepted")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := service.Execute(ctx, "advance-finance", f.project, pkg, pay)
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("concurrent duplicate disbursement", wins)
	}
	current, e = service.Workspace(ctx, "advance-finance", f.project, pkg)
	if e != nil || current.Record.State != "disbursed" || current.Record.Payment.Voucher != "ACTUAL-UTR-1" || current.Record.DecisionID != decision || len(current.History) != 12 || len(current.Actions) != 0 {
		t.Fatal("actual immutable payment", e)
	}
	checkProgress()
	if _, e = service.Execute(ctx, "advance-writer", f.project, pkg, purchaseDomain.AdvancesCommand{Version: 12, Action: "revise", Reason: "Try to edit completed payment"}); e == nil {
		t.Fatal("completed payment editable")
	}
	var central int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_advances' AND entity_id=$1", id).Scan(&central); e != nil || central != 12 {
		t.Fatal("exact central project history", e, central)
	}
}
