package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const clientClearanceProject = "11111111-1111-4111-8111-111111111111"
const clientClearancePkg = "22222222-2222-4222-8222-222222222222"
const clientClearanceActor = "33333333-3333-4333-8333-333333333333"
const clientClearanceRecordID = "44444444-4444-4444-8444-444444444444"

type clientClearanceFixture struct {
	record                  *domain.ClientClearanceRecord
	history                 []domain.ClientClearanceEvent
	caps                    domain.ClientClearanceAccess
	source                  domain.ClientClearanceSource
	commitFails, eventFails bool
	sourceCalls             int
}

func (f *clientClearanceFixture) Within(ctx context.Context, fn func(context.Context, ports.ClientClearanceRepository, ports.ClientClearanceContext) error) error {
	var old *domain.ClientClearanceRecord
	if f.record != nil {
		v := domain.ClientClearanceCopy(*f.record)
		old = &v
	}
	hist := append([]domain.ClientClearanceEvent{}, f.history...)
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
func (f *clientClearanceFixture) Get(context.Context, string, string) (*domain.ClientClearanceRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.ClientClearanceCopy(*f.record)
	return &v, nil
}
func (f *clientClearanceFixture) Save(_ context.Context, v domain.ClientClearanceRecord) (domain.ClientClearanceRecord, error) {
	c := domain.ClientClearanceCopy(v)
	f.record = &c
	return domain.ClientClearanceCopy(c), nil
}
func (f *clientClearanceFixture) Event(_ context.Context, v domain.ClientClearanceEvent) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *clientClearanceFixture) History(context.Context, string) ([]domain.ClientClearanceEvent, error) {
	return append([]domain.ClientClearanceEvent{}, f.history...), nil
}
func (f *clientClearanceFixture) Access(context.Context, domain.Principal, string, string) (domain.ClientClearanceAccess, error) {
	return f.caps, nil
}
func (f *clientClearanceFixture) Source(context.Context, string, string) (domain.ClientClearanceSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *clientClearanceFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: clientClearanceActor, Active: true}, nil
}
func (f *clientClearanceFixture) ID() string { return clientClearanceRecordID }
func (f *clientClearanceFixture) Now() domain.ClientClearanceTimestamp {
	return domain.ClientClearanceTimestamp{}.AddDate(2025, 9, 10)
}
func clientClearanceSetup() *clientClearanceFixture {
	return &clientClearanceFixture{caps: domain.ClientClearanceAccess{Read: true, Write: true, Scoped: true, Work: true}, source: domain.ClientClearanceSource{Available: true, Blockers: []string{}, ClientClearanceSourceFacts: domain.ClientClearanceSourceFacts{ProjectID: clientClearanceProject, PackageID: clientClearancePkg, QualityID: clientClearanceRecordID, QualityVersion: 23, QualityRevision: 2, ReportID: clientClearanceRecordID, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: clientClearanceRecordID, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: clientClearanceRecordID, InspectionVersion: 8}}}
}
func clientClearanceCommand() domain.ClientClearanceCommand {
	return domain.ClientClearanceCommand{Action: "record", Reason: "Record actual pending client clearance", Clearance: &domain.ClientClearanceClearanceInput{Status: "pending"}}
}
func TestClientClearanceClientClearanceCommitAndJournalRetainPriorTruth(t *testing.T) {
	f := clientClearanceSetup()
	s := NewClientClearance(f, f, f)
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", clientClearanceProject, clientClearancePkg, clientClearanceCommand())
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", clientClearanceProject, clientClearancePkg, clientClearanceCommand())
	if e != nil || w.Record == nil || len(w.History) != 1 {
		t.Fatal(e)
	}
	f.commitFails = true
	q := clientClearanceCommand()
	q.Version = 1
	q.Clearance = &domain.ClientClearanceClearanceInput{Status: "cleared", Reference: "Actual MDCC", Date: "2026-10-11"}
	if w, e = s.Execute(t.Context(), "actor", clientClearanceProject, clientClearancePkg, q); e == nil || w.Record != nil || f.record.Clearance.Status != "pending" || len(f.history) != 1 {
		t.Fatal("failed clearance escaped", e)
	}
}
func TestClientClearanceClientClearanceAuthorizationVersionAndTaskScope(t *testing.T) {
	f := clientClearanceSetup()
	s := NewClientClearance(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", clientClearanceProject, clientClearancePkg); e != domain.ClientClearanceForbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized evidence", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", clientClearanceProject, clientClearancePkg, clientClearanceCommand()); e != domain.ClientClearanceForbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := clientClearanceCommand()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", clientClearanceProject, clientClearancePkg, q); e != domain.ClientClearanceStale || f.sourceCalls != 0 {
		t.Fatal("stale read evidence", e)
	}
	if _, e := s.Execute(t.Context(), "actor", clientClearanceProject, clientClearancePkg, clientClearanceCommand()); e != nil {
		t.Fatal(e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "actor", clientClearanceProject, clientClearancePkg)
	if e != nil || !w.TaskOnly || w.Record == nil || len(w.History) != 0 {
		t.Fatal("task history leaked", e)
	}
	f.source.QualityVersion++
	w, e = s.Workspace(t.Context(), "actor", clientClearanceProject, clientClearancePkg)
	if e != nil || len(w.Actions) != 1 || w.Actions[0] != "revise" {
		t.Fatal("source correction actions", e)
	}
	f.source.Available = false
	w, e = s.Workspace(t.Context(), "actor", clientClearanceProject, clientClearancePkg)
	if e != nil || w.Record == nil || len(w.Actions) != 0 {
		t.Fatal("held saved history", e)
	}
}
