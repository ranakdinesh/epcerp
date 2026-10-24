package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type quotationApprovalsFixture struct {
	record       *domain.QuotationApprovalsRecord
	history      []domain.QuotationApprovalsEvent
	commitFail   bool
	budgetAbsent bool
	budgetRead   bool
	budgetWrite  bool
}

const quotationApprovalsProject = "11111111-1111-4111-8111-111111111111"
const quotationApprovalsDocument = "22222222-2222-4222-8222-222222222222"
const quotationApprovalsSubmitter = "33333333-3333-4333-8333-333333333333"
const quotationApprovalsReviewer = "44444444-4444-4444-8444-444444444444"

func (f *quotationApprovalsFixture) Resolve(_ context.Context, token string) (domain.Principal, error) {
	id := quotationApprovalsSubmitter
	if token == "reviewer" {
		id = quotationApprovalsReviewer
	}
	return domain.Principal{ID: id, Active: token != "", Superuser: true}, nil
}
func (f *quotationApprovalsFixture) Actions(context.Context, domain.Principal, string) (domain.QuotationApprovalsActions, error) {
	return domain.QuotationApprovalsActions{Read: true, Submit: true, ReviewRead: true, Review: true, BudgetRead: f.budgetRead, BudgetWrite: f.budgetRead && f.budgetWrite}, nil
}
func (f *quotationApprovalsFixture) Source(context.Context, string, string) (domain.QuotationApprovalsSource, error) {
	source := domain.QuotationApprovalsSource{ProjectID: quotationApprovalsProject, BOID: quotationApprovalsProject, Currency: "INR", Available: true, Groups: []domain.QuotationApprovalsGroup{{ID: quotationApprovalsProject}}, Quotes: []domain.QuotationApprovalsQuote{{Key: quotationApprovalsDocument + ":1", DocumentID: quotationApprovalsDocument, Revision: 1, GroupID: quotationApprovalsProject, VendorID: quotationApprovalsProject, ScopeRevision: 1, Total: "145.69", Eligible: true, Lines: []domain.QuotationApprovalsQuoteLine{{LineID: quotationApprovalsProject, Quantity: "10.0000", Quoted: "145.69"}}}}, Budget: &domain.QuotationApprovalsBudget{Revision: 1, Currency: "INR", Allocations: []domain.QuotationApprovalsBudgetAllocation{{BOID: quotationApprovalsProject, LineID: quotationApprovalsProject, Amount: "200.00"}}}, Verifications: map[string]domain.QuotationApprovalsVerification{quotationApprovalsProject: {State: "verified", Fingerprint: "current"}}}
	if f.budgetAbsent {
		source.Budget = nil
	}
	return source, nil
}
func (f *quotationApprovalsFixture) Get(context.Context, string, string) (*domain.QuotationApprovalsRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	r := domain.QuotationApprovalsCopyRecord(*f.record)
	return &r, nil
}
func (f *quotationApprovalsFixture) Save(_ context.Context, r domain.QuotationApprovalsRecord, _ string) (domain.QuotationApprovalsRecord, error) {
	r = domain.QuotationApprovalsCopyRecord(r)
	f.record = &r
	return r, nil
}
func (f *quotationApprovalsFixture) History(context.Context, string, string) ([]domain.QuotationApprovalsEvent, error) {
	return append([]domain.QuotationApprovalsEvent{}, f.history...), nil
}
func (f *quotationApprovalsFixture) Event(_ context.Context, e domain.QuotationApprovalsEvent) error {
	f.history = append(f.history, e)
	return nil
}
func (f *quotationApprovalsFixture) Decide(context.Context, domain.Principal, domain.QuotationApprovalsRecord, domain.QuotationApprovalsAttempt, string) (string, error) {
	return quotationApprovalsReviewer, nil
}
func (f *quotationApprovalsFixture) ID() string { return quotationApprovalsDocument }
func (f *quotationApprovalsFixture) Now() domain.QuotationApprovalsTimestamp {
	return domain.QuotationApprovalsTimestamp{}
}
func (f *quotationApprovalsFixture) Within(ctx context.Context, work func(context.Context, ports.QuotationApprovalsRepository, ports.QuotationApprovalsContext, ports.QuotationApprovalsFinance) error) error {
	before := f.record
	events := append([]domain.QuotationApprovalsEvent{}, f.history...)
	e := work(ctx, f, f, f)
	if e == nil && f.commitFail {
		e = errors.New("fixture commit failed")
	}
	if e != nil {
		f.record = before
		f.history = events
	}
	return e
}
func TestQuotationApprovalsReviewCommitFailureExposesNoDerivedWorkspace(t *testing.T) {
	f := &quotationApprovalsFixture{}
	s := NewQuotationApprovals(f, f, f)
	w, e := s.Execute(t.Context(), "submitter", quotationApprovalsProject, quotationApprovalsProject, "submit", domain.QuotationApprovalsCommand{Reason: "Select current quote", QuoteKey: quotationApprovalsDocument + ":1"})
	if e != nil || w.Version != 1 {
		t.Fatal(e)
	}
	review := domain.QuotationApprovalsCommand{Version: 1, AttemptID: w.Attempts[0].ID, Reason: "Accept every item", Decisions: []domain.QuotationApprovalsDecision{{LineID: quotationApprovalsProject, Decision: "accept", Comment: "Within budget"}}}
	if _, e = s.Execute(t.Context(), "submitter", quotationApprovalsProject, quotationApprovalsProject, "review", review); e == nil {
		t.Fatal("recovery self review accepted")
	}
	f.commitFail = true
	w, e = s.Execute(t.Context(), "reviewer", quotationApprovalsProject, quotationApprovalsProject, "review", review)
	if e == nil || w.Version != 0 || len(w.Attempts) != 0 || len(w.History) != 0 || f.record.Version != 1 || len(f.history) != 1 {
		t.Fatal("failed commit exposed financial results", e)
	}
	f.commitFail = false
	w, e = s.Execute(t.Context(), "reviewer", quotationApprovalsProject, quotationApprovalsProject, "review", review)
	if e != nil || w.Version != 2 || !w.Ready {
		t.Fatal("review did not commit", e)
	}
}

