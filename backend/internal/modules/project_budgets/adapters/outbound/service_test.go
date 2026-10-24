package outbound_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/ports"
	"testing"
	"time"
)

func identifier(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
func str(s string) *string    { return &s }

var clock = time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)

type values struct{ next int }

func (v *values) ID() string          { v.next++; return identifier(100 + v.next) }
func (*values) Now() domain.Timestamp { return clock }

type auth struct{ actor domain.Actor }

func (a *auth) Resolve(context.Context, string) (domain.Actor, error) { return a.actor, nil }

type state struct {
	record             *domain.Budget
	events             []domain.Event
	decisions, notices int
}
type transaction struct {
	state
	scope                                     domain.Scope
	write, finance                            bool
	scopeErr, financeErr, eventErr, commitErr error
}

func (x *transaction) Within(ctx context.Context, fn func(context.Context, ports.Repository, ports.Context, ports.Approvals) error) error {
	copy := *x
	copy.events = append([]domain.Event{}, x.events...)
	if x.record != nil {
		b := domain.Copy(*x.record)
		copy.record = &b
	}
	var approval ports.Approvals
	if x.finance {
		approval = &copy
	}
	if e := fn(ctx, &copy, &copy, approval); e != nil {
		return e
	}
	if x.commitErr != nil {
		return x.commitErr
	}
	x.state = copy.state
	return nil
}
func (x *transaction) Get(context.Context, string) (*domain.Budget, error) { return x.record, nil }
func (x *transaction) Save(_ context.Context, b domain.Budget, actor string) (domain.Budget, error) {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = clock
		b.CreatedBy = str(actor)
	}
	b.UpdatedAt = clock
	b.UpdatedBy = str(actor)
	x.record = &b
	return b, nil
}
func (x *transaction) Event(_ context.Context, e domain.Event) error {
	if x.eventErr != nil {
		return x.eventErr
	}
	x.events = append(x.events, e)
	return nil
}
func (x *transaction) History(context.Context, string) ([]domain.Event, error) { return x.events, nil }
func (x *transaction) Scope(context.Context, domain.Actor, string, string, bool) (bool, error) {
	return x.write, x.scopeErr
}
func (x *transaction) Source(context.Context, string) (domain.Scope, error) { return x.scope, nil }
func (x *transaction) Capabilities(context.Context, domain.Actor, string) (domain.Capabilities, error) {
	return domain.Capabilities{CanWrite: x.write, CanApprove: x.write}, nil
}
func (x *transaction) DecideBudget(_ context.Context, _ domain.Actor, b domain.Budget, _ string) (string, error) {
	if x.financeErr != nil {
		return "", x.financeErr
	}
	if b.State != "draft" {
		return "", errors.New("decision must evaluate a draft")
	}
	x.decisions++
	x.notices++
	return identifier(99), nil
}
func fixture() (*application.Service, *transaction, *auth, domain.Data) {
	owner, project := identifier(1), identifier(2)
	a := &auth{domain.Actor{ID: owner, Active: true, Permissions: []string{"projects.budgets.read", "projects.budgets.write", "projects.budgets.approve"}}}
	x := &transaction{write: true, finance: true, scope: domain.Scope{Project: domain.Project{ID: project, Currency: "INR", ContractValue: str("1000"), Available: true}, Owners: []domain.Owner{{ID: owner}}, Items: []domain.Item{{BOID: identifier(6), LineID: identifier(5), BOVersion: 2, Quantity: "10"}}}}
	d := domain.Data{Total: "1000", Currency: "INR", OwnerID: owner, PricingMode: "unit_rates", Heads: []domain.Head{{ID: identifier(3), Name: "Purchase", Kind: "purchase", Amount: "1000", OwnerID: owner, Active: true}}, Allocations: []domain.Allocation{{ID: identifier(7), BOID: identifier(6), LineID: identifier(5), HeadID: identifier(3), Amount: "100", OwnerID: owner, Active: true, UnitPrice: str("10"), PricingQuantity: str("10")}}}
	return application.New(x, a, &values{}), x, a, d
}
func execute(s *application.Service, c domain.Command) (domain.Workspace, error) {
	return s.Execute(context.Background(), "private-test-token", identifier(2), c)
}
func save(t *testing.T, s *application.Service, d domain.Data) {
	t.Helper()
	if _, e := execute(s, domain.Command{Action: "save", Reason: "Initial budget", Data: &d}); e != nil {
		t.Fatal(e)
	}
}

