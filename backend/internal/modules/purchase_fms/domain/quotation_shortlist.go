package domain

import (
	"strings"
	"unicode/utf8"
)

// Nominations are a draft choice, never vendor or monetary source data.
func QuotationApprovalsValidateNominations(input []QuotationApprovalsNomination) ([]QuotationApprovalsNomination, error) {
	if len(input) < 1 || len(input) > 2 {
		return nil, QuotationApprovalsInvalid("proposals", "Choose one best-price and one preferred offer")
	}
	rows := append([]QuotationApprovalsNomination{}, input...)
	best, preferred := 0, 0
	seen := map[string]bool{}
	for i := range rows {
		n := &rows[i]
		n.BestPriceComment = strings.TrimSpace(n.BestPriceComment)
		n.PreferredComment = strings.TrimSpace(n.PreferredComment)
		if !quotationApprovalsKeyPattern.MatchString(n.QuoteKey) || seen[n.QuoteKey] || (!n.BestPrice && !n.Preferred) {
			return nil, QuotationApprovalsInvalid("proposals", "Choose distinct saved quotation revisions with a designation")
		}
		seen[n.QuoteKey] = true
		if n.BestPrice {
			best++
			if n.BestPriceComment == "" {
				return nil, QuotationApprovalsInvalid("best_price_comment", "Explain the best-price choice")
			}
		}
		if n.Preferred {
			preferred++
			if n.PreferredComment == "" {
				return nil, QuotationApprovalsInvalid("preferred_comment", "Explain the preferred choice")
			}
		}
		if utf8.RuneCountInString(n.BestPriceComment) > 2000 || utf8.RuneCountInString(n.PreferredComment) > 2000 {
			return nil, QuotationApprovalsInvalid("proposals", "Each designation comment must be at most 2000 characters")
		}
	}
	if best != 1 || preferred != 1 {
		return nil, QuotationApprovalsInvalid("proposals", "Choose exactly one best-price and one preferred offer; one offer may have both")
	}
	return rows, nil
}

func QuotationApprovalsShortlist(input []QuotationApprovalsNomination, s QuotationApprovalsSource) (QuotationApprovalsAttempt, error) {
	rows, e := QuotationApprovalsValidateNominations(input)
	if e != nil {
		return QuotationApprovalsAttempt{}, e
	}
	proposals := []QuotationApprovalsProposal{}
	group := ""
	for _, n := range rows {
		var q *QuotationApprovalsQuote
		for i := range s.Quotes {
			if s.Quotes[i].Key == n.QuoteKey {
				q = &s.Quotes[i]
				break
			}
		}
		if q == nil {
			return QuotationApprovalsAttempt{}, QuotationApprovalsMissing
		}
		if group != "" && group != q.GroupID {
			return QuotationApprovalsAttempt{}, QuotationApprovalsInvalid("proposals", "Both proposals must belong to the same current group")
		}
		group = q.GroupID
		if n.BestPrice {
			amount, e := quotationApprovalsMoney(q.Total)
			if e != nil {
				return QuotationApprovalsAttempt{}, e
			}
			for _, other := range s.Quotes {
				if other.GroupID == group && other.Eligible {
					price, e := quotationApprovalsMoney(other.Total)
					if e != nil {
						return QuotationApprovalsAttempt{}, e
					}
					if price.Cmp(amount) < 0 {
						return QuotationApprovalsAttempt{}, QuotationApprovalsInvalid("best_price", "Best price must be the lowest eligible group total")
					}
				}
			}
		}
		snapshot, e := QuotationApprovalsSnapshot(*q, s)
		if e != nil {
			return QuotationApprovalsAttempt{}, e
		}
		proposals = append(proposals, QuotationApprovalsProposal{n, snapshot})
	}
	// Default to best price until Finance explicitly chooses a submitted proposal.
	chosen := proposals[0].Snapshot
	for _, p := range proposals {
		if p.BestPrice {
			chosen = p.Snapshot
		}
	}
	chosen.Proposals = proposals
	return chosen, nil
}

