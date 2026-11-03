package domain

import (
	"math"
	"sort"
	"strings"
)

// Ranking contains no source text and is not a correctness probability.
type TSCRank struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}
type TSCQuestionEvidence struct {
	QuestionID string       `json:"question_id"`
	Passages   []TSCPassage `json:"passages"`
}
type TSCSelection struct {
	Passages   []TSCPassage
	Diagnostic TSCQuestionDiagnostic
}

func TSCEvidenceBytes(p []TSCPassage) int {
	n := 0
	for _, v := range p {
		n += len(v.Text)
	}
	return n
}

// SelectRankedTSC preserves original objects; a model cannot inject new evidence.
// Full paragraphs remain intact. Context beyond the budget is a visible gap.
func SelectRankedTSC(q TSCQuestion, all, candidates []TSCPassage, ranks []TSCRank) (TSCSelection, bool) {
	out := TSCSelection{Passages: []TSCPassage{}, Diagnostic: TSCQuestionDiagnostic{QuestionID: q.ID, Candidates: len(candidates), Method: "bm25_minilm", BaselineBytes: TSCEvidenceBytes(RetrieveTSCEvidence(q, all))}}
	byID := map[string]TSCPassage{}
	seen := map[string]bool{}
	for _, p := range candidates {
		if p.ID == "" || byID[p.ID].ID != "" {
			return out, false
		}
		byID[p.ID] = p
	}
	if len(ranks) != len(candidates) {
		return out, false
	}
	for _, r := range ranks {
		if _, ok := byID[r.ID]; !ok || seen[r.ID] || math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
			return out, false
		}
		seen[r.ID] = true
	}
	ordered := append([]TSCRank{}, ranks...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Score > ordered[j].Score })
	seen = map[string]bool{}
	docs := map[string]bool{}
	add := func(p TSCPassage, required bool) {
		if seen[p.ID] {
			return
		}
		if len(out.Passages) >= 12 || out.Diagnostic.SelectedBytes+len(p.Text) > 9000 {
			if required {
				out.Diagnostic.ContextGap = true
			} else {
				out.Diagnostic.OptionalContextOmitted = true
			}
			return
		}
		seen[p.ID] = true
		out.Passages = append(out.Passages, p)
		docs[p.DocumentID] = true
		out.Diagnostic.SelectedBytes += len(p.Text)
	}
	// Keep every retrieved amendment candidate for applicability review. This does
	// not establish precedence, and excess context becomes a blocking review gap.
	for _, p := range candidates {
		if p.DocumentType == "corrigendum" {
			add(p, true)
		}
	}
	for i, r := range ordered {
		if i >= 6 {
			break
		}
		add(byID[r.ID], i == 0)
	}
	anchors := append([]TSCPassage{}, out.Passages...)
	for _, a := range anchors {
		for i, p := range all {
			if p.ID != a.ID {
				continue
			}
			for _, j := range []int{i - 1, i + 1} {
				if j < 0 || j >= len(all) {
					continue
				}
				n := all[j]
				delta := n.Segment - p.Segment
				if delta < 0 {
					delta = -delta
				}
				if n.DocumentID == p.DocumentID && n.Revision == p.Revision && n.Alternative == p.Alternative && delta <= 1 && !IsTSCReferenceSource(n.Filename) {
					required := tscRequiredContinuation(n.Text)
					for _, c := range n.TableCells {
						required = required || tscRequiredContinuation(c.Text)
					}
					add(n, required)
				}
			}
			break
		}
	}
	out.Diagnostic.Documents = len(docs)
	return out, true
}

func tscRequiredContinuation(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	for _, prefix := range []string{"exception", "provided that", "however", "subject to", "note:", "unless"} {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}
