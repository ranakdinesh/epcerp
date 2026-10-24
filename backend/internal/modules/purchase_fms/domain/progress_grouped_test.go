package domain

import (
	"testing"
)

func progressGroupedFixture() ProgressGroupedSource {
	line := ProgressGroupedLine{ID: "line", Quantity: "2.005", RequiredDate: "2026-10-11", Snapshot: "canonical full item snapshot"}
	s := ProgressGroupedSource{BOState: "completed", GroupState: "created", Lines: []ProgressGroupedLine{line}, Groups: []ProgressGroupedGroup{{ID: "group", Revision: 2, LineIDs: []string{"line"}, VendorIDs: []string{"a", "b", "c"}}}, ApprovedGroupIDs: []string{"group"}}
	for _, vendor := range s.Groups[0].VendorIDs {
		s.Documents = append(s.Documents, ProgressGroupedDocument{ID: vendor, GroupID: "group", VendorID: vendor, Version: 1, Revision: 2, Active: true, SentDate: "2026-10-10", Lines: []ProgressGroupedLine{line}})
		s.Quotes = append(s.Quotes, ProgressGroupedQuote{DocumentID: vendor, DocumentVersion: 1, Active: true, PricedLineIDs: []string{"line"}})
	}
	return s
}
func TestProgressGroupedFactsRequireActualCompleteCurrentScope(t *testing.T) {
	s := progressGroupedFixture()
	facts, err := ProgressGroupedFacts(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range facts {
		if fact.State != "completed" || fact.Number == 5 {
			t.Fatal("grouped evidence overwrote LOI or failed scope", fact)
		}
	}
	s.Documents[0].Lines[0].Quantity = "2.0050" // Equivalent decimal formatting retains exact scope.
	facts, err = ProgressGroupedFacts(s)
	if err != nil || facts[0].State != "completed" || facts[1].State != "completed" {
		t.Fatal("equivalent decimal lost current grouped scope", facts, err)
	}
}
func TestProgressGroupedFactsInvalidateStaleIncompleteAndDuplicateEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*ProgressGroupedSource)
	}{
		{"scope revision", func(s *ProgressGroupedSource) { s.Documents[0].Revision-- }},
		{"item snapshot", func(s *ProgressGroupedSource) { s.Documents[0].Lines[0].Snapshot = "old item" }},
		{"required date", func(s *ProgressGroupedSource) { s.Documents[0].Lines[0].RequiredDate = "2026-10-12" }},
		{"quantity", func(s *ProgressGroupedSource) { s.Documents[0].Lines[0].Quantity = "2.004" }},
		{"uncovered line", func(s *ProgressGroupedSource) { s.Lines = append(s.Lines, ProgressGroupedLine{ID: "extra"}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := progressGroupedFixture()
			test.edit(&s)
			p, e := ProgressGroupedFacts(s)
			if e != nil || p[0].State == "completed" || p[1].State == "completed" {
				t.Fatal(p, e)
			}
		})
	}
	s := progressGroupedFixture()
	s.Quotes[0].DocumentVersion++
	p, e := ProgressGroupedFacts(s)
	if e != nil || p[1].State == "completed" {
		t.Fatal("quote on changed document counted", p, e)
	}
	s = progressGroupedFixture()
	s.Quotes[0].PricedLineIDs = nil
	p, e = ProgressGroupedFacts(s)
	if e != nil || p[1].State == "completed" {
		t.Fatal("incomplete pricing counted", p, e)
	}
	s = progressGroupedFixture()
	s.Documents = append(s.Documents, s.Documents[0])
	if _, e = ProgressGroupedFacts(s); e == nil {
		t.Fatal("duplicate current document counted")
	}
	s = progressGroupedFixture()
	s.ApprovedGroupIDs = []string{"foreign"}
	if _, e = ProgressGroupedFacts(s); e == nil {
		t.Fatal("foreign approval counted")
	}
}
func TestProgressGroupedFactsPartialStatesAndNoLOIOverride(t *testing.T) {
	if p, e := ProgressGroupedFacts(ProgressGroupedSource{}); e != nil || len(p) != 0 {
		t.Fatal(p, e)
	}
	s := progressGroupedFixture()
	s.ApprovedGroupIDs = nil
	s.ApprovalStarted = true
	s.Documents[0].SentDate = ""
	p, e := ProgressGroupedFacts(s)
	if e != nil || p[1].State != "in_progress" || p[2].State != "in_progress" || len(p) != 3 {
		t.Fatal(p, e)
	}
}