func TestFinanceOwnsBudgetResolutionAfterProcurementHandoff(t *testing.T) {
	for _, outcome := range []string{"reject_without_budget", "approve_after_budget"} {
		t.Run(outcome, func(t *testing.T) {
			f := &quotationApprovalsFixture{budgetAbsent: true}
			service := NewQuotationApprovals(f, f, f)
			pending, err := service.Execute(t.Context(), "submitter", quotationApprovalsProject, quotationApprovalsProject, "submit", domain.QuotationApprovalsCommand{Reason: "Purchaser submitted items", QuoteKey: quotationApprovalsDocument + ":1"})
			if err != nil || pending.Version != 1 || pending.Attempts[0].EffectiveState != "pending_accounts" {
				t.Fatal("pre-budget submission", err)
			}
			decision := domain.QuotationApprovalsCommand{Version: 1, AttemptID: pending.Attempts[0].ID, Reason: "Finance reviewed", Decisions: []domain.QuotationApprovalsDecision{{LineID: quotationApprovalsProject, Decision: "accept", Comment: "Reviewed"}}}
			if _, err := service.Execute(t.Context(), "reviewer", quotationApprovalsProject, quotationApprovalsProject, "review", decision); err == nil || f.record.Version != 1 || len(f.history) != 1 {
				t.Fatal("missing budget approval was accepted", err)
			}
			if outcome == "reject_without_budget" {
				decision.Decisions[0].Decision = "reject"
			} else if outcome == "approve_after_budget" {
				f.budgetAbsent = false
			}
			result, err := service.Execute(t.Context(), "reviewer", quotationApprovalsProject, quotationApprovalsProject, "review", decision)
			if err != nil || result.Version != 2 {
				t.Fatal("Finance could not resolve existing submission", err)
			}
			if outcome == "approve_after_budget" {
				a := result.Attempts[0]
				if !result.Ready || a.BudgetRevision != 1 || a.Lines[0].Budget == nil || *a.Lines[0].Budget != "200.00" || a.Total != pending.Attempts[0].Total {
					t.Fatal("approved budget not bound at decision", a)
				}
			} else if result.Attempts[0].EffectiveState != "rejected" {
				t.Fatal("budget prevented Finance rejection", result)
			}
		})
	}
}

func TestQuotationReviewBudgetActionsFollowExplicitCapabilities(t *testing.T) {
	for _, caps := range []struct{ read, write bool }{{false, false}, {true, false}, {true, true}} {
		f := &quotationApprovalsFixture{budgetRead: caps.read, budgetWrite: caps.write}
		workspace, err := NewQuotationApprovals(f, f, f).Workspace(t.Context(), "reviewer", quotationApprovalsProject, quotationApprovalsProject)
		if err != nil || workspace.CanReadBudget != caps.read || workspace.CanUpdateBudget != (caps.read && caps.write) {
			t.Fatal("budget permissions lost in review projection", workspace, err)
		}
	}
}
