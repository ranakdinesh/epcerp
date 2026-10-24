package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const ordersProject = "11111111-1111-4111-8111-111111111111"
const ordersPkg = "22222222-2222-4222-8222-222222222222"
const ordersActor = "33333333-3333-4333-8333-333333333333"
const ordersRecord = "44444444-4444-4444-8444-444444444444"

type ordersFixture struct {
	order       *domain.OrdersOrder
	history     []domain.OrdersEvent
	caps        domain.OrdersAccess
	source      domain.OrdersSource
	sequence    int64
	commitFails bool
	sourceCalls int
}

func (f *ordersFixture) Within(ctx context.Context, fn func(context.Context, ports.OrdersRepository, ports.OrdersContext) error) error {
	old := f.order
	if old != nil {
		v := domain.OrdersCopy(*old)
		old = &v
	}
	hist := append([]domain.OrdersEvent{}, f.history...)
	seq := f.sequence
	e := fn(ctx, f, f)
	if e == nil && f.commitFails {
		e = errors.New("commit failed")
	}
	if e != nil {
		f.order = old
		f.history = hist
		f.sequence = seq
	}
	return e
}
func (f *ordersFixture) Get(context.Context, string, string) (*domain.OrdersOrder, error) {
	if f.order == nil {
		return nil, nil
	}
	v := domain.OrdersCopy(*f.order)
	return &v, nil
}
func (f *ordersFixture) Save(_ context.Context, v domain.OrdersOrder) (domain.OrdersOrder, error) {
	c := domain.OrdersCopy(v)
	f.order = &c
	return domain.OrdersCopy(c), nil
}
func (f *ordersFixture) Event(_ context.Context, v domain.OrdersEvent) error {
	f.history = append(f.history, v)
	return nil
}
func (f *ordersFixture) History(context.Context, string) ([]domain.OrdersEvent, error) {
	return append([]domain.OrdersEvent{}, f.history...), nil
}
func (f *ordersFixture) DocumentEvent(context.Context, domain.OrdersDocumentEvent) error { return nil }
func (f *ordersFixture) NextNumber(context.Context, int) (int64, error) {
	f.sequence++
	return f.sequence, nil
}
func (f *ordersFixture) Access(context.Context, domain.Principal, string, string) (domain.OrdersAccess, error) {
	return f.caps, nil
}
func (f *ordersFixture) Source(context.Context, string, string) (domain.OrdersSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *ordersFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: ordersActor, Active: true, Superuser: true}, nil
}
func (f *ordersFixture) ID() string { return ordersRecord }
func (f *ordersFixture) Now() domain.OrdersTimestamp {
	return domain.OrdersTimestamp{}.AddDate(2025, 9, 10)
}
func ordersSetup() *ordersFixture {
	return &ordersFixture{caps: domain.OrdersAccess{Read: true, Write: true, Release: true, Work: true, Scoped: true}, source: domain.OrdersSource{OrdersSourceFacts: domain.OrdersSourceFacts{ProjectID: ordersProject, PackageID: ordersPkg, SourcingID: ordersRecord, SourcingVersion: 1, LOIReference: "Actual LOI", LOIDate: "2026-10-11", FinancialReference: "real-finance:fp", SupplierVerification: "real-verified", Supplier: domain.OrdersSupplier{ID: ordersRecord, Name: "Actual supplier", Address: "Actual location"}, Technical: domain.OrdersTechnical{CaseID: ordersRecord, Version: 10, Revision: 1, Reference: "Actual letter", ApprovalDate: "2026-10-11", ApprovalFileID: ordersRecord, QAPApproved: true, ConditionsClosed: true}, ItemID: ordersRecord, Description: "Approved cable", UOM: "metre", Quantity: "10", Currency: "INR", PricingMode: "unit_price", UnitPrice: "12.3456", Base: "123.46", Discount: "0", Tax: "22.23", Freight: "0", Other: "0", Total: "145.69", DeliveryDays: 21, PaymentTerms: "Actual payment", Warranty: "Actual warranty", LOITerms: "Actual agreed terms"}, Available: true}}
}
func TestOrdersCommitFailureReturnsNoOrderAndRollsBackItsAllocatedNumber(t *testing.T) {
	f := ordersSetup()
	s := NewOrders(f, f, f)
	details := domain.OrdersDetails{Date: "2026-10-11", IssuerName: "Actual issuer", IssuerAddress: "Actual office", BillTo: "Actual invoice", DeliveryAddress: "Actual site"}
	w, e := s.Execute(t.Context(), "actor", ordersProject, ordersPkg, domain.OrdersCommand{Action: "prepare", Reason: "Prepare reviewed terms", Details: &details})
	if e != nil || w.Order == nil {
		t.Fatal(e)
	}
	now := f.Now()
	f.order.ClientApproval = domain.OrdersClientApproval{Status: "approved", DraftRevision: f.order.DraftRevision, Submission: &domain.OrdersClientSubmission{Reference: "SUB", Date: "2026-10-11"}, Decision: &domain.OrdersClientDecision{Status: "approved", FileID: ordersRecord}, RecordedBy: ordersActor, RecordedAt: &now}
	f.commitFails = true
	w, e = s.Execute(t.Context(), "actor", ordersProject, ordersPkg, domain.OrdersCommand{Version: 1, Action: "release", Reason: "Commit failure"})
	if e == nil || w.Order != nil || len(w.History) != 0 || f.order.State != "draft" || f.sequence != 0 || len(f.history) != 1 {
		t.Fatal("commit leaked order/number", e)
	}
	if doc, e := s.Document(t.Context(), "actor", ordersProject, ordersPkg); e == nil || doc.ID != "" {
		t.Fatal("failed export audit exposed snapshot")
	}
	f.commitFails = false
	w, e = s.Execute(t.Context(), "actor", ordersProject, ordersPkg, domain.OrdersCommand{Version: 1, Action: "release", Reason: "Release reviewed terms"})
	if e != nil || w.Order.Number != "PO/VTP/2026-27/000001" || f.sequence != 1 {
		t.Fatal(e, w)
	}
}
func TestOrdersAuthorizationPrecedesSourceReadingAndTaskOnlyHistoryIsNarrow(t *testing.T) {
	f := ordersSetup()
	s := NewOrders(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", ordersProject, ordersPkg); e != domain.OrdersForbidden || f.sourceCalls != 0 {
		t.Fatal("source read before authorization", e)
	}
	f.caps.Read = true
	f.caps.Scoped = false
	f.history = []domain.OrdersEvent{{ID: ordersRecord}}
	f.order = &domain.OrdersOrder{ID: ordersRecord, ProjectID: ordersProject, PackageID: ordersPkg, Version: 1, Active: true, State: "draft", Source: f.source}
	w, e := s.Workspace(t.Context(), "actor", ordersProject, ordersPkg)
	if e != nil || !w.TaskOnly || len(w.History) != 0 {
		t.Fatal("task-only reader obtained project history", e)
	}
	f.source.PackageID = ordersRecord
	if w, e = s.Workspace(t.Context(), "actor", ordersProject, ordersPkg); e == nil || w.Order != nil {
		t.Fatal("foreign source exposed")
	}
}
