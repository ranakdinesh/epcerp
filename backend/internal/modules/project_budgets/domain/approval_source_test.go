package domain

import (
	"testing"
)

func TestEffectiveApprovedSourceUsesFrozenRevisionNotDraft(t *testing.T) {
	for _, record := range []*Budget{nil, {Active: false, Approved: &Approved{}}, {Active: true}} {
		if EffectiveApprovedSource(record) != nil {
			t.Fatal("nonapproved record exposed a baseline")
		}
	}
	b := &Budget{ProjectID: "project", Active: true, State: "draft", Revision: 2, Data: Data{Currency: "USD", Allocations: []Allocation{{Active: true, Amount: "999.99"}}}, Approved: &Approved{Revision: 1, DecisionID: "decision", Data: Data{Currency: "INR", Allocations: []Allocation{{BOID: "bo", LineID: "line", Amount: "12.34", Active: true}, {BOID: "bo", LineID: "inactive", Amount: "1.00", Active: false}}}}}
	s := EffectiveApprovedSource(b)
	if s.Revision != 1 || s.Currency != "INR" || s.DecisionID != "decision" || len(s.Allocations) != 1 || s.Allocations[0].Amount != "12.34" {
		t.Fatal("draft replaced approved evidence")
	}
	s.Allocations[0].Amount = "0.00"
	if b.Approved.Data.Allocations[0].Amount != "12.34" {
		t.Fatal("consumer mutated saved evidence")
	}
}
