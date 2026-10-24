package domain

import (
	"testing"
)

func clientRulesFixture() ClientRulesSave {
	return ClientRulesSave{Data: ClientRulesData{ScopeCode: "CAB", Mode: "approved_only", Allowed: []string{"11111111-1111-1111-1111-111111111111"}, Excluded: []string{}, EffectiveFrom: "2026-10-01", Evidence: "Client approved make schedule"}, Version: 1, Active: true, Reason: "Capture client instruction"}
}
func TestClientRulesStrictMakePolicyValidation(t *testing.T) {
	for _, change := range []func(*ClientRulesSave){func(s *ClientRulesSave) { s.Data.Allowed = nil }, func(s *ClientRulesSave) { s.Data.Excluded = s.Data.Allowed }, func(s *ClientRulesSave) { s.Data.Allowed = append(s.Data.Allowed, s.Data.Allowed[0]) }, func(s *ClientRulesSave) { d := "2026-09-01"; s.Data.EffectiveUntil = &d }, func(s *ClientRulesSave) { s.Data.Evidence = "" }, func(s *ClientRulesSave) { s.Data.ProjectID = new(string) }} {
		s := clientRulesFixture()
		change(&s)
		if _, err := s.Validate(); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
	if _, err := clientRulesFixture().Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestClientRulesPolicyMissingDatesAndProjectExclusions(t *testing.T) {
	q := ClientRulesEvaluation{ProjectID: "project", Category: "CAB", Family: "LTPC", Configuration: "LTPC_R2", Manufacturer: "11111111-1111-1111-1111-111111111111", Today: "2026-10-10"}
	if v := ClientRulesEvaluate(nil, q); v.Status != "eligible" || len(v.Reasons) != 0 || len(v.Rules) != 0 {
		t.Fatal(v)
	}
	source := ClientRulesRecord{ID: "source", Data: clientRulesFixture().Data, Active: true, Version: 3}
	if v := ClientRulesEvaluate([]ClientRulesRecord{source}, q); v.Status != "eligible" || v.Rules[0].Version != 3 {
		t.Fatal(v)
	}
	project := q.ProjectID
	exclusion := ClientRulesRecord{ID: "excluded", Data: ClientRulesData{ProjectID: &project, ScopeCode: "*", Mode: "unrestricted", Excluded: []string{q.Manufacturer}, EffectiveFrom: "2026-10-01", Evidence: "Excluded make"}, Active: true, Version: 1}
	if v := ClientRulesEvaluate([]ClientRulesRecord{source, exclusion}, q); v.Status != "blocked" || len(v.Rules) != 2 {
		t.Fatal("project rule relaxed exclusion", v)
	}
	other := "other"
	exclusion.Data.ProjectID = &other
	if v := ClientRulesEvaluate([]ClientRulesRecord{source, exclusion}, q); v.Status != "eligible" {
		t.Fatal("other project affected", v)
	}
	source.Data.EffectiveFrom = "2026-10-11"
	if v := ClientRulesEvaluate([]ClientRulesRecord{source}, q); v.Status != "eligible" || len(v.Rules) != 0 || len(v.Reasons) != 0 {
		t.Fatal("future policy effective", v)
	}
	source.Data.EffectiveFrom = "2026-10-01"
	expired := "2026-10-09"
	source.Data.EffectiveUntil = &expired
	if v := ClientRulesEvaluate([]ClientRulesRecord{source}, q); v.Status != "eligible" || len(v.Rules) != 0 || len(v.Reasons) != 0 {
		t.Fatal("expired policy effective", v)
	}
}

func TestClientRulesOnlyApplicableInstructionsRestrictRelease(t *testing.T) {
	q := ClientRulesEvaluation{ProjectID: "project", Category: "CAB", Family: "LTPC", Configuration: "LTPC_R2", Manufacturer: "11111111-1111-1111-1111-111111111111", Today: "2026-10-10"}
	for _, test := range []struct {
		name   string
		change func(*ClientRulesRecord)
		status string
		rules  int
	}{
		{"inactive", func(r *ClientRulesRecord) { r.Active = false }, "eligible", 0},
		{"other scope", func(r *ClientRulesRecord) { r.Data.ScopeCode = "OTHER" }, "eligible", 0},
		{"other project", func(r *ClientRulesRecord) { other := "other"; r.Data.ProjectID = &other }, "eligible", 0},
		{"approved list excludes make", func(r *ClientRulesRecord) { r.Data.Allowed = []string{"22222222-2222-2222-2222-222222222222"} }, "blocked", 1},
		{"explicit exclusion", func(r *ClientRulesRecord) {
			r.Data.Mode = "unrestricted"
			r.Data.Allowed = nil
			r.Data.Excluded = []string{q.Manufacturer}
		}, "blocked", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := ClientRulesRecord{ID: "instruction", Active: true, Version: 1, Data: clientRulesFixture().Data}
			test.change(&r)
			v := ClientRulesEvaluate([]ClientRulesRecord{r}, q)
			if v.Status != test.status || len(v.Rules) != test.rules || (v.Status == "blocked") != (len(v.Reasons) > 0) {
				t.Fatal(v)
			}
		})
	}
}
