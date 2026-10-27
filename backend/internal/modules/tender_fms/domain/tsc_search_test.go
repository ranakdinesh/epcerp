package domain

import (
	"fmt"
	"strings"
	"testing"
)

func TestStructuredRetrievalFindsAliasesRetainsConditionsAndAmendments(t *testing.T) {
	rows := []TSCPassage{}
	for i := 0; i < 40; i++ {
		rows = append(rows, TSCPassage{ID: fmt.Sprint(i), DocumentID: "gcc", Text: "Repeated general tender submission contract page header", Suppressed: true})
	}
	rows = append(rows, TSCPassage{ID: "emd", DocumentID: "notice", DocumentType: "notice", Text: "Earnest money deposit is INR 10000. See exception below.", Segment: 1}, TSCPassage{ID: "exception", DocumentID: "notice", Text: "Exception: no waiver for works contracts.", Segment: 1}, TSCPassage{ID: "amendment", DocumentID: "amendment", DocumentType: "corrigendum", Text: "Bid security amount is amended to INR 20000."})
	selected := RetrieveTSCEvidence(TSCQuestion{ID: "emd.q05", Section: "emd", Text: "Value of EMD"}, rows)
	found := map[string]bool{}
	bytes := 0
	for _, p := range selected {
		found[p.ID] = true
		bytes += len(p.Text)
	}
	if !found["emd"] || !found["exception"] || !found["amendment"] || len(selected) > 16 || bytes > 14000 {
		t.Fatalf("evidence missing or unbounded: %v", found)
	}
}

func TestLabelledCandidatesKeepConflictsAndNeverDecideEligibility(t *testing.T) {
	in := TSCInput{Questions: []TSCQuestion{{ID: "particulars.q02"}, {ID: "particulars.q07"}, {ID: "financial.q01"}}, Passages: []TSCPassage{
		{ID: "a", Text: "Name of work: Synthetic lighting works\nCompletion period: 90 days excluding monsoon"},
		{ID: "b", Text: "Completion period: 120 days from site handover", DocumentType: "corrigendum"},
		{ID: "c", Text: "Annual turnover: INR 10000000"},
	}}
	result := CollectTSCCandidates(in)
	if result["particulars.q02"].Status != "answered" || result["particulars.q07"].Status != "conflicting" || !strings.Contains(result["particulars.q07"].Text, "excluding monsoon") {
		t.Fatal("source candidate or conditions lost")
	}
	if _, ok := result["financial.q01"]; ok {
		t.Fatal("company eligibility invented")
	}
	for _, a := range result {
		if err := ValidateTSCAnswer(a, in); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEvidenceMarkdownRetainsSuppressedSources(t *testing.T) {
	data := string(EvidenceMarkdown(TSCInput{Passages: []TSCPassage{{ID: "body", Text: "Synthetic clause"}, {ID: "header", Text: "Synthetic page header", Suppressed: true}}}))
	for _, term := range []string{"Synthetic clause", "Synthetic page header", "retained for fallback"} {
		if !strings.Contains(data, term) {
			t.Fatal("evidence export omitted", term)
		}
	}
}
