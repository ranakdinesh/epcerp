package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const inspectionsProject = "11111111-1111-4111-8111-111111111111"
const inspectionsPkg = "22222222-2222-4222-8222-222222222222"
const inspectionsActor = "33333333-3333-4333-8333-333333333333"
const inspectionsRecordID = "44444444-4444-4444-8444-444444444444"

type inspectionsFixture struct {
	record                  *domain.InspectionsRecord
	history                 []domain.InspectionsEvent
	caps                    domain.InspectionsAccess
	source                  domain.InspectionsSource
	commitFails, eventFails bool
	sourceCalls             int
}

func (f *inspectionsFixture) Within(ctx context.Context, fn func(context.Context, ports.InspectionsRepository, ports.InspectionsContext) error) error {
	var old *domain.InspectionsRecord
	if f.record != nil {
		v := domain.InspectionsCopy(*f.record)
		old = &v
	}
	hist := append([]domain.InspectionsEvent{}, f.history...)
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
func (f *inspectionsFixture) Get(context.Context, string, string) (*domain.InspectionsRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.InspectionsCopy(*f.record)
	return &v, nil
}
func (f *inspectionsFixture) Save(_ context.Context, v domain.InspectionsRecord) (domain.InspectionsRecord, error) {
	c := domain.InspectionsCopy(v)
	f.record = &c
	return domain.InspectionsCopy(c), nil
}
func (f *inspectionsFixture) Event(_ context.Context, v domain.InspectionsEvent) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *inspectionsFixture) History(context.Context, string) ([]domain.InspectionsEvent, error) {
	return append([]domain.InspectionsEvent{}, f.history...), nil
}
func (f *inspectionsFixture) Access(context.Context, domain.Principal, string, string) (domain.InspectionsAccess, error) {
	return f.caps, nil
}
func (f *inspectionsFixture) Source(context.Context, string, string) (domain.InspectionsSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *inspectionsFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: inspectionsActor, Active: true}, nil
}
func (f *inspectionsFixture) ID() string { return inspectionsRecordID }
func (f *inspectionsFixture) Now() domain.InspectionsTimestamp {
	return domain.InspectionsTimestamp{}.AddDate(2025, 9, 10)
}
func inspectionsSetup() *inspectionsFixture {
	return &inspectionsFixture{caps: domain.InspectionsAccess{Read: true, Write: true, Scoped: true, Work: true}, source: domain.InspectionsSource{Available: true, Blockers: []string{}, InspectionsSourceFacts: domain.InspectionsSourceFacts{ProjectID: inspectionsProject, PackageID: inspectionsPkg, OrderID: inspectionsRecordID, OrderVersion: 2, Number: "PO/actual", Date: "2026-10-10", SupplierID: inspectionsRecordID, SupplierName: "Actual supplier", Description: "Actual material", Quantity: "10", UOM: "metre", AcceptanceID: inspectionsRecordID, AcceptanceVersion: 1, TechnicalID: inspectionsRecordID, TechnicalVersion: 8, TechnicalRevision: 1, QAPReference: "Actual client QAP", QAPDate: "2026-10-10", QAPFileID: inspectionsRecordID, SignedApprovalFileID: inspectionsRecordID}}}
}
func inspectionsCommand() domain.InspectionsCommand {
	return domain.InspectionsCommand{Action: "record", Reason: "Actual vendor inspection call", Inspection: &domain.InspectionsInspectionInput{Status: "call_received", CallReference: "Vendor call reference", CallDate: "2026-10-11", Points: []domain.InspectionsWitnessInput{}}}
}
func TestInspectionsInspectionCommitAndJournalFailuresRetainPriorTruth(t *testing.T) {
	f := inspectionsSetup()
	s := NewInspections(f, f, f)
	q := inspectionsCommand()
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", inspectionsProject, inspectionsPkg, q)
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", inspectionsProject, inspectionsPkg, q)
	if e != nil || w.Record == nil || w.Record.Version != 1 || len(w.History) != 1 {
		t.Fatal(e)
	}
	q.Version = 1
	q.Inspection.Status = "scheduled"
	q.Inspection.ScheduledDate = "2026-10-14"
	q.Inspection.Factory = "Actual works"
	f.commitFails = true
	if w, e = s.Execute(t.Context(), "actor", inspectionsProject, inspectionsPkg, q); e == nil || w.Record != nil || f.record.Inspection.Status != "call_received" || len(f.history) != 1 {
		t.Fatal("failed correction escaped", e)
	}
}
func TestInspectionsInspectionAuthorizationVersionAndScopePrecedeEvidence(t *testing.T) {
	f := inspectionsSetup()
	s := NewInspections(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", inspectionsProject, inspectionsPkg); e != domain.InspectionsForbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized source read", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", inspectionsProject, inspectionsPkg, inspectionsCommand()); e != domain.InspectionsForbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := inspectionsCommand()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", inspectionsProject, inspectionsPkg, q); e != domain.InspectionsStale || f.sourceCalls != 0 {
		t.Fatal("stale version read source", e)
	}
	f.source.ProjectID = inspectionsRecordID
	if w, e := s.Workspace(t.Context(), "actor", inspectionsProject, inspectionsPkg); e == nil || w.Record != nil {
		t.Fatal("foreign source leaked")
	}
}
func TestInspectionsTaskOnlyInspectionReadsRetainRecordWithoutProjectHistory(t *testing.T) {
	f := inspectionsSetup()
	s := NewInspections(f, f, f)
	if _, e := s.Execute(t.Context(), "actor", inspectionsProject, inspectionsPkg, inspectionsCommand()); e != nil {
		t.Fatal(e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "actor", inspectionsProject, inspectionsPkg)
	if e != nil || !w.TaskOnly || w.Record == nil || len(w.History) != 0 {
		t.Fatal("task-only history exposed", e)
	}
}
