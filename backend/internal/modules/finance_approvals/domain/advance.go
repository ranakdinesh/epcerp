package domain

import (
	"strings"
	"unicode/utf8"
)

// AdvanceRequest is a validated saved submission fact from the owning purchasing
// capability. It carries no browser grant or permission for Finance review.
type AdvanceRequest struct {
	ProjectID, PackageID, RequestID, OrderID, SubmittedBy, Outcome, Amount, Currency, Fingerprint, Reason string
	Version, OrderVersion                                                                                 int32
	Kind                                                                                                  string // payment or not_applicable
}
type AdvanceDecision struct {
	AdvanceRequest
	ID, ActorID string
	CreatedAt   Timestamp
}

func (q AdvanceRequest) Validate() (AdvanceRequest, error) {
	for _, id := range []string{q.ProjectID, q.PackageID, q.RequestID, q.OrderID, q.SubmittedBy} {
		if !identifier.MatchString(id) {
			return q, Invalid("source", "Review an actual saved advance submission")
		}
	}
	q.Reason = strings.TrimSpace(q.Reason)
	if q.Version < 1 || q.OrderVersion < 2 || (q.Outcome != "approved" && q.Outcome != "rejected") || !amount.MatchString(q.Amount) || !currency.MatchString(q.Currency) || !fingerprint.MatchString(q.Fingerprint) || q.Reason == "" || utf8.RuneCountInString(q.Reason) > 2000 {
		return q, Invalid("source", "Use a validated exact advance decision and reason")
	}
	zero := strings.Trim(q.Amount, "0.") == ""
	if q.Kind != "payment" && q.Kind != "not_applicable" || q.Kind == "payment" && zero || q.Kind == "not_applicable" && !zero {
		return q, Invalid("amount", "Review a positive payment request or an explicit zero-advance disposition")
	}
	return q, nil
}
