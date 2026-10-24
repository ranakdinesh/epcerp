package integration

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	inboxout "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	in "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/application"
	d "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/ports"
	govout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	accessapp "github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"

	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func verifyNativePurchaseAccountsAcceptance(t *testing.T, f purchaseFixture, ids projectIdentities, ws *purchaseApp.WorkflowService, pkg string, ep func(database.Connection) purchaseRepository.EngineerConfirmationsProviders, engineer *purchaseApp.EngineerConfirmationsService) {
	t.Helper()
	ctx := t.Context()
	v := out.Values{}
	role := v.ID()
	codes := []string{"accounts.purchase_acceptance.view", "accounts.purchase_acceptance.read", "accounts.purchase_acceptance.write"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native engineer acceptance',true)", role, "Native Accounts "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native engineer acceptance')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.outsider, role)
	finance := ids["advance-finance"]
	finance.Permissions = append(append([]string{}, finance.Permissions...), codes...)
	ids["accounts-finance"] = finance
	team := accessapp.New(accessout.NewUnitOfWork(f.pool, accessProviders, purchaseJournal), accessout.NewAuthBridge(ids), accessout.Values{})
	if _, e := team.Save(ctx, "budget-manager", f.project, "", access.Input{UserID: f.outsider, BundleID: role, Responsibility: "Final Accounts acceptance", Permissions: codes, Active: true, Reason: "Delegate explicit Finance authority"}); e != nil {
		t.Fatal(e)
	}
	// A procurement doer with all three global codes still has no Finance project profile.
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	task := ids["engineer-writer"]
	task.Permissions = append(append([]string{}, task.Permissions...), codes...)
	ids["accounts-task"] = task
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign actual dispatch doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 20, Team: "Project engineer", DoerID: f.actor, ResolverID: f.manager, PlannedDate: d.Today(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	providers := func(c database.Connection) out.Providers {
		x := ep(c)
		return out.Providers{Projects: x.Projects, Purchase: x.Purchase, Access: x.Access, Users: x.Users, Work: x.Work, Engineer: purchaseRepository.NewEngineerConfirmationsConfirmedReader(c, x), Orders: purchaseRepository.NewOrdersReleasedReader(c)}
	}
	factory := func(c database.Connection) ports.Context { return out.NewContext(providers(c)) }
	build := func(j func(database.Connection) events.Writer) *app.Service {
		return app.New(out.NewUnitOfWork(f.pool, factory, j, func(c database.Connection) inbox.Writer {
			return inboxout.NewWriter(c, govout.NewDirectory(c, directorUsers(c)), j(c), inboxout.Values{})
		}), out.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	in.Register(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/accounts-acceptance"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r := httptest.NewRecorder()
		api.ServeHTTP(r, req)
		return r
	}
	initial, e := service.Workspace(ctx, "accounts-finance", f.project, pkg)
	if e != nil || initial.Record != nil || initial.Source.EngineerVersion != 6 || initial.Source.ReportVersion != 8 || initial.Source.Quantity != "1.000001" || initial.Source.Guard() != nil {
		t.Fatal("actual verified-quality source", e, initial.Source)
	}
	if request("GET", "", "").Code != 401 || request("POST", "accounts-finance", `{"version":0,"action":"record","reason":"r","acceptance":{"status":"pending","cleared_quantity":"forged"}}`).Code != 422 {
		t.Fatal("dispatch HTTP protection")
	}
	if w, e := service.Workspace(ctx, "accounts-task", f.project, pkg); e != nil || !w.TaskOnly || len(w.Actions) != 0 {
		t.Fatal("task read/scope", e, w)
	}
	if _, e := service.Execute(ctx, "accounts-task", f.project, pkg, d.Command{Action: "record", Reason: "Task bypass", Acceptance: &d.AcceptanceInput{Status: "pending"}}); e != d.Forbidden {
		t.Fatal("assigned procurement task became Finance authority", e)
	}
	q := d.Command{Action: "record", Reason: "Prepare actual engineer acceptance", Acceptance: &d.AcceptanceInput{Status: "pending"}}
	if leaked, e := failed.Execute(ctx, "accounts-finance", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure leaked dispatch")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM finance_accounts_acceptance").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed dispatch persisted", e)
	}
	raw, _ := json.Marshal(q)
	r := request("POST", "accounts-finance", string(raw))
	var current d.Workspace
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &current) != nil || current.Record == nil || current.Record.Acceptance.Status != "pending" || len(current.History) != 1 {
		t.Fatal("native engineer acceptance HTTP", r.Code, r.Body.String())
	}
	checkProgress := func(want string) {
		t.Helper()
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			p, err := out.NewProgressReader(c).Progress(txctx, f.project, pkg, current.Source.SourceFacts, true)
			if err != nil || p.State != want || !p.Current {
				t.Fatal("actual finance_accounts_acceptance progress", err, p)
			}
			if want == "completed" && (p.Snapshot == nil || p.Snapshot.ID != current.Record.ID) {
				t.Fatal("completion lacks actual owned evidence", p)
			}
			changed := current.Source.SourceFacts
			changed.EngineerVersion++
			stale, err := out.NewProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
			if err != nil || stale.State != "blocked" || stale.Current {
				t.Fatal("changed source earned completion", err, stale)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := out.NewProgressReader(f.pool).Progress(ctx, f.project, pkg, current.Source.SourceFacts, true); err == nil {
			t.Fatal("unbound progress reader accepted pool")
		}
	}
	checkProgress("not_started")
	q.Version = 1
	q.Acceptance = &d.AcceptanceInput{Status: "accepted", OfficerName: "Actual Accounts officer", Date: d.Today(v.Now()), VoucherReference: "Actual voucher", BookedAmount: "0", Notes: "Actual Accounts review"}
	for _, field := range []string{"officer_name", "acceptance_date"} {
		invalid := q
		input := *q.Acceptance
		invalid.Acceptance = &input
		if field == "officer_name" {
			input.OfficerName = " "
		} else {
			input.Date = ""
		}
		raw, _ := json.Marshal(invalid)
		response := request("POST", "accounts-finance", string(raw))
		if response.Code != 422 || !strings.Contains(response.Body.String(), field) {
			t.Fatal("server required field", field, response.Code, response.Body.String())
		}
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='accounts.purchase_acceptance.write')", role)
	if _, e = service.Execute(ctx, "accounts-finance", f.project, pkg, q); e == nil {
		t.Fatal("revoked dispatch permission accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='accounts.purchase_acceptance.write'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "accounts-finance", f.project, pkg)
	if e != nil || held.Record == nil || len(held.Actions) > 0 {
		t.Fatal("held dispatch lost history or permits actions", e)
	}
	if _, e = service.Execute(ctx, "accounts-finance", f.project, pkg, q); e == nil {
		t.Fatal("held client clearance saved")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=false WHERE id=$1", f.manager)
	if failed, e := service.Execute(ctx, "accounts-finance", f.project, pkg, q); e == nil || failed.Record != nil {
		t.Fatal("missing effective Director accepted", e)
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=true WHERE id=$1", f.manager)
	wBefore, e := service.Workspace(ctx, "accounts-finance", f.project, pkg)
	if e != nil || wBefore.Record.Version != 1 || len(wBefore.History) != 1 {
		t.Fatal("Director failure persisted acceptance", e)
	}
	// Successful-looking delivery without a durable row must fail the deferred guard.
	ghost := app.New(out.NewUnitOfWork(f.pool, factory, purchaseJournal, func(database.Connection) inbox.Writer { return accountsSilentNotice{} }), out.NewAuthBridge(ids), v)
	if leaked, e := ghost.Execute(ctx, "accounts-finance", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("missing durable Director notice committed", e)
	}
	var profileID string
	var profileVersion int32
	if e = f.pool.QueryRow(ctx, "SELECT id::text,version FROM project_responsibility_profiles WHERE project_id=$1 AND user_id=$2 AND bundle_id=$3", f.project, f.outsider, role).Scan(&profileID, &profileVersion); e != nil {
		t.Fatal(e)
	}
	profile, e := team.Save(ctx, "budget-manager", f.project, profileID, access.Input{Version: profileVersion, UserID: f.outsider, BundleID: role, Responsibility: "Final Accounts acceptance", Permissions: codes, Active: false, Reason: "Revoke Finance project delegation"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.Execute(ctx, "accounts-finance", f.project, pkg, q); e == nil {
		t.Fatal("revoked Finance project delegation accepted")
	}
	if _, e = team.Save(ctx, "budget-manager", f.project, profileID, access.Input{Version: profile.Version, UserID: f.outsider, BundleID: role, Responsibility: "Final Accounts acceptance", Permissions: codes, Active: true, Reason: "Restore scoped Finance delegation"}); e != nil {
		t.Fatal(e)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := service.Execute(ctx, "accounts-finance", f.project, pkg, q)
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
	current, e = service.Workspace(ctx, "accounts-finance", f.project, pkg)
	if e != nil || current.Record.Version != 2 || current.Record.Acceptance.OfficerName != "Actual Accounts officer" {
		t.Fatal(e)
	}
	checkProgress("completed")
	id := current.Record.ID
	if _, e = out.NewAcceptedReader(f.pool, providers(f.pool)).Accepted(ctx, f.project, pkg); e == nil {
		t.Fatal("unbound dispatch consumer accepted")
	}
	fact := func(want bool) {
		t.Helper()
		if e := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			r, e := out.NewAcceptedReader(c, providers(c)).Accepted(txctx, f.project, pkg)
			if want && (e != nil || r == nil || r.ID != id) || !want && e == nil {
				t.Fatal("current physical receipt fact", want, e)
			}
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	fact(true)
	if _, e = service.Execute(ctx, "accounts-finance", f.project, pkg, q); e == nil {
		t.Fatal("final dispatch overwritten")
	}
	if leaked, e := failed.Execute(ctx, "accounts-finance", f.project, pkg, d.Command{Version: 2, Action: "revise", Reason: "Must roll back actual correction"}); e == nil || leaked.Record != nil {
		t.Fatal("failed revision escaped")
	}
	fact(true)
	current, e = service.Execute(ctx, "accounts-finance", f.project, pkg, d.Command{Version: 2, Action: "revise", Reason: "Actual GRN correction"})
	if e != nil || current.Record.Acceptance.OfficerName != "" || current.Record.Acceptance.Status != "pending" || current.Record.Revision != 2 {
		t.Fatal("dispatch revision reset", e)
	}
	checkProgress("not_started")
	fact(false)
	q.Version = 3
	if _, e = service.Execute(ctx, "accounts-finance", f.project, pkg, q); e != nil {
		t.Fatal(e)
	}
	fact(true)
	// Revise and re-clear the actual engineer acceptance through its owning application.
	old, e := engineer.Workspace(ctx, "engineer-writer", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = engineer.Execute(ctx, "engineer-writer", f.project, pkg, purchaseDomain.EngineerConfirmationsCommand{Version: old.Record.Version, Action: "revise", Reason: "Actual GRN rework"}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e := service.Workspace(ctx, "accounts-finance", f.project, pkg)
	if e != nil || blocked.Record == nil || len(blocked.History) != 4 || len(blocked.Actions) != 0 {
		t.Fatal("GRN rework lost engineer history", e)
	}
	next := old.Record.Confirmation
	next.EngineerName = "Corrected actual engineer"
	if _, e = engineer.Execute(ctx, "engineer-writer", f.project, pkg, purchaseDomain.EngineerConfirmationsCommand{Version: old.Record.Version + 1, Action: "record", Reason: "Actual corrected GRN", Confirmation: &next}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	blocked, e = service.Workspace(ctx, "accounts-finance", f.project, pkg)
	if e != nil || len(blocked.Actions) != 1 || blocked.Actions[0] != "revise" {
		t.Fatal("renewed GRN reused obsolete acceptance", e)
	}
	if _, e = service.Execute(ctx, "accounts-finance", f.project, pkg, d.Command{Version: 4, Action: "revise", Reason: "Review logistics against corrected dispatch"}); e != nil {
		t.Fatal(e)
	}
	q.Version = 5
	current, e = service.Execute(ctx, "accounts-finance", f.project, pkg, q)
	if e != nil || current.Record.Source.EngineerVersion != 8 {
		t.Fatal(e)
	}
	fact(true)
	for _, sql := range []string{"UPDATE finance_accounts_acceptance SET version=version+1 WHERE id=$1", "DELETE FROM finance_accounts_acceptance WHERE id=$1", "DELETE FROM finance_accounts_acceptance_events WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("unaudited dispatch/history mutation accepted")
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='finance_accounts_acceptance' AND entity_id=$1", id).Scan(&count); e != nil || count != 6 || len(current.History) != 6 {
		t.Fatal("atomic dispatch history", e, count, len(current.History))
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM communication_notices n JOIN business_events e ON e.id=n.source_event_id WHERE e.source_module='finance_accounts_acceptance' AND e.entity_id=$1 AND n.recipient_id=$2", id, f.manager).Scan(&count); e != nil || count != 5 {
		t.Fatal("atomic final and correction Director notices", e, count)
	}

}

type accountsSilentNotice struct{}

func (accountsSilentNotice) DirectorDecision(context.Context, string) (inbox.Notice, error) {
	return inbox.Notice{}, nil
}
