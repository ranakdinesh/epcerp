package domain

import (
	"math"
	"strings"
	"testing"
)

func TestRankedEvidenceRejectsInventedDuplicateAndNonFiniteScores(t *testing.T) {
	q := TSCQuestion{ID: "emd.q05", Section: "emd", Text: "EMD amount"}
	p := []TSCPassage{{ID: "a", Text: "EMD INR 5000"}, {ID: "b", Text: "Tender fee INR 100"}}
	for _, r := range [][]TSCRank{{{ID: "invented", Score: 1}, {ID: "b", Score: 0}}, {{ID: "a", Score: 1}, {ID: "a", Score: 2}}, {{ID: "a", Score: math.NaN()}, {ID: "b", Score: 0}}, {{ID: "a", Score: math.Inf(1)}, {ID: "b", Score: 0}}, {{ID: "a", Score: 1}}} {
		if _, ok := SelectRankedTSC(q, p, p, r); ok {
			t.Fatal("untrusted ranking accepted")
		}
	}
}
func TestRankedEvidencePreservesCrossPageExceptionsAndAmendments(t *testing.T) {
	q := TSCQuestion{ID: "emd.q05", Section: "emd", Text: "EMD amount"}
	all := []TSCPassage{{ID: "a", DocumentID: "nit", Revision: 1, Segment: 0, Text: "Earnest money INR 500000. Conditions follow."}, {ID: "b", DocumentID: "nit", Revision: 1, Segment: 1, Text: "Exception: no exemption applies to works."}, {ID: "c", DocumentID: "corr", Revision: 1, DocumentType: "corrigendum", Text: "Bid security amended to INR 600000."}}
	candidates := []TSCPassage{all[0], all[2]}
	out, ok := SelectRankedTSC(q, all, candidates, []TSCRank{{ID: "a", Score: 3}, {ID: "c", Score: -9}})
	seen := map[string]bool{}
	for _, p := range out.Passages {
		seen[p.ID] = true
	}
	if !ok || !seen["a"] || !seen["b"] || !seen["c"] || out.Diagnostic.ContextGap {
		t.Fatal("exception or amendment lost", out)
	}
}
func TestOptionalRankedEvidenceOmissionDoesNotBlockCompleteAnchor(t *testing.T) {
	q := TSCQuestion{ID: "emd.q05", Text: "EMD"}
	p := []TSCPassage{}
	r := []TSCRank{}
	for _, id := range []string{"a", "b", "c", "d", "e", "f"} {
		p = append(p, TSCPassage{ID: id, Text: strings.Repeat("x", 1800)})
		r = append(r, TSCRank{ID: id, Score: 1})
	}
	out, ok := SelectRankedTSC(q, p, p, r)
	if !ok || out.Diagnostic.ContextGap || !out.Diagnostic.OptionalContextOmitted || TSCEvidenceBytes(out.Passages) > 9000 {
		t.Fatal("context silently truncated", out)
	}
	for _, v := range out.Passages {
		if len(v.Text) != 1800 {
			t.Fatal("source bytes altered")
		}
	}
}
