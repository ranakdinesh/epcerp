package domain

import (
	"testing"
)

func advanceFixture() AdvanceRequest {
	return AdvanceRequest{ProjectID: "11111111-1111-4111-8111-111111111111", PackageID: "22222222-2222-4222-8222-222222222222", RequestID: "33333333-3333-4333-8333-333333333333", OrderID: "44444444-4444-4444-8444-444444444444", SubmittedBy: "55555555-5555-4555-8555-555555555555", Outcome: "approved", Amount: "100.00", Currency: "INR", Fingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Version: 2, OrderVersion: 2, Kind: "payment", Reason: "Reviewed actual evidence"}
}
func TestAdvanceDecisionInputIsExactSavedEvidence(t *testing.T) {
	q := advanceFixture()
	if _, e := q.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*AdvanceRequest){func(q *AdvanceRequest) { q.ProjectID = "forged" }, func(q *AdvanceRequest) { q.RequestID = "" }, func(q *AdvanceRequest) { q.OrderVersion = 1 }, func(q *AdvanceRequest) { q.Version = 0 }, func(q *AdvanceRequest) { q.Outcome = "disbursed" }, func(q *AdvanceRequest) { q.Amount = "1e2" }, func(q *AdvanceRequest) { q.Amount = "100.001" }, func(q *AdvanceRequest) { q.Amount = "-1" }, func(q *AdvanceRequest) { q.Amount = "0.00" }, func(q *AdvanceRequest) { q.Kind = "not_applicable" }, func(q *AdvanceRequest) { q.Currency = "rupees" }, func(q *AdvanceRequest) { q.Fingerprint = "caller approved" }, func(q *AdvanceRequest) { q.Reason = "  " }} {
		q := advanceFixture()
		edit(&q)
		if _, e := q.Validate(); e == nil {
			t.Fatal("invalid financial evidence accepted")
		}
	}
	q.Kind = "not_applicable"
	q.Amount = "0.00"
	if _, e := q.Validate(); e != nil {
		t.Fatal("explicit waiver rejected", e)
	}
}
