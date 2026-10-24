package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type quotationApprovalsQueueFixture struct {
	quotationApprovalsFixture
	labelCalls int
	repoCalls  int
	rows       []domain.QuotationApprovalsRecord
}

func (f *quotationApprovalsQueueFixture) Get(_ context.Context, p, b string) (*domain.QuotationApprovalsRecord, error) {
	for _, r := range f.rows {
		if r.ProjectID == p && r.BOID == b {
			r.Active = true
			return &r, nil
		}
	}
	return nil, nil
}
func (f *quotationApprovalsQueueFixture) Queue(context.Context, int32) ([]domain.QuotationApprovalsRecord, error) {
	f.repoCalls++
	return f.rows, nil
}
func (f *quotationApprovalsQueueFixture) ProjectLabel(_ context.Context, p string) (string, string, error) {
	f.labelCalls++
	if p != quotationApprovalsProject {
		return "", "", errors.New("unauthorized project label requested")
	}
	return "Assigned project", "PRJ-1", nil
}
func (f *quotationApprovalsQueueFixture) Actions(_ context.Context, _ domain.Principal, p string) (domain.QuotationApprovalsActions, error) {
	return domain.QuotationApprovalsActions{ReviewRead: p == quotationApprovalsProject}, nil
}
func (f *quotationApprovalsQueueFixture) Within(ctx context.Context, work func(context.Context, ports.QuotationApprovalsRepository, ports.QuotationApprovalsContext, ports.QuotationApprovalsFinance) error) error {
	e := work(ctx, f, f, f)
	if e == nil && f.commitFail {
		return errors.New("fixture commit failed")
	}
	return e
}
func (f *quotationApprovalsQueueFixture) Resolve(_ context.Context, token string) (domain.Principal, error) {
	return domain.Principal{ID: quotationApprovalsReviewer, Active: token != "", Permissions: []string{"accounts.quotation_approvals.view", "accounts.quotation_approvals.read"}}, nil
}
func TestQuotationApprovalsQueueFiltersBeforeReadingProjectLabelsAndClearsFailedCommit(t *testing.T) {
	f := &quotationApprovalsQueueFixture{rows: []domain.QuotationApprovalsRecord{{ProjectID: quotationApprovalsProject, BOID: quotationApprovalsDocument, Version: 1, Attempts: []domain.QuotationApprovalsAttempt{{ID: "first", GroupID: "one", GroupName: "First", State: "pending_accounts"}, {ID: "second", GroupID: "two", GroupName: "Second", State: "pending_accounts"}, {ID: "old", GroupID: "old-group", State: "rejected"}, {ID: "latest", GroupID: "old-group", State: "approved"}}}, {ProjectID: quotationApprovalsDocument, BOID: quotationApprovalsProject, Version: 2}}}
	s := NewQuotationApprovals(f, f, f)
	out, e := s.Queue(t.Context(), "accounts", 0)
	if e != nil || len(out.Rows) != 3 || out.Rows[0].ProjectID != quotationApprovalsProject || f.labelCalls != 1 {
		t.Fatal("queue scope leak", out, e)
	}
	if out.Rows[0].AttemptID != "first" || out.Rows[1].AttemptID != "second" || out.Rows[2].AttemptID != "latest" {
		t.Fatal("group attempts collapsed or superseded attempt duplicated", out)
	}
	f.commitFail = true
	out, e = s.Queue(t.Context(), "accounts", 0)
	if e == nil || len(out.Rows) != 0 {
		t.Fatal("failed transaction exposed rows")
	}
	calls := f.repoCalls
	if _, e = s.Queue(t.Context(), "accounts", -1); e == nil || f.repoCalls != calls {
		t.Fatal("invalid offset reached persistence")
	}
	if _, e = s.Queue(t.Context(), "", 0); e == nil || f.repoCalls != calls {
		t.Fatal("anonymous queue reached persistence")
	}
}
