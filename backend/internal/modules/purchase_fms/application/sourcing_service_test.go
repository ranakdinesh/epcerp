package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const sourcingOperator = "11111111-1111-4111-8111-111111111111"
const sourcingApprover = "22222222-2222-4222-8222-222222222222"
const sourcingPackageID = "66666666-6666-4666-8666-666666666666"
const sourcingProjectID = "77777777-7777-4777-8777-777777777777"
const sourcingRecordID = "44444444-4444-4444-8444-444444444444"

type sourcingFixture struct {
	record     *domain.SourcingCase
	events     []domain.SourcingEvent
	caps       domain.SourcingAccess
	fail       bool
	blocked    bool
	badSource  bool
	proofCalls int
}

func (f *sourcingFixture) Within(ctx context.Context, work func(context.Context, ports.SourcingRepository, ports.SourcingContext) error) error {
	before := f.record
	if before != nil {
		v := domain.SourcingCopy(*before)
		before = &v
	}
	events := append([]domain.SourcingEvent{}, f.events...)
	e := work(ctx, f, f)
	if e == nil && f.fail {
		e = errors.New("later commit refused")
	}
	if e != nil {
		f.record = before
		f.events = events
	}
	return e
}
func (f *sourcingFixture) Get(context.Context, string, string) (*domain.SourcingCase, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.SourcingCopy(*f.record)
	return &v, nil
}
func (f *sourcingFixture) Save(_ context.Context, c domain.SourcingCase, _ string) (domain.SourcingCase, error) {
	v := domain.SourcingCopy(c)
	f.record = &v
	return domain.SourcingCopy(v), nil
}
func (f *sourcingFixture) History(context.Context, string, string) ([]domain.SourcingEvent, error) {
	return append([]domain.SourcingEvent{}, f.events...), nil
}
func (f *sourcingFixture) Event(_ context.Context, e domain.SourcingEvent) error {
	f.events = append(f.events, e)
	return nil
}
func (f *sourcingFixture) Access(context.Context, domain.Principal, string, string) (domain.SourcingAccess, error) {
	return f.caps, nil
}
func (f *sourcingFixture) Source(context.Context, string, string) (domain.SourcingSource, error) {
	id := sourcingPackageID
	if f.badSource {
		id = sourcingRecordID
	}
	return domain.SourcingSource{PackageID: id, DemandID: sourcingRecordID, DemandVersion: 1, Quantity: "10", Currency: "INR", Available: !f.blocked, Eligible: true, Blockers: []string{}}, nil
}
func (f *sourcingFixture) Suppliers(context.Context, string, string) ([]domain.SourcingSupplier, error) {
	return []domain.SourcingSupplier{}, nil
}
func (f *sourcingFixture) Approvers(context.Context, string) ([]domain.SourcingMember, error) {
	return []domain.SourcingMember{{ID: sourcingApprover, Label: "Independent approver"}}, nil
}
func (f *sourcingFixture) Evidence(context.Context, string, string) ([]domain.SourcingApprovedEvidence, error) {
	return []domain.SourcingApprovedEvidence{}, nil
}
func (f *sourcingFixture) VerifySelection(context.Context, string, string, domain.SourcingOffer) (*domain.SourcingProof, error) {
	f.proofCalls++
	return nil, nil
}
func (f *sourcingFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: sourcingOperator, Active: true, Superuser: true}, nil
}
func (f *sourcingFixture) ID() string                    { return sourcingRecordID }
func (f *sourcingFixture) Now() domain.SourcingTimestamp { return domain.SourcingTimestamp{} }
func TestSourcingAtomicConfigurationAndLateCommitFailureReturnNoDerivedState(t *testing.T) {
	f := &sourcingFixture{caps: domain.SourcingAccess{Read: true, Write: true, Configure: true, Scoped: true}}
	s := NewSourcing(f, f, f)
	ctx := context.Background()
	w, e := s.Execute(ctx, "actor", sourcingProjectID, sourcingPackageID, domain.SourcingCommand{Action: "configure", Reason: "Set accountable approver", ApproverID: sourcingApprover})
	if e != nil || w.Case == nil || w.Case.Version != 1 || len(w.History) != 1 {
		t.Fatal("native application lifecycle", e)
	}
	f.fail = true
	w, e = s.Execute(ctx, "actor", sourcingProjectID, sourcingPackageID, domain.SourcingCommand{Version: 1, Action: "configure", Reason: "Change with failed commit", ApproverID: sourcingApprover})
	if e == nil || w.Case != nil || len(w.History) != 0 || f.record.Version != 1 || len(f.events) != 1 {
		t.Fatal("failed transaction leaked state or persisted audit", e)
	}
}
func TestSourcingScopeAndSourceIdentityAreOwnedGuards(t *testing.T) {
	f := &sourcingFixture{caps: domain.SourcingAccess{Read: true, Write: true, Configure: true, Scoped: true}, badSource: true}
	s := NewSourcing(f, f, f)
	q := domain.SourcingCommand{Action: "configure", Reason: "Configure", ApproverID: sourcingApprover}
	if w, e := s.Execute(context.Background(), "actor", sourcingProjectID, sourcingPackageID, q); e == nil || w.Case != nil || f.record != nil {
		t.Fatal("wrong source accepted")
	}
	f.badSource = false
	f.caps.Configure = false
	if _, e := s.Execute(context.Background(), "actor", sourcingProjectID, sourcingPackageID, q); e == nil {
		t.Fatal("claimed superadmin bypassed actual configuration scope")
	}
}
func TestSourcingAssignedReadonlyWorkspaceDoesNotPublishDirectoriesOrHistory(t *testing.T) {
	f := &sourcingFixture{caps: domain.SourcingAccess{Read: true, Write: true, Configure: true, Scoped: true}}
	s := NewSourcing(f, f, f)
	if _, e := s.Execute(context.Background(), "actor", sourcingProjectID, sourcingPackageID, domain.SourcingCommand{Action: "configure", Reason: "Create source case", ApproverID: sourcingApprover}); e != nil {
		t.Fatal(e)
	}
	f.caps = domain.SourcingAccess{Read: true}
	w, e := s.Workspace(context.Background(), "actor", sourcingProjectID, sourcingPackageID)
	if e != nil || !w.TaskOnly || len(w.Actions) != 0 || len(w.Approvers) != 0 || len(w.Suppliers) != 0 || len(w.History) != 0 {
		t.Fatal("task scope leak", e)
	}
}