func QuotationApprovalsSelectProposal(a QuotationApprovalsAttempt, key string) (QuotationApprovalsAttempt, error) {
	if len(a.Proposals) == 0 {
		if key != "" && key != a.QuoteKey {
			return a, QuotationApprovalsInvalid("selected_quote_key", "Select the submitted quotation")
		}
		return a, nil
	}
	for _, p := range a.Proposals {
		if p.QuoteKey != key {
			continue
		}
		n := p.Snapshot
		n.ID = a.ID
		n.State = a.State
		n.SubmittedBy = a.SubmittedBy
		n.SubmittedAt = a.SubmittedAt
		n.Reason = a.Reason
		n.Proposals = a.Proposals
		return n, nil
	}
	return a, QuotationApprovalsInvalid("selected_quote_key", "Choose one of the frozen submitted proposals")
}

func QuotationApprovalsProposalStaleReason(a QuotationApprovalsAttempt, s QuotationApprovalsSource) string {
	if a.State == "approved" {
		return QuotationApprovalsStaleReason(a, s)
	}
	if len(a.Proposals) > 0 {
		for _, p := range a.Proposals {
			if reason := QuotationApprovalsStaleReason(p.Snapshot, s); reason != "" {
				return reason
			}
		}
	}
	return QuotationApprovalsStaleReason(a, s)
}

type QuotationApprovalsPrint struct {
	ProjectName, ProjectNumber, GroupID, GroupName, Currency string
	Quotes                                                   []QuotationApprovalsQuote
	Attempt                                                  *QuotationApprovalsAttempt
	Nominations                                              []QuotationApprovalsNomination
	Finance                                                  bool
	GeneratedAt                                              QuotationApprovalsTimestamp
}

func QuotationApprovalsComparisonSheet(source QuotationApprovalsSource, group string, keys []string) ([]QuotationApprovalsQuote, error) {
	if len(keys) > 50 {
		return nil, QuotationApprovalsInvalid("quote_keys", "Print at most 50 vendor revisions")
	}
	selected := map[string]QuotationApprovalsQuote{}
	order := []string{}
	for _, q := range source.Quotes {
		if q.GroupID != group {
			continue
		}
		include := len(keys) == 0
		for _, key := range keys {
			if q.Key == key {
				include = true
			}
		}
		if !include {
			continue
		}
		old, exists := selected[q.DocumentID]
		if !exists {
			selected[q.DocumentID] = q
			order = append(order, q.DocumentID)
			continue
		}
		if len(keys) > 0 {
			return nil, QuotationApprovalsInvalid("quote_keys", "Choose one revision per vendor column")
		}
		if q.Eligible && !old.Eligible {
			selected[q.DocumentID] = q
			continue
		}
		if q.Eligible && old.Eligible {
			price, e := quotationApprovalsMoney(q.Total)
			if e != nil {
				return nil, e
			}
			oldPrice, e := quotationApprovalsMoney(old.Total)
			if e != nil {
				return nil, e
			}
			if price.Cmp(oldPrice) < 0 || (price.Cmp(oldPrice) == 0 && q.Revision > old.Revision) {
				selected[q.DocumentID] = q
			}
		}
		if !q.Eligible && !old.Eligible && q.Revision > old.Revision {
			selected[q.DocumentID] = q
		}
	}
	out := []QuotationApprovalsQuote{}
	for _, id := range order {
		out = append(out, selected[id])
	}
	if len(keys) > 0 && len(out) != len(keys) {
		return nil, QuotationApprovalsMissing
	}
	return out, nil
}

func QuotationApprovalsDraftFinanceNotes(a QuotationApprovalsAttempt, notes map[string]string) (QuotationApprovalsAttempt, error) {
	if len(notes) > 100 || a.State != "pending_accounts" {
		return a, QuotationApprovalsInvalid("finance_notes", "Draft notes require a pending submission")
	}
	ids := map[string]bool{}
	for _, l := range a.Lines {
		ids[l.LineID] = true
	}
	for id, note := range notes {
		if !ids[id] || utf8.RuneCountInString(note) > 2000 {
			return a, QuotationApprovalsInvalid("finance_notes", "Use submitted item identifiers and notes of at most 2000 characters")
		}
	}
	copy := QuotationApprovalsCopyAttempts([]QuotationApprovalsAttempt{a})[0]
	for i := range copy.Lines {
		copy.Lines[i].Comment = strings.TrimSpace(notes[copy.Lines[i].LineID])
	}
	for p := range copy.Proposals {
		for l := range copy.Proposals[p].Snapshot.Lines {
			line := &copy.Proposals[p].Snapshot.Lines[l]
			line.Comment = strings.TrimSpace(notes[line.LineID])
		}
	}
	copy.ReviewComment = "Draft Finance notes - no decision recorded"
	return copy, nil
}