func TestFinalDelegatedApprovalAndRevisionSnapshot(t *testing.T) {
	s, x, _, d := fixture()
	save(t, s, d)
	if x.record.Version != 1 || len(x.events) != 1 || x.record.CreatedBy == nil {
		t.Fatal("missing draft audit")
	}
	w, e := execute(s, domain.Command{Action: "approve", Version: 1, Reason: "Delegated Finance approval"})
	if e != nil {
		t.Fatal(e)
	}
	if w.Record.State != "approved" || w.Record.Version != 2 || w.Record.Approved.DecisionID != identifier(99) || x.decisions != 1 || x.notices != 1 {
		t.Fatal("approval must be final with its mandatory notice")
	}
	if _, e = execute(s, domain.Command{Action: "save", Version: 2, Reason: "Forbidden overwrite", Data: &d}); e == nil {
		t.Fatal("approved draft overwritten")
	}
	w, e = execute(s, domain.Command{Action: "revise", Version: 2, Reason: "New estimate"})
	if e != nil || w.Record.State != "draft" || w.Record.Revision != 2 || w.Record.Approved.Revision != 1 || w.Record.Approved.Data.Total != "1000.00" {
		t.Fatal("lost approved baseline", e)
	}
	d.Notes = "New estimate"
	w, e = execute(s, domain.Command{Action: "save", Version: 3, Reason: "Update revised draft", Data: &d})
	if e != nil || w.Record.Approved.Data.Notes != "" || w.Record.Data.Notes != d.Notes {
		t.Fatal("approval snapshot mutated", e)
	}
}

func TestBudgetApprovalAtomicFailures(t *testing.T) {
	for _, stage := range []string{"finance", "journal", "commit", "missing finance"} {
		t.Run(stage, func(t *testing.T) {
			s, x, _, d := fixture()
			save(t, s, d)
			failure := errors.New("forced transaction failure")
			switch stage {
			case "finance":
				x.financeErr = failure
			case "journal":
				x.eventErr = failure
			case "commit":
				x.commitErr = failure
			case "missing finance":
				x.finance = false
			}
			w, e := execute(s, domain.Command{Action: "approve", Version: 1, Reason: "Approve"})
			if e == nil {
				t.Fatal("failure ignored")
			}
			if w.Record != nil || w.Report != nil || len(w.History) != 0 {
				t.Fatal("failed transaction returned uncommitted financial data")
			}
			if x.record.State != "draft" || x.record.Version != 1 || x.record.Approved != nil || len(x.events) != 1 || x.decisions != 0 || x.notices != 0 {
				t.Fatal("partial financial decision or budget write escaped rollback")
			}
		})
	}
}

func TestBudgetWriteGuards(t *testing.T) {
	for _, stage := range []string{"inactive actor", "missing read", "missing write", "project scope", "scope revoked", "closed", "no contract", "over contract", "stale version", "changed quantity"} {
		t.Run(stage, func(t *testing.T) {
			s, x, a, d := fixture()
			command := domain.Command{Action: "save", Reason: "Budget", Data: &d}
			switch stage {
			case "inactive actor":
				a.actor.Active = false
			case "missing read":
				a.actor.Permissions = []string{"projects.budgets.write"}
			case "missing write":
				a.actor.Permissions = []string{"projects.budgets.read"}
			case "project scope":
				x.write = false
			case "scope revoked":
				x.scopeErr = domain.Forbidden
			case "closed":
				x.scope.Project.Available = false
			case "no contract":
				x.scope.Project.ContractValue = nil
			case "over contract":
				d.Total = "1001"
			case "stale version":
				command.Version = 1
			case "changed quantity":
				x.scope.Items[0].Quantity = "11"
			}
			before := len(x.events)
			if _, e := execute(s, command); e == nil {
				t.Fatal("guard bypassed")
			}
			if len(x.events) != before || x.decisions != 0 || x.notices != 0 {
				t.Fatal("denied write emitted evidence")
			}
			if before == 0 && x.record != nil {
				t.Fatal("denied write persisted")
			}
		})
	}
}

func TestReadOnlyWorkspaceAndVersionRace(t *testing.T) {
	s, x, a, d := fixture()
	save(t, s, d)
	a.actor.Permissions = []string{"projects.budgets.read"}
	x.write = false
	w, e := s.Workspace(context.Background(), "private-test-token", identifier(2))
	if e != nil || w.CanApprove || w.CanWrite || w.Record.Version != 1 {
		t.Fatal("readonly capabilities", e)
	}
	a.actor.Permissions = append(a.actor.Permissions, "projects.budgets.write")
	x.write = true
	if _, e = execute(s, domain.Command{Action: "save", Version: 1, Reason: "First editor", Data: &d}); e != nil {
		t.Fatal(e)
	}
	if _, e = execute(s, domain.Command{Action: "save", Version: 1, Reason: "Stale editor", Data: &d}); e != domain.Stale || x.record.Version != 2 || len(x.events) != 2 {
		t.Fatal("stale editor changed budget", e)
	}
}

func TestProjectBudgetApprovalAllowsUnallocatedItems(t *testing.T) {
	service, state, _, data := fixture()
	data.Allocations = nil
	save(t, service, data)
	result, err := execute(service, domain.Command{Action: "approve", Version: 1, Reason: "Approve project ceiling; item budgets optional"})
	if err != nil || result.Record.State != "approved" || len(result.Record.Approved.Data.Allocations) != 0 || state.decisions != 1 || state.notices != 1 {
		t.Fatal("unallocated items blocked project approval or invented allocations", result, err)
	}
}
