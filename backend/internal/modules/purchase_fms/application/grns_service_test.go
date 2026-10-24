package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const gRNsProject = "11111111-1111-4111-8111-111111111111"
const gRNsPkg = "22222222-2222-4222-8222-222222222222"
const gRNsActor = "33333333-3333-4333-8333-333333333333"
const gRNsRecordID = "44444444-4444-4444-8444-444444444444"

type gRNsFixture struct {
	record                  *domain.GRNsRecord
	history                 []domain.GRNsEvent
	caps                    domain.GRNsAccess
	source                  domain.GRNsSource
	commitFails, eventFails bool
	sourceCalls             int
}

func (f *gRNsFixture) Within(ctx context.Context, fn func(context.Context, ports.GRNsRepository, ports.GRNsContext) error) error {
	var old *domain.GRNsRecord
	if f.record != nil {
		v := domain.GRNsCopy(*f.record)
		old = &v
	}
	hist := append([]domain.GRNsEvent{}, f.history...)
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
func (f *gRNsFixture) Get(context.Context, string, string) (*domain.GRNsRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.GRNsCopy(*f.record)
	return &v, nil
}
func (f *gRNsFixture) Save(_ context.Context, v domain.GRNsRecord) (domain.GRNsRecord, error) {
	c := domain.GRNsCopy(v)
	f.record = &c
	return domain.GRNsCopy(c), nil
}
func (f *gRNsFixture) Event(_ context.Context, v domain.GRNsEvent) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *gRNsFixture) History(context.Context, string) ([]domain.GRNsEvent, error) {
	return append([]domain.GRNsEvent{}, f.history...), nil
}
func (f *gRNsFixture) Access(context.Context, domain.Principal, string, string) (domain.GRNsAccess, error) {
	return f.caps, nil
}
func (f *gRNsFixture) Source(context.Context, string, string) (domain.GRNsSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *gRNsFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: gRNsActor, Active: true}, nil
}
func (f *gRNsFixture) ID() string                { return gRNsRecordID }
func (f *gRNsFixture) Now() domain.GRNsTimestamp { return domain.GRNsTimestamp{}.AddDate(2025, 9, 10) }
func gRNsSetup() *gRNsFixture {
	return &gRNsFixture{caps: domain.GRNsAccess{Read: true, Write: true, Scoped: true, Work: true}, source: domain.GRNsSource{Available: true, Blockers: []string{}, GRNsSourceFacts: domain.GRNsSourceFacts{ReceiptID: gRNsRecordID, ReceiptVersion: 6, ReceiptRevision: 3, ArrivalDate: "2026-10-10", ReceivingOfficer: "Actual officer", PackagesCondition: "quarantine", DispatchID: gRNsRecordID, DispatchVersion: 6, DispatchRevision: 3, DispatchDate: "2026-10-10", LRNumber: "LR-1", Transporter: "Carrier", ClearanceID: gRNsRecordID, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: gRNsProject, PackageID: gRNsPkg, QualityID: gRNsRecordID, QualityVersion: 23, QualityRevision: 2, ReportID: gRNsRecordID, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: gRNsRecordID, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: gRNsRecordID, InspectionVersion: 8}}}
}
func gRNsCommand() domain.GRNsCommand {
	return domain.GRNsCommand{Action: "record", Reason: "Record actual pending client grn", GRN: &domain.GRNsGRNInput{Status: "pending"}}
}
func TestGRNsSiteGRNCommitAndJournalRetainPriorTruth(t *testing.T) {
	f := gRNsSetup()
	s := NewGRNs(f, f, f)
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", gRNsProject, gRNsPkg, gRNsCommand())
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", gRNsProject, gRNsPkg, gRNsCommand())
	if e != nil || w.Record == nil || len(w.History) != 1 {
		t.Fatal(e)
	}
	f.commitFails = true
	q := gRNsCommand()
	q.Version = 1
	q.GRN = &domain.GRNsGRNInput{Status: "issued", Number: "GRN-actual", Date: "2026-10-11", Received: "1.000001", Accepted: "1", Rejected: "0.000001"}
	if w, e = s.Execute(t.Context(), "actor", gRNsProject, gRNsPkg, q); e == nil || w.Record != nil || f.record.GRN.Status != "pending" || len(f.history) != 1 {
		t.Fatal("failed grn escaped", e)
	}
}
func TestGRNsSiteGRNAuthorizationVersionAndTaskScope(t *testing.T) {
	f := gRNsSetup()
	s := NewGRNs(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", gRNsProject, gRNsPkg); e != domain.GRNsForbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized evidence", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", gRNsProject, gRNsPkg, gRNsCommand()); e != domain.GRNsForbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := gRNsCommand()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", gRNsProject, gRNsPkg, q); e != domain.GRNsStale || f.sourceCalls != 0 {
		t.Fatal("stale read evidence", e)
	}
	if _, e := s.Execute(t.Context(), "actor", gRNsProject, gRNsPkg, gRNsCommand()); e != nil {
		t.Fatal(e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "actor", gRNsProject, gRNsPkg)
	if e != nil || !w.TaskOnly || w.Record == nil || len(w.History) != 0 {
		t.Fatal("task history leaked", e)
	}
	f.source.QualityVersion++
	w, e = s.Workspace(t.Context(), "actor", gRNsProject, gRNsPkg)
	if e != nil || len(w.Actions) != 1 || w.Actions[0] != "revise" {
		t.Fatal("source correction actions", e)
	}
	f.source.Available = false
	w, e = s.Workspace(t.Context(), "actor", gRNsProject, gRNsPkg)
	if e != nil || w.Record == nil || len(w.Actions) != 0 {
		t.Fatal("held saved history", e)
	}
}