func TestSourcingBlockedSourceRetainsScopedHistoryWithoutMutationActions(t *testing.T) {
	f := &sourcingFixture{caps: domain.SourcingAccess{Read: true, Write: true, Configure: true, Scoped: true}}
	s := NewSourcing(f, f, f)
	if _, err := s.Execute(t.Context(), "actor", sourcingProjectID, sourcingPackageID, domain.SourcingCommand{Action: "configure", Reason: "Create source case", ApproverID: sourcingApprover}); err != nil {
		t.Fatal(err)
	}
	f.blocked = true
	w, err := s.Workspace(t.Context(), "actor", sourcingProjectID, sourcingPackageID)
	if err != nil || w.Case == nil || len(w.History) != 1 || len(w.Actions) != 0 || len(w.Evidence) != 0 || len(w.Suppliers) != 0 {
		t.Fatal("blocked history projection", w, err)
	}
	if _, err := s.Execute(t.Context(), "actor", sourcingProjectID, sourcingPackageID, domain.SourcingCommand{Version: 1, Action: "configure", Reason: "Try blocked change", ApproverID: sourcingApprover}); err == nil || f.record.Version != 1 {
		t.Fatal("blocked source mutation accepted")
	}
	f.caps = domain.SourcingAccess{Read: true}
	w, err = s.Workspace(t.Context(), "actor", sourcingProjectID, sourcingPackageID)
	if err != nil || !w.TaskOnly || len(w.History) != 0 || len(w.Actions) != 0 {
		t.Fatal("blocked task history leak", w, err)
	}
}

func (f *sourcingFixture) LockAllocation(context.Context, string, string, string, string) (domain.SourcingBalances, error) {
	return domain.SourcingBalances{Quantity: "0", Amount: "0"}, nil
}
func (f *sourcingFixture) StoreAllocation(context.Context, domain.SourcingCase, *domain.SourcingProof, domain.SourcingAllocation) error {
	return nil
}
