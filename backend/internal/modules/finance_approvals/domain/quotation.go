package domain

import (
	"strings"
	"unicode/utf8"
)

type QuotationRequest struct {
	ProjectID, BOID, AttemptID, SubmittedBy, Outcome, Amount, Currency, Fingerprint, Reason string
	Version                                                                                 int32
}
type QuotationDecision struct {
	QuotationRequest
	ID, ActorID string
	CreatedAt   Timestamp
}

func (q QuotationRequest) Validate() (QuotationRequest, error) {
	for _, id := range []string{q.ProjectID, q.BOID, q.AttemptID, q.SubmittedBy} {
		if !identifier.MatchString(id) {
			return q, Invalid("source", "Use a saved quotation submission")
		}
	}
	q.Reason = strings.TrimSpace(q.Reason)
	if q.Version < 1 || (q.Outcome != "approved" && q.Outcome != "rejected") || !amount.MatchString(q.Amount) || !currency.MatchString(q.Currency) || !fingerprint.MatchString(q.Fingerprint) || q.Reason == "" || utf8.RuneCountInString(q.Reason) > 2000 {
		return q, Invalid("source", "Use a validated quotation decision and reason")
	}
	return q, nil
}
