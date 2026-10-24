package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const inspectionReportsProject = "11111111-1111-4111-8111-111111111111"
const inspectionReportsPkg = "22222222-2222-4222-8222-222222222222"
const inspectionReportsActor = "33333333-3333-4333-8333-333333333333"
const inspectionReportsRecordID = "44444444-4444-4444-8444-444444444444"

type inspectionReportsFixture struct {
	record                  *domain.InspectionReportsRecord
	history                 []domain.InspectionReportsEvent
	caps                    domain.InspectionReportsAccess
	source                  domain.InspectionReportsSource
	commitFails, eventFails bool
	sourceCalls             int
}

func (f *inspectionReportsFixture) Within(ctx context.Context, fn func(context.Context, ports.InspectionReportsRepository, ports.InspectionReportsContext) error) error {
	var old *domain.InspectionReportsRecord
	if f.record != nil {
		v := domain.InspectionReportsCopy(*f.record)
		old = &v
	}
	hist := append([]domain.InspectionReportsEvent{}, f.history...)
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
func (f *inspectionReportsFixture) Get(context.Context, string, string) (*domain.InspectionReportsRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.InspectionReportsCopy(*f.record)
	return &v, nil
}
func (f *inspectionReportsFixture) Save(_ context.Context, v domain.InspectionReportsRecord) (domain.InspectionReportsRecord, error) {
	c := domain.InspectionReportsCopy(v)
	f.record = &c
	return domain.InspectionReportsCopy(c), nil
}
func (f *inspectionReportsFixture) Event(_ context.Context, v domain.InspectionReportsEvent) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *inspectionReportsFixture) History(context.Context, string) ([]domain.InspectionReportsEvent, error) {
	return append([]domain.InspectionReportsEvent{}, f.history...), nil
}
func (f *inspectionReportsFixture) Access(context.Context, domain.Principal, string, string) (domain.InspectionReportsAccess, error) {
	return f.caps, nil
}
func (f *inspectionReportsFixture) Source(context.Context, string, string) (domain.InspectionReportsSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *inspectionReportsFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: inspectionReportsActor, Active: true}, nil
}
func (f *inspectionReportsFixture) ID() string { return inspectionReportsRecordID }
func (f *inspectionReportsFixture) Now() domain.InspectionReportsTimestamp {
	return domain.InspectionReportsTimestamp{}.AddDate(2025, 9, 10)
}
func inspectionReportsSetup() *inspectionReportsFixture {
	return &inspectionReportsFixture{caps: domain.InspectionReportsAccess{Read: true, Write: true, Scoped: true, Work: true}, source: domain.InspectionReportsSource{Available: true, Blockers: []string{}, InspectionReportsSourceFacts: domain.InspectionReportsSourceFacts{ProjectID: inspectionReportsProject, PackageID: inspectionReportsPkg, InspectionID: inspectionReportsRecordID, InspectionVersion: 6, InspectionRevision: 3, ActualDate: "2026-10-10", OrderID: inspectionReportsRecordID, OrderVersion: 2, Number: "Actual PO", SupplierName: "Actual supplier", Description: "Inspected material", Quantity: "10.000001", UOM: "metre", QAPReference: "Actual QAP"}}}
}
func inspectionReportsCommand() domain.InspectionReportsCommand {
	return domain.InspectionReportsCommand{Action: "record", Reason: "Prepare actual report", Report: &domain.InspectionReportsReportInput{Status: "pending"}}
}
func TestInspectionReportsReportCommitAndJournalFailuresRetainPriorTruth(t *testing.T) {
	f := inspectionReportsSetup()
	s := NewInspectionReports(f, f, f)
	q := inspectionReportsCommand()
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg, q)
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg, q)
	if e != nil || w.Record == nil || w.Record.Version != 1 || len(w.History) != 1 {
		t.Fatal(e)
	}
	q.Version = 1
	q.Report.Status = "issued"
	q.Report.IRNumber = "Actual IR"
	q.Report.IRNNumber = "Actual IRN"
	q.Report.IssueDate = "2026-10-11"
	q.Report.Quantity = "5"
	f.commitFails = true
	if w, e = s.Execute(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg, q); e == nil || w.Record != nil || f.record.Report.Status != "pending" || len(f.history) != 1 {
		t.Fatal("failed correction escaped", e)
	}
}
func TestInspectionReportsReportAuthorizationVersionAndScopePrecedeEvidence(t *testing.T) {
	f := inspectionReportsSetup()
	s := NewInspectionReports(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg); e != domain.InspectionReportsForbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized source read", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg, inspectionReportsCommand()); e != domain.InspectionReportsForbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := inspectionReportsCommand()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg, q); e != domain.InspectionReportsStale || f.sourceCalls != 0 {
		t.Fatal("stale version read source", e)
	}
	f.source.ProjectID = inspectionReportsRecordID
	if w, e := s.Workspace(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg); e == nil || w.Record != nil {
		t.Fatal("foreign source leaked")
	}
}
func TestInspectionReportsTaskOnlyReportReadsRetainRecordWithoutProjectHistory(t *testing.T) {
	f := inspectionReportsSetup()
	s := NewInspectionReports(f, f, f)
	if _, e := s.Execute(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg, inspectionReportsCommand()); e != nil {
		t.Fatal(e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "actor", inspectionReportsProject, inspectionReportsPkg)
	if e != nil || !w.TaskOnly || w.Record == nil || len(w.History) != 0 {
		t.Fatal("task-only history exposed", e)
	}
}
