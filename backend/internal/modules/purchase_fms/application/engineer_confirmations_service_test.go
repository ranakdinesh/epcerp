package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const engineerConfirmationsProject = "11111111-1111-4111-8111-111111111111"
const engineerConfirmationsPkg = "22222222-2222-4222-8222-222222222222"
const engineerConfirmationsActor = "33333333-3333-4333-8333-333333333333"
const engineerConfirmationsRecordID = "44444444-4444-4444-8444-444444444444"

type engineerConfirmationsFixture struct {
	record                  *domain.EngineerConfirmationsRecord
	history                 []domain.EngineerConfirmationsEvent
	caps                    domain.EngineerConfirmationsAccess
	source                  domain.EngineerConfirmationsSource
	commitFails, eventFails bool
	sourceCalls             int
}

func (f *engineerConfirmationsFixture) Within(ctx context.Context, fn func(context.Context, ports.EngineerConfirmationsRepository, ports.EngineerConfirmationsContext) error) error {
	var old *domain.EngineerConfirmationsRecord
	if f.record != nil {
		v := domain.EngineerConfirmationsCopy(*f.record)
		old = &v
	}
	hist := append([]domain.EngineerConfirmationsEvent{}, f.history...)
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
func (f *engineerConfirmationsFixture) Get(context.Context, string, string) (*domain.EngineerConfirmationsRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.EngineerConfirmationsCopy(*f.record)
	return &v, nil
}
func (f *engineerConfirmationsFixture) Save(_ context.Context, v domain.EngineerConfirmationsRecord) (domain.EngineerConfirmationsRecord, error) {
	c := domain.EngineerConfirmationsCopy(v)
	f.record = &c
	return domain.EngineerConfirmationsCopy(c), nil
}
func (f *engineerConfirmationsFixture) Event(_ context.Context, v domain.EngineerConfirmationsEvent) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *engineerConfirmationsFixture) History(context.Context, string) ([]domain.EngineerConfirmationsEvent, error) {
	return append([]domain.EngineerConfirmationsEvent{}, f.history...), nil
}
func (f *engineerConfirmationsFixture) Access(context.Context, domain.Principal, string, string) (domain.EngineerConfirmationsAccess, error) {
	return f.caps, nil
}
func (f *engineerConfirmationsFixture) Source(context.Context, string, string) (domain.EngineerConfirmationsSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *engineerConfirmationsFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: engineerConfirmationsActor, Active: true}, nil
}
func (f *engineerConfirmationsFixture) ID() string { return engineerConfirmationsRecordID }
func (f *engineerConfirmationsFixture) Now() domain.EngineerConfirmationsTimestamp {
	return domain.EngineerConfirmationsTimestamp{}.AddDate(2025, 9, 10)
}
func engineerConfirmationsSetup() *engineerConfirmationsFixture {
	return &engineerConfirmationsFixture{caps: domain.EngineerConfirmationsAccess{Read: true, Write: true, Scoped: true, Work: true}, source: domain.EngineerConfirmationsSource{Available: true, Blockers: []string{}, EngineerConfirmationsSourceFacts: domain.EngineerConfirmationsSourceFacts{GRNID: engineerConfirmationsRecordID, GRNVersion: 6, GRNRevision: 3, GRNNumber: "GRN-1", GRNDate: "2026-10-10", Received: "1.000001", Accepted: "1", Rejected: "0.000001", ReceiptID: engineerConfirmationsRecordID, ReceiptVersion: 6, ReceiptRevision: 3, ArrivalDate: "2026-10-10", ReceivingOfficer: "Actual officer", PackagesCondition: "quarantine", DispatchID: engineerConfirmationsRecordID, DispatchVersion: 6, DispatchRevision: 3, DispatchDate: "2026-10-10", LRNumber: "LR-1", Transporter: "Carrier", ClearanceID: engineerConfirmationsRecordID, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: engineerConfirmationsProject, PackageID: engineerConfirmationsPkg, QualityID: engineerConfirmationsRecordID, QualityVersion: 23, QualityRevision: 2, ReportID: engineerConfirmationsRecordID, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: engineerConfirmationsRecordID, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: engineerConfirmationsRecordID, InspectionVersion: 8}}}
}
func engineerConfirmationsCommand() domain.EngineerConfirmationsCommand {
	return domain.EngineerConfirmationsCommand{Action: "record", Reason: "Record actual pending client confirmation", Confirmation: &domain.EngineerConfirmationsConfirmationInput{Status: "pending"}}
}
func TestEngineerConfirmationsSiteConfirmationCommitAndJournalRetainPriorTruth(t *testing.T) {
	f := engineerConfirmationsSetup()
	s := NewEngineerConfirmations(f, f, f)
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg, engineerConfirmationsCommand())
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg, engineerConfirmationsCommand())
	if e != nil || w.Record == nil || len(w.History) != 1 {
		t.Fatal(e)
	}
	f.commitFails = true
	q := engineerConfirmationsCommand()
	q.Version = 1
	q.Confirmation = &domain.EngineerConfirmationsConfirmationInput{Status: "confirmed", EngineerName: "Actual engineer", Date: "2026-10-11"}
	if w, e = s.Execute(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg, q); e == nil || w.Record != nil || f.record.Confirmation.Status != "pending" || len(f.history) != 1 {
		t.Fatal("failed confirmation escaped", e)
	}
}
func TestEngineerConfirmationsSiteConfirmationAuthorizationVersionAndTaskScope(t *testing.T) {
	f := engineerConfirmationsSetup()
	s := NewEngineerConfirmations(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg); e != domain.EngineerConfirmationsForbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized evidence", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg, engineerConfirmationsCommand()); e != domain.EngineerConfirmationsForbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := engineerConfirmationsCommand()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg, q); e != domain.EngineerConfirmationsStale || f.sourceCalls != 0 {
		t.Fatal("stale read evidence", e)
	}
	if _, e := s.Execute(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg, engineerConfirmationsCommand()); e != nil {
		t.Fatal(e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg)
	if e != nil || !w.TaskOnly || w.Record == nil || len(w.History) != 0 {
		t.Fatal("task history leaked", e)
	}
	f.source.QualityVersion++
	w, e = s.Workspace(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg)
	if e != nil || len(w.Actions) != 1 || w.Actions[0] != "revise" {
		t.Fatal("source correction actions", e)
	}
	f.source.Available = false
	w, e = s.Workspace(t.Context(), "actor", engineerConfirmationsProject, engineerConfirmationsPkg)
	if e != nil || w.Record == nil || len(w.Actions) != 0 {
		t.Fatal("held saved history", e)
	}
}
