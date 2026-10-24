package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const workflowManager = "11111111-1111-4111-8111-111111111111"
const workflowWorker = "22222222-2222-4222-8222-222222222222"

type workflowFixture struct {
	record *domain.WorkflowPackage
	events []domain.WorkflowEvent
	fail   bool
}

func (f *workflowFixture) Resolve(_ context.Context, token string) (domain.Principal, error) {
	return domain.Principal{ID: token, Active: token != ""}, nil
}
func (f *workflowFixture) Now() domain.WorkflowTimestamp { return domain.WorkflowTimestamp{} }
func (f *workflowFixture) ID() string                    { return workflowManager }
func (f *workflowFixture) Actions(_ context.Context, a domain.Principal, _ string) (domain.WorkflowAccess, error) {
	return domain.WorkflowAccess{Read: a.ID == workflowManager, Write: a.ID == workflowManager, Execute: true, Resolve: true}, nil
}
func (f *workflowFixture) Source(context.Context, string, string, string) (domain.WorkflowSource, error) {
	return domain.WorkflowSource{Available: true, Eligible: true, Description: "Released cable", Blockers: []string{}}, nil
}
func (f *workflowFixture) Candidates(context.Context, string) ([]domain.WorkflowCandidate, error) {
	return []domain.WorkflowCandidate{}, nil
}
func (f *workflowFixture) Members(context.Context, string) ([]domain.WorkflowMember, error) {
	return []domain.WorkflowMember{{ID: workflowWorker, Execute: true}, {ID: workflowManager, Resolve: true}}, nil
}
func (f *workflowFixture) Exists(_ context.Context, p, indent, demand string) (bool, error) {
	return f.record != nil && f.record.ProjectID == p && f.record.IndentID == indent && f.record.DemandID == demand, nil
}
func (f *workflowFixture) Get(context.Context, string, string) (*domain.WorkflowPackage, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.WorkflowCopy(*f.record)
	return &v, nil
}
func (f *workflowFixture) Save(_ context.Context, p domain.WorkflowPackage, _ string) (domain.WorkflowPackage, error) {
	v := domain.WorkflowCopy(p)
	f.record = &v
	return v, nil
}
func (f *workflowFixture) History(context.Context, string, string) ([]domain.WorkflowEvent, error) {
	return append([]domain.WorkflowEvent{}, f.events...), nil
}
func (f *workflowFixture) Event(_ context.Context, e domain.WorkflowEvent) error {
	f.events = append(f.events, e)
	return nil
}
func (f *workflowFixture) Within(ctx context.Context, w func(context.Context, ports.WorkflowRepository, ports.WorkflowContext) error) error {
	before := f.record
	events := append([]domain.WorkflowEvent{}, f.events...)
	e := w(ctx, f, f)
	if e == nil && f.fail {
		e = errors.New("fixture commit failed")
	}
	if e != nil {
		f.record = before
		f.events = events
	}
	return e
}
func TestWorkflowCommitRollbackAndExternalTaskProjection(t *testing.T) {
	f := &workflowFixture{}
	s := NewWorkflow(f, f, f)
	w, e := s.Create(t.Context(), workflowManager, workflowManager, domain.WorkflowCreate{IndentID: workflowManager, DemandID: workflowManager, Reason: "Create from release"})
	if e != nil {
		t.Fatal(e)
	}
	w, e = s.Assign(t.Context(), workflowManager, workflowManager, w.Package.ID, domain.WorkflowAssign{Version: 1, Reason: "Scoped process assignment", Rows: []domain.WorkflowAssignment{{Number: 5, Team: "Procurement", DoerID: workflowWorker, ResolverID: workflowManager, PlannedDate: "2026-10-10"}}})
	if e != nil {
		t.Fatal(e)
	}
	external, e := s.Workspace(t.Context(), workflowWorker, workflowManager, w.Package.ID)
	if e != nil || !external.TaskOnly || len(external.Package.Steps) != 1 || len(external.History) != 0 || len(external.Members) != 0 {
		t.Fatal("task-only leak", e)
	}
	f.fail = true
	external, e = s.Command(t.Context(), workflowWorker, workflowManager, w.Package.ID, 5, domain.WorkflowCommand{Version: 2, Action: "start", Reason: "Prepare task"})
	if e == nil || external.Package.Version != 0 || len(external.Package.Steps) != 0 || f.record.Version != 2 || len(f.events) != 2 {
		t.Fatal("failed commit exposed task result", e)
	}
}
