package domain

import "testing"

func TestProcurementSubmissionDoesNotRequireFinanceBudget(t *testing.T) {
	for _, mode := range []string{"absent", "missing_item", "currency"} {
		t.Run(mode, func(t *testing.T) {
			s := quotationApprovalsSourceFixture()
			switch mode {
			case "absent":
				s.Budget = nil
			case "missing_item":
				s.Budget.Allocations = nil
			case "currency":
				s.Budget.Currency = "USD"
			}
			a, err := QuotationApprovalsSnapshot(s.Quotes[0], s)
			if err != nil || (mode != "missing_item" && a.BudgetIssue == "") || (mode == "missing_item" && a.BudgetIssue != "") || a.Lines[0].Budget != nil {
				t.Fatal("Finance issue blocked procurement handoff or invented a budget", a, err)
			}
			a.State = "pending_accounts"
			if reason := QuotationApprovalsProposalStaleReason(a, s); reason != "" {
				t.Fatal("budget issue staled a pending request", reason)
			}
			redacted := QuotationApprovalsRedact([]QuotationApprovalsAttempt{a})
			if redacted[0].BudgetIssue != "" || redacted[0].Lines[0].Budget != nil {
				t.Fatal("Finance budget details leaked to procurement")
			}
			current := quotationApprovalsSourceFixture()
			current.Budget.Revision = 7
			current.Budget.Allocations[0].Amount = "180.00"
			finance := QuotationApprovalsPendingFinanceBudget(a, current)
			if finance.BudgetIssue != "" || finance.BudgetRevision != 7 || *finance.Lines[0].Budget != "180.00" || *finance.Lines[0].Difference != "-34.31" || finance.Total != a.Total || a.Lines[0].Budget != nil {
				t.Fatal("Finance projection mutated submission or ignored current allocation", finance)
			}
			finance.State = "approved"
			if reason := QuotationApprovalsStaleReason(finance, current); reason != "" {
				t.Fatal(reason)
			}
			current.Budget.Revision++
			if reason := QuotationApprovalsStaleReason(finance, current); reason != "" {
				t.Fatal("budget-only revision invalidated Finance judgement", reason)
			}
		})
	}
}

func TestFinanceApprovalIsNotInvalidatedByBudgetOnlyChanges(t *testing.T) {
	for _, change := range []func(*QuotationApprovalsSource){
		func(s *QuotationApprovalsSource) { s.Budget = nil },
		func(s *QuotationApprovalsSource) { s.Budget.Revision++ },
		func(s *QuotationApprovalsSource) { s.Budget.Currency = "USD" },
		func(s *QuotationApprovalsSource) { s.Budget.Allocations = nil },
	} {
		s := quotationApprovalsSourceFixture()
		a, err := QuotationApprovalsSnapshot(s.Quotes[0], s)
		if err != nil {
			t.Fatal(err)
		}
		a.State = "approved"
		change(&s)
		rows, _, ready := QuotationApprovalsEvaluate([]QuotationApprovalsAttempt{a}, s)
		if !ready || rows[0].EffectiveState != "approved" || rows[0].BudgetRevision != a.BudgetRevision || *rows[0].Lines[0].Budget != *a.Lines[0].Budget {
			t.Fatal("budget altered saved Finance judgement", rows)
		}
	}
}
