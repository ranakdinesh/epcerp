package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const dispatchProject = "11111111-1111-4111-8111-111111111111"
const dispatchPkg = "22222222-2222-4222-8222-222222222222"
const dispatchActor = "33333333-3333-4333-8333-333333333333"
const dispatchRecordID = "44444444-4444-4444-8444-444444444444"

type dispatchFixture struct {
	record                  *domain.DispatchRecord
	history                 []domain.DispatchEvent
	caps                    domain.DispatchAccess
	source                  domain.DispatchSource
	commitFails, eventFails bool
	sourceCalls             int
}

func (f *dispatchFixture) Within(ctx context.Context, fn func(context.Context, ports.DispatchRepository, ports.DispatchContext) error) error {
	var old *domain.DispatchRecord
	if f.record != nil {
		v := domain.DispatchCopy(*f.record)
		old = &v
	}
	hist := append([]domain.DispatchEvent{}, f.history...)
	e := fn(ctx, f, f)
	if e == nil && f.commitFails {
		e = errors.New("late commit failure")
	}
	if e != nil {
		f.record = old
		f.history = hist
	}
	return e
}
func (f *dispatchFixture) Get(context.Context, string, string) (*domain.DispatchRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.DispatchCopy(*f.record)
	return &v, nil
}
func (f *dispatchFixture) Save(_ context.Context, v domain.DispatchRecord) (domain.DispatchRecord, error) {
	c := domain.DispatchCopy(v)
	f.record = &c
	return domain.DispatchCopy(c), nil
}
func (f *dispatchFixture) Event(_ context.Context, v domain.DispatchEvent) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *dispatchFixture) History(context.Context, string) ([]domain.DispatchEvent, error) {
	return append([]domain.DispatchEvent{}, f.history...), nil
}
func (f *dispatchFixture) Access(context.Context, domain.Principal, string, string) (domain.DispatchAccess, error) {
	return f.caps, nil
}
func (f *dispatchFixture) Source(context.Context, string, string) (domain.DispatchSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *dispatchFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: dispatchActor, Active: true}, nil
}
func (f *dispatchFixture) ID() string { return dispatchRecordID }
func (f *dispatchFixture) Now() domain.DispatchTimestamp {
	return domain.DispatchTimestamp{}.AddDate(2025, 9, 10)
}
func dispatchSetup() *dispatchFixture {
	return &dispatchFixture{caps: domain.DispatchAccess{Read: true, Write: true, Scoped: true, Work: true}, source: domain.DispatchSource{Available: true, Blockers: []string{}, DispatchSourceFacts: domain.DispatchSourceFacts{ClearanceID: dispatchRecordID, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: dispatchProject, PackageID: dispatchPkg, QualityID: dispatchRecordID, QualityVersion: 23, QualityRevision: 2, ReportID: dispatchRecordID, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: dispatchRecordID, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: dispatchRecordID, InspectionVersion: 8}}}
}
func dispatchCommand() domain.DispatchCommand {
	return domain.DispatchCommand{Action: "record", Reason: "Record actual pending client dispatch", Dispatch: &domain.DispatchInput{Status: "pending"}}
}
func TestDispatchMaterialDispatchCommitAndJournalRetainPriorTruth(t *testing.T) {
	f := dispatchSetup()
	s := NewDispatch(f, f, f)
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", dispatchProject, dispatchPkg, dispatchCommand())
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", dispatchProject, dispatchPkg, dispatchCommand())
	if e != nil || w.Record == nil || len(w.History) != 1 {
		t.Fatal(e)
	}
	f.commitFails = true
	q := dispatchCommand()
	q.Version = 1
	q.Dispatch = &domain.DispatchInput{Status: "dispatched", LRNumber: "Actual LR", Transporter: "Actual transporter", Date: "2026-10-11"}
	if w, e = s.Execute(t.Context(), "actor", dispatchProject, dispatchPkg, q); e == nil || w.Record != nil || f.record.Dispatch.Status != "pending" || len(f.history) != 1 {
		t.Fatal("failed dispatch escaped", e)
	}
}
func TestDispatchMaterialDispatchAuthorizationVersionAndTaskScope(t *testing.T) {
	f := dispatchSetup()
	s := NewDispatch(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", dispatchProject, dispatchPkg); e != domain.DispatchForbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized evidence", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", dispatchProject, dispatchPkg, dispatchCommand()); e != domain.DispatchForbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := dispatchCommand()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", dispatchProject, dispatchPkg, q); e != domain.DispatchStale || f.sourceCalls != 0 {
		t.Fatal("stale read evidence", e)
	}
	if _, e := s.Execute(t.Context(), "actor", dispatchProject, dispatchPkg, dispatchCommand()); e != nil {
		t.Fatal(e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "actor", dispatchProject, dispatchPkg)
	if e != nil || !w.TaskOnly || w.Record == nil || len(w.History) != 0 {
		t.Fatal("task history leaked", e)
	}
	f.source.QualityVersion++
	w, e = s.Workspace(t.Context(), "actor", dispatchProject, dispatchPkg)
	if e != nil || len(w.Actions) != 1 || w.Actions[0] != "revise" {
		t.Fatal("source correction actions", e)
	}
	f.source.Available = false
	w, e = s.Workspace(t.Context(), "actor", dispatchProject, dispatchPkg)
	if e != nil || w.Record == nil || len(w.Actions) != 0 {
		t.Fatal("held saved history", e)
	}
}
