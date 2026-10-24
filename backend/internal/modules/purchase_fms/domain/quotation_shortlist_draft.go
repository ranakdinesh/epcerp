package domain

import "strings"

type QuotationShortlistDraft struct {
	Active    bool                           `json:"is_active"`
	ProjectID string                         `json:"project_id"`
	BOID      string                         `json:"bo_id"`
	GroupID   string                         `json:"group_id"`
	Version   int32                          `json:"version"`
	Proposals []QuotationApprovalsNomination `json:"proposals"`
	Comment   string                         `json:"comment"`
	UpdatedBy string                         `json:"updated_by"`
	UpdatedAt QuotationApprovalsTimestamp    `json:"updated_at"`
}

// Draft choices can be prepared before supplier verification or budget approval.
// Submission still rebuilds and validates the full immutable financial snapshot.
func QuotationShortlistDraftValidate(d QuotationShortlistDraft, s QuotationApprovalsSource, attempts []QuotationApprovalsAttempt) (QuotationShortlistDraft, error) {
	if d.Version < 0 {
		return d, QuotationApprovalsInvalid("version", "Use a nonnegative draft version")
	}
	if _, e := QuotationApprovalsID(d.GroupID); e != nil {
		return d, e
	}
	current := false
	for _, g := range s.Groups {
		current = current || g.ID == d.GroupID
	}
	if !current {
		return d, QuotationApprovalsInvalid("group_id", "Choose a current purchase group")
	}
	rows, e := QuotationApprovalsValidateNominations(d.Proposals)
	if e != nil {
		return d, e
	}
	d.Proposals = rows
	d.Comment = strings.TrimSpace(d.Comment)
	if len([]rune(d.Comment)) > 2000 {
		return d, QuotationApprovalsInvalid("comment", "Use at most 2000 characters")
	}
	_, latest, _ := QuotationApprovalsEvaluate(attempts, s)
	if a, ok := latest[d.GroupID]; ok && (a.EffectiveState == "approved" || a.EffectiveState == "pending_accounts") {
		return d, QuotationApprovalsInvalid("group_id", "This group is frozen for Finance review or approved")
	}
	for _, n := range rows {
		found := false
		for _, q := range s.Quotes {
			if q.Key != n.QuoteKey {
				continue
			}
			found = true
			if q.GroupID != d.GroupID || !q.Eligible {
				return d, QuotationApprovalsInvalid("proposals", "Choose eligible offers from this group")
			}
			if n.BestPrice {
				price, e := quotationApprovalsMoney(q.Total)
				if e != nil {
					return d, e
				}
				for _, other := range s.Quotes {
					if other.GroupID == d.GroupID && other.Eligible {
						v, e := quotationApprovalsMoney(other.Total)
						if e != nil {
							return d, e
						}
						if v.Cmp(price) < 0 {
							return d, QuotationApprovalsInvalid("best_price", "Best price must be the lowest eligible group total")
						}
					}
				}
			}
		}
		if !found {
			return d, QuotationApprovalsMissing
		}
	}
	return d, nil
}
