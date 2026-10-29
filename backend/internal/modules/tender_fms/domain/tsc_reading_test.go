package domain

import "testing"

func TestCompletedChecklistsCannotBecomeSourceEvidence(t *testing.T) {
	for _, name := range []string{"TENDER SELECTION CHECK LIST.docx", "batch/Tender Selection Check List.docx", "TSC format.docx"} {
		if !IsTSCReferenceSource(name) {
			t.Fatal("answer leakage", name)
		}
	}
	for _, name := range []string{"Technical Specification.pdf", "NIT.pdf", "Signed Corrigendum-IX.pdf"} {
		if IsTSCReferenceSource(name) {
			t.Fatal("operative evidence excluded", name)
		}
	}
}

func TestHistoricalChecklistPassagesCannotLeakIntoResearch(t *testing.T) {
	q := TSCQuestion{ID: "particulars.q01", Text: "Employer", Section: "particulars"}
	passages := []TSCPassage{{ID: "answer", Filename: "TENDER SELECTION CHECK LIST.docx", Text: "Employer: Copied answer"}, {ID: "source", Filename: "NIT.pdf", Text: "Employer: Original authority"}}
	for _, p := range RetrieveTSCEvidence(q, passages) {
		if p.ID == "answer" {
			t.Fatal("historical answers retrieved")
		}
	}
	answer := CollectTSCCandidates(TSCInput{Questions: []TSCQuestion{q}, Passages: passages})[q.ID]
	if answer.Text != "Original authority" {
		t.Fatal("historical answer influenced candidate", answer)
	}
}
