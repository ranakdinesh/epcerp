package integration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inboxout "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	financeout "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/adapters/outbound"
	govout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	govapp "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/application"
	gov "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	accessapp "github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	in "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/ports"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestFinanceBudgetApprovalAtomicBaselineNoticeRevisionAndAuthority(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := t.Context()
	v := out.Values{}
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.manager)
	execFixture(t, f.pool, "UPDATE projects SET contract_value=1000 WHERE id=$1", f.project)
	role := v.ID()
	codes := []string{"projects.budgets.read", "projects.budgets.approve"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'budget approver fixture','test',true)", role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.outsider, role)
	ids := projectIdentities{"manager": auth.Identity{ID: f.manager, Active: true, Superuser: true}, "finance": auth.Identity{ID: f.outsider, Active: true, Permissions: codes}, "spoof": auth.Identity{ID: f.outsider, Active: true, Superuser: true}}
	team := accessapp.New(accessout.NewUnitOfWork(f.pool, accessProviders, purchaseJournal), accessout.NewAuthBridge(ids), accessout.Values{})
	if _, e := team.Save(ctx, "manager", f.project, "", access.Input{UserID: f.outsider, BundleID: role, Responsibility: "Finance", Permissions: codes, Active: true, Reason: "Budget decision authority"}); e != nil {
		t.Fatal(e)
	}
	build := func(j func(database.Connection) events.Writer) *app.Service {
		approval := func(c database.Connection) ports.Approvals {
			users := directorUsers(c)
			notices := inboxout.NewWriter(c, govout.NewDirectory(c, users), j(c), inboxout.Values{})
			return out.NewFinanceBridge(financeout.NewBudgetApprover(c, users, accessout.NewGate(c, accessProviders(c)), j(c), notices, financeout.Values{}))
		}
		return app.New(out.NewFinanceUnitOfWork(f.pool, budgetProviders, j, approval), out.NewAuthBridge(ids), v)
	}
	s := build(purchaseJournal)
	data := domain.Data{Total: "1000.00", Currency: "INR", OwnerID: f.manager, Heads: []domain.Head{{ID: v.ID(), Name: "Purchase", Kind: "purchase", Amount: "1000.00", OwnerID: f.manager, Active: true}}, Allocations: []domain.Allocation{}}
	draft, e := s.Execute(ctx, "manager", f.project, domain.Command{Action: "save", Reason: "Budget draft", Data: &data})
	if e != nil {
		t.Fatal(e)
	}
	readBaseline := func(wantRevision int32) {
		t.Helper()
		e := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			reader := out.NewApprovedReader(c, budgetProviders(c).Projects)
			if _, e := reader.Approved(ctx, f.project); e == nil {
				t.Fatal("Outer context read approved evidence")
			}
			if _, e := out.NewApprovedReader(f.pool, budgetProviders(c).Projects).Approved(txctx, f.project); e == nil {
				t.Fatal("Pool read approved evidence outside consumer transaction")
			}
			source, e := reader.Approved(txctx, f.project)
			if e != nil {
				return e
			}
			if wantRevision == 0 {
				if source != nil {
					t.Fatal("Draft exposed approved source")
				}
				return nil
			}
			if source == nil || source.ProjectID != f.project || source.Revision != wantRevision || source.Currency != "INR" || source.DecisionID == "" {
				t.Fatal("Effective approved budget projection", source)
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
	}
	readBaseline(0)
	approve := domain.Command{Action: "approve", Version: draft.Record.Version, Reason: "Delegated Finance budget approval"}
	if result, e := s.Execute(ctx, "finance", f.project, approve); e == nil || result.Record != nil {
		t.Fatal("Missing Director approval escaped rollback", e)
	}
	checkCounts := func(decisions, notices int) {
		t.Helper()
		for _, check := range []struct {
			sql  string
			want int
		}{{"SELECT count(*) FROM finance_budget_decisions", decisions}, {"SELECT count(*) FROM communication_notices", notices}, {"SELECT count(*) FROM business_events WHERE source_module='finance_approvals'", decisions}} {
			var n int
			if e := f.pool.QueryRow(ctx, check.sql).Scan(&n); e != nil || n != check.want {
				t.Fatal("Atomic financial evidence counts", n, check.want, e)
			}
		}
	}
	checkCounts(0, 0)
	governance := govapp.New(govout.NewUnitOfWork(f.pool, directorUsers, purchaseJournal), govout.NewAuthBridge(ids), govout.Values{})
	if _, e := governance.Save(ctx, "manager", gov.Input{UserID: f.actor, Active: true, Reason: "Budget oversight recipient"}); e != nil {
		t.Fatal(e)
	}
	if result, e := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Execute(ctx, "finance", f.project, approve); e == nil || result.Record != nil {
		t.Fatal("Journal failure returned approval", e)
	}
	checkCounts(0, 0)
	for _, module := range []string{"communication_inbox", "project_budgets"} {
		broken := build(func(c database.Connection) events.Writer {
			return financeBudgetRejectModule{purchaseJournal(c), module}
		})
		if result, e := broken.Execute(ctx, "finance", f.project, approve); e == nil || result.Record != nil {
			t.Fatal("Late journal failure exposed approval", module, e)
		}
		checkCounts(0, 0)
	}
	// A forged recovery claim does not bypass the live scope or actual Auth truth.
	if _, e := s.Execute(ctx, "spoof", f.otherProject, approve); e == nil {
		t.Fatal("Spoofed recovery crossed project scope")
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id IN(SELECT id FROM auth.permissions WHERE code='projects.budgets.approve')", role)
	if _, e := s.Execute(ctx, "finance", f.project, approve); !errors.Is(e, domain.Forbidden) {
		t.Fatal("Live approval revocation ignored", e)
	}
	checkCounts(0, 0)
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.budgets.approve'", role)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Execute(ctx, "finance", f.project, approve); results <- e }()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else if !errors.Is(e, domain.Stale) {
			t.Fatal("Concurrent approval", e)
		}
	}
	if success != 1 {
		t.Fatal("Concurrent approval count", success)
	}
	checkCounts(1, 1)
	readBaseline(1)
	approved, e := s.Workspace(ctx, "finance", f.project)
	if e != nil || approved.Record == nil || approved.Record.State != "approved" || approved.Record.Approved == nil || approved.Record.Version != 2 || approved.Record.Approved.ApprovedBy != f.outsider || approved.ApprovedReport == nil {
		t.Fatal("Native approved baseline", e)
	}
	baseline, e := json.Marshal(approved.Record.Approved)
	if e != nil {
		t.Fatal(e)
	}
	var decision, recipient string
	if e = f.pool.QueryRow(ctx, "SELECT decision_id,recipient_id FROM communication_notices").Scan(&decision, &recipient); e != nil || decision != approved.Record.Approved.DecisionID || recipient != f.actor {
		t.Fatal("Mandatory Director notice", e)
	}
	if _, e = s.Execute(ctx, "manager", f.project, domain.Command{Action: "save", Version: 2, Reason: "Overwrite approved", Data: &data}); e == nil {
		t.Fatal("Approved baseline directly edited")
	}
	revised, e := s.Execute(ctx, "manager", f.project, domain.Command{Action: "revise", Version: 2, Reason: "Prepare budget revision"})
	if e != nil || revised.Record.Revision != 2 || revised.Record.State != "draft" {
		t.Fatal("Budget revision", e)
	}
	preserved, _ := json.Marshal(revised.Record.Approved)
	if string(preserved) != string(baseline) {
		t.Fatal("Revision discarded approved baseline")
	}
	readBaseline(1)
	// HTTP uses saved version/reason only; no forged source prices/data accepted.
	router := chi.NewRouter()
	in.Register(router, s)
	req := httptest.NewRequest("POST", "/api/v1/projects/"+f.project+"/budget", strings.NewReader(`{"action":"approve","version":3,"reason":"Approve revised baseline"}`))
	req.Header.Set("Authorization", "Bearer finance")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatal("Native approval HTTP", response.Code, response.Body.String())
	}
	checkCounts(2, 2)
	readBaseline(2)
	for _, sql := range []string{"UPDATE finance_budget_decisions SET reason='rewrite'", "DELETE FROM finance_budget_decisions", "TRUNCATE finance_budget_decisions", "UPDATE project_budgets SET approved_snapshot=jsonb_set(approved_snapshot,'{data,total}','\"1.00\"'),version=version+1"} {
		if _, e = f.pool.Exec(ctx, sql); e == nil {
			t.Fatal("Finance evidence mutable", sql)
		}
	}
	migration, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer migration.Close()
	if e = migration.Steps(-(schema.LatestVersion - 40)); e == nil {
		t.Fatal("Populated Finance migration rolled back")
	}
	checkCounts(2, 2)
}

// Reject only after Finance has written its decision/event, proving later writes
// and the consuming budget remain in the same transaction.
type financeBudgetRejectModule struct {
	next   events.Writer
	module string
}

func (j financeBudgetRejectModule) Record(ctx context.Context, e events.Event, targets []events.Target) error {
	if e.Module == j.module {
		return errors.New("fixture journal failure")
	}
	return j.next.Record(ctx, e, targets)
}
