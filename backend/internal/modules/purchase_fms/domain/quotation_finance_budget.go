package domain

// QuotationApprovalsFinanceBudget attaches the live approved baseline without changing commercial evidence.
// A missing baseline is a Finance work item; it never prevents procurement submission.
func QuotationApprovalsFinanceBudget(a QuotationApprovalsAttempt, s QuotationApprovalsSource) QuotationApprovalsAttempt {
	a.Lines = append([]QuotationApprovalsLine{}, a.Lines...)
	a.BudgetRevision = 0
	a.BudgetIssue = ""
	for i := range a.Lines {
		a.Lines[i].Budget, a.Lines[i].Difference = nil, nil
	}
	if s.Budget == nil || s.Budget.Revision < 1 {
		a.BudgetIssue = "Approve the project budget before approving this vendor."
		return a
	}
	if s.Budget.Currency != s.Currency {
		a.BudgetIssue = "Approve the project budget in the quotation currency before approving this vendor."
		return a
	}
	a.BudgetRevision = s.Budget.Revision
	allocations := map[string]string{}
	for _, allocation := range s.Budget.Allocations {
		if allocation.BOID == s.BOID {
			allocations[allocation.LineID] = allocation.Amount
		}
	}
	for i := range a.Lines {
		line := &a.Lines[i]
		budget, ok := allocations[line.LineID]
		if !ok {
			continue
		}
		difference, err := quotationApprovalsDifference(line.Quoted, budget)
		if err != nil {
			a.BudgetIssue = "Resolve the approved item budget amounts before approving this vendor."
			continue
		}
		line.Budget, line.Difference = &budget, &difference
	}
	return a
}

// Pending requests display current budgets to Finance without mutating saved submissions.
func QuotationApprovalsPendingFinanceBudget(a QuotationApprovalsAttempt, s QuotationApprovalsSource) QuotationApprovalsAttempt {
	a = QuotationApprovalsFinanceBudget(a, s)
	a.Proposals = append([]QuotationApprovalsProposal{}, a.Proposals...)
	for i := range a.Proposals {
		a.Proposals[i].Snapshot = QuotationApprovalsFinanceBudget(a.Proposals[i].Snapshot, s)
	}
	return a
}
