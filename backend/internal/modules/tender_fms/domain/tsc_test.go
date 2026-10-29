package domain

import "testing"

func TestTSCCitationAndDecisionGuards(t *testing.T) {
	in := TSCInput{Questions: []TSCQuestion{{ID: "particulars.q01"}, {ID: "conclusion.q02"}}, Passages: []TSCPassage{{ID: "source-1", Text: "Department of synthetic electrical works"}}}
	good := TSCAnswer{QuestionID: "particulars.q01", Status: "answered", Text: "Department of synthetic electrical works", Citations: []TSCCitation{{SourceID: "source-1", Quote: "synthetic electrical works"}}}
	if e := ValidateTSCAnswer(good, in); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*TSCAnswer){func(a *TSCAnswer) { a.Citations = nil }, func(a *TSCAnswer) {
		a.Citations = []TSCCitation{{SourceID: "other", Quote: "synthetic electrical works"}}
	}, func(a *TSCAnswer) { a.Citations = []TSCCitation{{SourceID: "source-1", Quote: "invented text"}} }, func(a *TSCAnswer) { a.QuestionID = "unknown" }, func(a *TSCAnswer) { a.QuestionID = "conclusion.q02" }, func(a *TSCAnswer) { a.Status = "approved" }} {
		candidate := good
		mutate(&candidate)
		if ValidateTSCAnswer(candidate, in) == nil {
			t.Fatal("unsafe answer accepted", candidate)
		}
	}
	q := TSCQuestion{ID: "conclusion.q02"}
	if e := ValidateTSCAnswer(EmptyTSCAnswer(q), in); e != nil {
		t.Fatal(e)
	}
	blank := EmptyTSCAnswer(TSCQuestion{ID: "particulars.q01"})
	if blank.Text != "" || ValidateTSCAnswer(blank, in) != nil {
		t.Fatal("failed/manual answer must remain blank")
	}
	blank.Accepted = true
	if ValidateTSCAnswer(blank, in) == nil {
		t.Fatal("blank answer accepted")
	}
	for _, text := range []string{"answered", "Answered:", "needs_review", "Not found", "unknown"} {
		candidate := good
		candidate.Text = text
		if ValidateTSCAnswer(candidate, in) == nil {
			t.Fatalf("status-only answer accepted: %q", text)
		}
	}
}
