package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const acceptanceProject = "11111111-1111-4111-8111-111111111111"
const acceptancePkg = "22222222-2222-4222-8222-222222222222"
const acceptanceActor = "33333333-3333-4333-8333-333333333333"
const acceptanceRecord = "44444444-4444-4444-8444-444444444444"

type acceptanceFixture struct {
	record      *domain.AcceptanceRecord
	history     []domain.AcceptanceEvent
	caps        domain.AcceptanceAccess
	source      domain.AcceptanceSource
	commitFails bool
	sourceCalls int
}

func (f *acceptanceFixture) Within(ctx context.Context, fn func(context.Context, ports.AcceptanceRepository, ports.AcceptanceContext) error) error {
	old := f.record
	if old != nil {
		v := *old
		old = &v
	}
	hist := append([]domain.AcceptanceEvent{}, f.history...)
	e := fn(ctx, f, f)
	if e == nil && f.commitFails {
		e = errors.New("commit failed")
	}
	if e != nil {
		f.record = old
		f.history = hist
	}
	return e
}
func (f *acceptanceFixture) Get(context.Context, string, string) (*domain.AcceptanceRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := *f.record
	return &v, nil
}
func (f *acceptanceFixture) Save(_ context.Context, v domain.AcceptanceRecord) (domain.AcceptanceRecord, error) {
	c := v
	f.record = &c
	return c, nil
}
func (f *acceptanceFixture) Event(_ context.Context, v domain.AcceptanceEvent) error {
	f.history = append(f.history, v)
	return nil
}
func (f *acceptanceFixture) History(context.Context, string) ([]domain.AcceptanceEvent, error) {
	return append([]domain.AcceptanceEvent{}, f.history...), nil
}
func (f *acceptanceFixture) Access(context.Context, domain.Principal, string, string) (domain.AcceptanceAccess, error) {
	return f.caps, nil
}
func (f *acceptanceFixture) Source(context.Context, string, string) (domain.AcceptanceSource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *acceptanceFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: acceptanceActor, Active: true, Superuser: true}, nil
}
func (f *acceptanceFixture) ID() string { return acceptanceRecord }
func (f *acceptanceFixture) Now() domain.AcceptanceTimestamp {
	return domain.AcceptanceTimestamp{}.AddDate(2025, 9, 10)
}
func acceptanceSetup() *acceptanceFixture {
	now := (&acceptanceFixture{}).Now()
	return &acceptanceFixture{caps: domain.AcceptanceAccess{Read: true, Write: true, Work: true, Scoped: true}, source: domain.AcceptanceSource{ProjectID: acceptanceProject, PackageID: acceptancePkg, Available: true, Order: &domain.AcceptanceOrder{ID: acceptanceRecord, ProjectID: acceptanceProject, PackageID: acceptancePkg, Version: 2, Number: "PO/VTP/2026-27/000001", Date: "2026-10-10", SupplierID: acceptanceRecord, SupplierName: "Actual vendor", Quantity: "10", UOM: "metre", Currency: "INR", Total: "145.69", ReleasedAt: now, ReleasedBy: acceptanceActor}}}
}
func TestAcceptanceCommitFailurePublishesNoAcceptance(t *testing.T) {
	f := acceptanceSetup()
	s := NewAcceptance(f, f, f)
	q := domain.AcceptanceCommand{Action: "record", Reason: "Vendor confirmed", Response: domain.AcceptanceResponse{Status: "accepted", AcceptedDate: "2026-10-11"}}
	f.commitFails = true
	w, e := s.Execute(t.Context(), "actor", acceptanceProject, acceptancePkg, q)
	if e == nil || w.Record != nil || len(w.History) != 0 || f.record != nil || len(f.history) != 0 {
		t.Fatal("failed commit published acceptance", e)
	}
	f.commitFails = false
	w, e = s.Execute(t.Context(), "actor", acceptanceProject, acceptancePkg, q)
	if e != nil || w.Record == nil || w.Record.Version != 1 || len(w.History) != 1 {
		t.Fatal("acceptance did not commit", e)
	}
	q.Version = 1
	q.Response.Status = "pending"
	q.Response.AcceptedDate = ""
	q.Reason = "Response withdrawn"
	f.commitFails = true
	if w, e = s.Execute(t.Context(), "actor", acceptanceProject, acceptancePkg, q); e == nil || w.Record != nil || f.record.Response.Status != "accepted" || len(f.history) != 1 {
		t.Fatal("failed correction leaked", e)
	}
}
func TestAcceptanceAuthorizationPrecedesEvidenceAndTaskOnlyHistoryIsNarrow(t *testing.T) {
	f := acceptanceSetup()
	s := NewAcceptance(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", acceptanceProject, acceptancePkg); e != domain.AcceptanceForbidden || f.sourceCalls != 0 {
		t.Fatal("source read before authorization", e)
	}
	f.caps.Read = true
	f.caps.Scoped = false
	f.history = []domain.AcceptanceEvent{{ID: acceptanceRecord}}
	f.record = &domain.AcceptanceRecord{ID: acceptanceRecord, ProjectID: acceptanceProject, PackageID: acceptancePkg, Version: 1, Active: true, Order: *f.source.Order}
	w, e := s.Workspace(t.Context(), "actor", acceptanceProject, acceptancePkg)
	if e != nil || !w.TaskOnly || len(w.History) != 0 {
		t.Fatal("task-only history leaked", e)
	}
	f.source.PackageID = acceptanceRecord
	if w, e = s.Workspace(t.Context(), "actor", acceptanceProject, acceptancePkg); e == nil || w.Record != nil {
		t.Fatal("foreign source exposed")
	}
}
