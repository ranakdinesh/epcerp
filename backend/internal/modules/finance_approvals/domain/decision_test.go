package domain

import (
	"testing"
)

func TestBudgetRequestRejectsInexactOrEmptyEvidence(t *testing.T) {
	valid := BudgetRequest{ProjectID: "22222222-2222-4222-8222-222222222222", Version: 1, Revision: 1, Amount: "1000.00", Currency: "INR", Fingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reason: "Decision reason"}
	for _, amount := range []string{"0", "0.00", "-1.00", "1e3", "NaN", "1.001", "100000000000000.00", " 1.00"} {
		q := valid
		q.Amount = amount
		if _, e := q.Validate(); e == nil {
			t.Fatal("invalid amount", amount)
		}
	}
	for _, change := range []func(*BudgetRequest){func(q *BudgetRequest) { q.Version = 0 }, func(q *BudgetRequest) { q.Revision = 0 }, func(q *BudgetRequest) { q.Currency = "inr" }, func(q *BudgetRequest) { q.Fingerprint = "" }, func(q *BudgetRequest) { q.Reason = " " }, func(q *BudgetRequest) { q.ProjectID = "bad" }} {
		q := valid
		change(&q)
		if _, e := q.Validate(); e == nil {
			t.Fatal("invalid evidence accepted")
		}
	}
	for _, amount := range []string{"0.01", "123.4", "99999999999999.99"} {
		q := valid
		q.Amount = amount
		if _, e := q.Validate(); e != nil {
			t.Fatal("valid amount", e)
		}
	}
}
