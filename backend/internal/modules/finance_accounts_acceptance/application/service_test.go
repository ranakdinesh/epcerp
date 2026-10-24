package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/ports"
	"testing"
)

const project = "11111111-1111-4111-8111-111111111111"
const pkg = "22222222-2222-4222-8222-222222222222"
const actor = "33333333-3333-4333-8333-333333333333"
const recordID = "44444444-4444-4444-8444-444444444444"

type fixture struct {
	record                  *domain.Record
	history                 []domain.Event
	caps                    domain.Access
	source                  domain.Source
	commitFails, eventFails bool
	sourceCalls             int
	notices                 int
	noticeFails             bool
}

func (f *fixture) Within(ctx context.Context, fn func(context.Context, ports.Repository, ports.Context) error) error {
	var old *domain.Record
	if f.record != nil {
		v := domain.Copy(*f.record)
		old = &v
	}
	hist := append([]domain.Event{}, f.history...)
	notices := f.notices
	e := fn(ctx, f, f)
	if e == nil && f.commitFails {
		e = errors.New("late commit failure")
	}
	if e != nil {
		f.record = old
		f.history = hist
		f.notices = notices
	}
	return e
}
func (f *fixture) Get(context.Context, string, string) (*domain.Record, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.Copy(*f.record)
	return &v, nil
}
func (f *fixture) Save(_ context.Context, v domain.Record) (domain.Record, error) {
	c := domain.Copy(v)
	f.record = &c
	return domain.Copy(c), nil
}
func (f *fixture) Event(_ context.Context, v domain.Event) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *fixture) History(context.Context, string) ([]domain.Event, error) {
	return append([]domain.Event{}, f.history...), nil
}
func (f *fixture) Access(context.Context, domain.Actor, string, string) (domain.Access, error) {
	return f.caps, nil
}
func (f *fixture) Source(context.Context, string, string) (domain.Source, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *fixture) Resolve(context.Context, string) (domain.Actor, error) {
	return domain.Actor{ID: actor, Active: true}, nil
}
func (f *fixture) ID() string            { return recordID }
func (f *fixture) Now() domain.Timestamp { return domain.Timestamp{}.AddDate(2025, 9, 10) }
func setup() *fixture {
	return &fixture{caps: domain.Access{Read: true, Write: true, Scoped: true, Work: true}, source: domain.Source{Available: true, Blockers: []string{}, SourceFacts: domain.SourceFacts{EngineerID: recordID, EngineerVersion: 6, EngineerRevision: 3, ConfirmationDate: "2026-10-11", EngineerName: "Actual engineer", Currency: "INR", GRNID: recordID, GRNVersion: 6, GRNRevision: 3, GRNNumber: "GRN-1", GRNDate: "2026-10-10", Received: "1.000001", Accepted: "1", Rejected: "0.000001", ReceiptID: recordID, ReceiptVersion: 6, ReceiptRevision: 3, ArrivalDate: "2026-10-10", ReceivingOfficer: "Actual officer", PackagesCondition: "quarantine", DispatchID: recordID, DispatchVersion: 6, DispatchRevision: 3, DispatchDate: "2026-10-10", LRNumber: "LR-1", Transporter: "Carrier", ClearanceID: recordID, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: project, PackageID: pkg, QualityID: recordID, QualityVersion: 23, QualityRevision: 2, ReportID: recordID, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: recordID, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: recordID, InspectionVersion: 8}}}
}
func command() domain.Command {
	return domain.Command{Action: "record", Reason: "Record actual pending client acceptance", Acceptance: &domain.AcceptanceInput{Status: "pending"}}
}
func TestSiteAcceptanceCommitAndJournalRetainPriorTruth(t *testing.T) {
	f := setup()
	s := New(f, f, f)
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", project, pkg, command())
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", project, pkg, command())
	if e != nil || w.Record == nil || len(w.History) != 1 {
		t.Fatal(e)
	}
	f.commitFails = true
	q := command()
	q.Version = 1
	q.Acceptance = &domain.AcceptanceInput{Status: "accepted", OfficerName: "Actual Accounts officer", Date: "2026-10-11"}
	if w, e = s.Execute(t.Context(), "actor", project, pkg, q); e == nil || w.Record != nil || f.record.Acceptance.Status != "pending" || len(f.history) != 1 {
		t.Fatal("failed acceptance escaped", e)
	}
}
func TestSiteAcceptanceAuthorizationVersionAndTaskScope(t *testing.T) {
	f := setup()
	s := New(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", project, pkg); e != domain.Forbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized evidence", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", project, pkg, command()); e != domain.Forbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := command()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", project, pkg, q); e != domain.Stale || f.sourceCalls != 0 {
		t.Fatal("stale read evidence", e)
	}
	if _, e := s.Execute(t.Context(), "actor", project, pkg, command()); e != nil {
		t.Fatal(e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "actor", project, pkg)
	if e != nil || !w.TaskOnly || w.Record == nil || len(w.History) != 0 {
		t.Fatal("task history leaked", e)
	}
	f.source.QualityVersion++
	w, e = s.Workspace(t.Context(), "actor", project, pkg)
	if e != nil || len(w.Actions) != 1 || w.Actions[0] != "revise" {
		t.Fatal("source correction actions", e)
	}
	f.source.Available = false
	w, e = s.Workspace(t.Context(), "actor", project, pkg)
	if e != nil || w.Record == nil || len(w.Actions) != 0 {
		t.Fatal("held saved history", e)
	}
}

func (f *fixture) Director(context.Context, string) error {
	if f.noticeFails {
		return errors.New("Director delivery failed")
	}
	f.notices++
	return nil
}
func TestDirectorNoticeAtomicForAcceptanceAndCorrection(t *testing.T) {
	f := setup()
	s := New(f, f, f)
	q := command()
	q.Acceptance = &domain.AcceptanceInput{Status: "accepted", OfficerName: "Officer", Date: "2026-10-11", BookedAmount: "0"}
	f.noticeFails = true
	if w, e := s.Execute(t.Context(), "actor", project, pkg, q); e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 || f.notices != 0 {
		t.Fatal("notice failure escaped", e)
	}
	f.noticeFails = false
	w, e := s.Execute(t.Context(), "actor", project, pkg, q)
	if e != nil || f.notices != 1 {
		t.Fatal(e)
	}
	f.noticeFails = true
	if w, e = s.Execute(t.Context(), "actor", project, pkg, domain.Command{Version: w.Record.Version, Action: "revise", Reason: "Correction"}); e == nil || w.Record != nil || f.record.Acceptance.Status != "accepted" || f.notices != 1 {
		t.Fatal("correction notice rollback", e)
	}
	f.noticeFails = false
	if _, e = s.Execute(t.Context(), "actor", project, pkg, domain.Command{Version: 1, Action: "revise", Reason: "Correction"}); e != nil || f.notices != 2 {
		t.Fatal(e)
	}
}
