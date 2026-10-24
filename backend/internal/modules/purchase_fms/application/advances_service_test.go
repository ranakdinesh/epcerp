package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const advancesProject = "11111111-1111-4111-8111-111111111111"
const advancesPkg = "22222222-2222-4222-8222-222222222222"
const advancesRequester = "33333333-3333-4333-8333-333333333333"
const advancesReviewer = "44444444-4444-4444-8444-444444444444"
const advancesIdentity = "55555555-5555-4555-8555-555555555555"

type advancesFixture struct {
	record                   *domain.AdvancesRecord
	history                  []domain.AdvancesEvent
	source                   domain.AdvancesSource
	caps                     domain.AdvancesAccess
	commitFails, noticeFails bool
	decisions, sourceCalls   int
}

func (f *advancesFixture) Within(ctx context.Context, fn func(context.Context, ports.AdvancesRepository, ports.AdvancesContext, ports.AdvancesFinance) error) error {
	old := f.record
	if old != nil {
		v := domain.AdvancesCopy(*old)
		old = &v
	}
	history := append([]domain.AdvancesEvent{}, f.history...)
	decisions := f.decisions
	e := fn(ctx, f, f, f)
	if e == nil && f.commitFails {
		e = errors.New("commit failed")
	}
	if e != nil {
		f.record = old
		f.history = history
		f.decisions = decisions
	}
	return e
}
func (f *advancesFixture) Get(context.Context, string, string) (*domain.AdvancesRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.AdvancesCopy(*f.record)
	return &v, nil
}
func (f *advancesFixture) Save(_ context.Context, r domain.AdvancesRecord) (domain.AdvancesRecord, error) {
	v := domain.AdvancesCopy(r)
	f.record = &v
	return domain.AdvancesCopy(v), nil
}
func (f *advancesFixture) Event(_ context.Context, e domain.AdvancesEvent) error {
	f.history = append(f.history, e)
	return nil
}
func (f *advancesFixture) History(context.Context, string) ([]domain.AdvancesEvent, error) {
	return append([]domain.AdvancesEvent{}, f.history...), nil
}
func (f *advancesFixture) Access(context.Context, domain.Principal, string, string) (domain.AdvancesAccess, error) {
	return f.caps, nil
}
func (f *advancesFixture) Source(context.Context, string, string) (domain.AdvancesSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *advancesFixture) Decide(_ context.Context, a domain.Principal, r domain.AdvancesRecord, outcome, reason string) (string, error) {
	if a.ID == r.SubmittedBy {
		return "", domain.AdvancesForbidden
	}
	f.decisions++
	if f.noticeFails {
		return "", errors.New("Director notice failed")
	}
	return advancesIdentity, nil
}
func (f *advancesFixture) Resolve(_ context.Context, token string) (domain.Principal, error) {
	id := advancesRequester
	if token == "finance" {
		id = advancesReviewer
	}
	return domain.Principal{ID: id, Active: true}, nil
}
func (f *advancesFixture) ID() string { return advancesIdentity }
func (f *advancesFixture) Now() domain.AdvancesTimestamp {
	return domain.AdvancesTimestamp{}.AddDate(2025, 9, 10)
}
func advancesSetup() *advancesFixture {
	return &advancesFixture{caps: domain.AdvancesAccess{Read: true, Write: true, Work: true, Review: true, Disburse: true, Scoped: true}, source: domain.AdvancesSource{AdvancesSourceFacts: domain.AdvancesSourceFacts{ProjectID: advancesProject, PackageID: advancesPkg, OrderID: advancesIdentity, OrderVersion: 2, Number: "PO-actual", Date: "2026-10-10", SupplierID: advancesIdentity, SupplierName: "Actual supplier", Currency: "INR", Total: "145.69", PaymentTerms: "Actual agreed payment", AcceptanceID: advancesIdentity, AcceptanceVersion: 1, SupplierVerification: "actual-verified", CalculationBasis: "po_total"}, Available: true}}
}
func advancesPrepareAndSubmit(t *testing.T, f *advancesFixture, s *AdvancesService) {
	t.Helper()
	d := domain.AdvancesDetails{Kind: "payment", RequestNo: "ADV-actual", RequestDate: "2026-10-11", Percentage: "30", InvoiceReference: "PI-actual", InvoiceDate: "2026-10-09", BankReference: "Actual instruction reference"}
	if _, e := s.Execute(t.Context(), "requester", advancesProject, advancesPkg, domain.AdvancesCommand{Action: "prepare", Reason: "Prepare", Details: &d}); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Execute(t.Context(), "requester", advancesProject, advancesPkg, domain.AdvancesCommand{Version: 1, Action: "submit", Reason: "Submit"}); e != nil {
		t.Fatal(e)
	}
}
func TestAdvancesAdvanceDecisionAndPaymentRollBackOnNoticeOrLateCommitFailure(t *testing.T) {
	f := advancesSetup()
	s := NewAdvances(f, f, f)
	advancesPrepareAndSubmit(t, f, s)
	f.noticeFails = true
	q := domain.AdvancesCommand{Version: 2, Action: "approve", Reason: "Independent Finance terms review"}
	if w, e := s.Execute(t.Context(), "finance", advancesProject, advancesPkg, q); e == nil || w.Record != nil || f.record.State != "requested" || f.decisions != 0 || len(f.history) != 2 {
		t.Fatal("notice failure leaked approval", e)
	}
	f.noticeFails = false
	f.commitFails = true
	if w, e := s.Execute(t.Context(), "finance", advancesProject, advancesPkg, q); e == nil || w.Record != nil || f.record.State != "requested" || f.decisions != 0 || len(f.history) != 2 {
		t.Fatal("late commit leaked approval", e)
	}
	f.commitFails = false
	w, e := s.Execute(t.Context(), "finance", advancesProject, advancesPkg, q)
	if e != nil || w.Record.State != "approved" || f.decisions != 1 || w.Record.Amount != "43.71" {
		t.Fatal(e)
	}
	f.commitFails = true
	q = domain.AdvancesCommand{Version: 3, Action: "disburse", Reason: "Record actual voucher", Payment: &domain.AdvancesPayment{Voucher: "UTR-actual", Date: "2026-10-11"}}
	if w, e = s.Execute(t.Context(), "finance", advancesProject, advancesPkg, q); e == nil || w.Record != nil || f.record.Payment != nil || f.record.State != "approved" || len(f.history) != 3 {
		t.Fatal("failed commit published payment", e)
	}
}
func TestAdvancesAdvanceRejectsStaleScopeSelfReviewAndForgedSavedAmountBeforeFinance(t *testing.T) {
	f := advancesSetup()
	s := NewAdvances(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "requester", advancesProject, advancesPkg); e != domain.AdvancesForbidden || f.sourceCalls != 0 {
		t.Fatal("source read before authorization", e)
	}
	f.caps.Read = true
	advancesPrepareAndSubmit(t, f, s)
	q := domain.AdvancesCommand{Version: 2, Action: "approve", Reason: "Review"}
	if _, e := s.Execute(t.Context(), "requester", advancesProject, advancesPkg, q); e != domain.AdvancesForbidden || f.decisions != 0 {
		t.Fatal("self approval", e)
	}
	f.record.Amount = "0.01"
	if _, e := s.Execute(t.Context(), "finance", advancesProject, advancesPkg, q); e != domain.AdvancesStale || f.decisions != 0 {
		t.Fatal("forged saved amount reached Finance", e)
	}
	f.record.Amount = "43.71"
	f.source.AcceptanceVersion++
	if _, e := s.Execute(t.Context(), "finance", advancesProject, advancesPkg, q); e != domain.AdvancesStale || f.decisions != 0 {
		t.Fatal("obsolete vendor response approved", e)
	}
	f.caps.Scoped = false
	w, e := s.Workspace(t.Context(), "requester", advancesProject, advancesPkg)
	if e != nil || !w.TaskOnly || len(w.History) != 0 {
		t.Fatal("task-only history exposed", e)
	}
}
