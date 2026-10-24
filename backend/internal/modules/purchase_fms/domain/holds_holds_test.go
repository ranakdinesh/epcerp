package domain

import (
	"testing"
	"time"
)

func TestHoldsHoldElapsedMergesOverlapsAndFiltersScopes(t *testing.T) {
	start := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	now := start.Add(3 * time.Hour)
	released := start.Add(2 * time.Hour)
	rows := []HoldsRecord{{ID: "one", Active: true, Data: HoldsData{Scope: "project", StartedAt: start}}, {ID: "two", Data: HoldsData{Scope: "category", CategoryCode: "CAB", StartedAt: start.Add(time.Hour), ReleasedAt: &released}}, {ID: "other", Active: true, Data: HoldsData{Scope: "category", CategoryCode: "LIGHT", StartedAt: start.Add(-time.Hour)}}, {ID: "future", Active: true, Data: HoldsData{Scope: "items", DemandIDs: []string{"demand"}, StartedAt: now.Add(time.Hour)}}}
	v := HoldsEvaluate(rows, "demand", "CAB", now)
	if len(v.Holds) != 1 || v.HoldMinutes != 180 || v.FinishImpact != "Unassessed" {
		t.Fatal(v)
	}
	rows[0].Data.ReleasedAt = &released
	rows[0].Active = false
	v = HoldsEvaluate(rows, "demand", "CAB", now)
	if len(v.Holds) != 0 || v.HoldMinutes != 120 {
		t.Fatal(v)
	}
}
func TestHoldsHoldScopeDateAndSelectionValidation(t *testing.T) {
	now := time.Now().UTC()
	id := "11111111-1111-4111-8111-111111111111"
	c := HoldsCreate{Scope: "project", StartedAt: now.Add(-time.Hour), Reason: "Client instruction", Evidence: "Client email", DoerID: id, ResolverID: id}
	if _, e := c.Validate(now); e != nil {
		t.Fatal(e)
	}
	c.StartedAt = now.Add(time.Second)
	if _, e := c.Validate(now); e == nil {
		t.Fatal("future hold")
	}
	c.StartedAt = now.Add(-time.Hour)
	c.Scope = "items"
	if _, e := c.Validate(now); e == nil {
		t.Fatal("empty item scope")
	}
	c.DemandIDs = []string{id, id}
	if _, e := c.Validate(now); e == nil {
		t.Fatal("duplicate demand")
	}
	c.DemandIDs = []string{id}
	c.CategoryCode = "CAB"
	if _, e := c.Validate(now); e == nil {
		t.Fatal("category on item hold")
	}
}
