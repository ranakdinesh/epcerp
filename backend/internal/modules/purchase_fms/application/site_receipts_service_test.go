package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const siteReceiptsProject = "11111111-1111-4111-8111-111111111111"
const siteReceiptsPkg = "22222222-2222-4222-8222-222222222222"
const siteReceiptsActor = "33333333-3333-4333-8333-333333333333"
const siteReceiptsRecordID = "44444444-4444-4444-8444-444444444444"

type siteReceiptsFixture struct {
	record                  *domain.SiteReceiptsRecord
	history                 []domain.SiteReceiptsEvent
	caps                    domain.SiteReceiptsAccess
	source                  domain.SiteReceiptsSource
	commitFails, eventFails bool
	sourceCalls             int
}

func (f *siteReceiptsFixture) Within(ctx context.Context, fn func(context.Context, ports.SiteReceiptsRepository, ports.SiteReceiptsContext) error) error {
	var old *domain.SiteReceiptsRecord
	if f.record != nil {
		v := domain.SiteReceiptsCopy(*f.record)
		old = &v
	}
	hist := append([]domain.SiteReceiptsEvent{}, f.history...)
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
func (f *siteReceiptsFixture) Get(context.Context, string, string) (*domain.SiteReceiptsRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.SiteReceiptsCopy(*f.record)
	return &v, nil
}
func (f *siteReceiptsFixture) Save(_ context.Context, v domain.SiteReceiptsRecord) (domain.SiteReceiptsRecord, error) {
	c := domain.SiteReceiptsCopy(v)
	f.record = &c
	return domain.SiteReceiptsCopy(c), nil
}
func (f *siteReceiptsFixture) Event(_ context.Context, v domain.SiteReceiptsEvent) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *siteReceiptsFixture) History(context.Context, string) ([]domain.SiteReceiptsEvent, error) {
	return append([]domain.SiteReceiptsEvent{}, f.history...), nil
}
func (f *siteReceiptsFixture) Access(context.Context, domain.Principal, string, string) (domain.SiteReceiptsAccess, error) {
	return f.caps, nil
}
func (f *siteReceiptsFixture) Source(context.Context, string, string) (domain.SiteReceiptsSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *siteReceiptsFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: siteReceiptsActor, Active: true}, nil
}
func (f *siteReceiptsFixture) ID() string { return siteReceiptsRecordID }
func (f *siteReceiptsFixture) Now() domain.SiteReceiptsTimestamp {
	return domain.SiteReceiptsTimestamp{}.AddDate(2025, 9, 10)
}
func siteReceiptsSetup() *siteReceiptsFixture {
	return &siteReceiptsFixture{caps: domain.SiteReceiptsAccess{Read: true, Write: true, Scoped: true, Work: true}, source: domain.SiteReceiptsSource{Available: true, Blockers: []string{}, SiteReceiptsSourceFacts: domain.SiteReceiptsSourceFacts{DispatchID: siteReceiptsRecordID, DispatchVersion: 6, DispatchRevision: 3, DispatchDate: "2026-10-10", LRNumber: "LR-1", Transporter: "Carrier", ClearanceID: siteReceiptsRecordID, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: siteReceiptsProject, PackageID: siteReceiptsPkg, QualityID: siteReceiptsRecordID, QualityVersion: 23, QualityRevision: 2, ReportID: siteReceiptsRecordID, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: siteReceiptsRecordID, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: siteReceiptsRecordID, InspectionVersion: 8}}}
}
func siteReceiptsCommand() domain.SiteReceiptsCommand {
	return domain.SiteReceiptsCommand{Action: "record", Reason: "Record actual pending client receipt", Receipt: &domain.SiteReceiptsReceiptInput{Status: "pending"}}
}
func TestSiteReceiptsSiteReceiptCommitAndJournalRetainPriorTruth(t *testing.T) {
	f := siteReceiptsSetup()
	s := NewSiteReceipts(f, f, f)
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg, siteReceiptsCommand())
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg, siteReceiptsCommand())
	if e != nil || w.Record == nil || len(w.History) != 1 {
		t.Fatal(e)
	}
	f.commitFails = true
	q := siteReceiptsCommand()
	q.Version = 1
	q.Receipt = &domain.SiteReceiptsReceiptInput{Status: "delivered_at_site", ReceivingOfficer: "Actual receiving officer", ArrivalDate: "2026-10-11"}
	if w, e = s.Execute(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg, q); e == nil || w.Record != nil || f.record.Receipt.Status != "pending" || len(f.history) != 1 {
		t.Fatal("failed receipt escaped", e)
	}
}
func TestSiteReceiptsSiteReceiptAuthorizationVersionAndTaskScope(t *testing.T) {
	f := siteReceiptsSetup()
	s := NewSiteReceipts(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg); e != domain.SiteReceiptsForbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized evidence", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg, siteReceiptsCommand()); e != domain.SiteReceiptsForbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := siteReceiptsCommand()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg, q); e != domain.SiteReceiptsStale || f.sourceCalls != 0 {
		t.Fatal("stale read evidence", e)
	}
	if _, e := s.Execute(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg, siteReceiptsCommand()); e != nil {
		t.Fatal(e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg)
	if e != nil || !w.TaskOnly || w.Record == nil || len(w.History) != 0 {
		t.Fatal("task history leaked", e)
	}
	f.source.QualityVersion++
	w, e = s.Workspace(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg)
	if e != nil || len(w.Actions) != 1 || w.Actions[0] != "revise" {
		t.Fatal("source correction actions", e)
	}
	f.source.Available = false
	w, e = s.Workspace(t.Context(), "actor", siteReceiptsProject, siteReceiptsPkg)
	if e != nil || w.Record == nil || len(w.Actions) != 0 {
		t.Fatal("held saved history", e)
	}
}
