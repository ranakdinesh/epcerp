package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

// TSCEvidence owns one selection path for generation, research and paid identity.
type TSCEvidence struct{ ranker ports.TSCReranker }

func NewTSCEvidence(r ports.TSCReranker) *TSCEvidence { return &TSCEvidence{ranker: r} }
func (s *TSCEvidence) Model() string {
	if s == nil || s.ranker == nil {
		return "bm25-v1"
	}
	return s.ranker.Model()
}
func (s *TSCEvidence) Select(ctx context.Context, q domain.TSCQuestion, all []domain.TSCPassage) domain.TSCSelection {
	p := domain.RetrieveTSCEvidence(q, all)
	out := domain.TSCSelection{Passages: p, Diagnostic: domain.TSCSearchDiagnostic(q, all)}
	out.Diagnostic.Method = "bm25"
	out.Diagnostic.BaselineBytes = domain.TSCEvidenceBytes(p)
	out.Diagnostic.SelectedBytes = out.Diagnostic.BaselineBytes
	if s == nil || s.ranker == nil || domain.TSCRequiresHumanReview(q) {
		return out
	}
	candidates := domain.TSCEvidenceCandidates(q, all)
	if len(candidates) == 0 {
		return out
	}
	ranks, e := s.ranker.Rank(ctx, q, candidates)
	if e != nil {
		out.Diagnostic.Fallback = "local_ranking_unavailable"
		return out
	}
	selected, ok := domain.SelectRankedTSC(q, all, candidates, ranks)
	if !ok {
		out.Diagnostic.Fallback = "invalid_local_ranking"
		return out
	}
	selected.Diagnostic.Model = s.Model()
	return selected
}
func (s *TSC) WithEvidence(e *TSCEvidence) *TSC { s.evidence = e; return s }
func (s *TSC) selectEvidence(ctx context.Context, q domain.TSCQuestion, in domain.TSCInput) domain.TSCSelection {
	if in.RetrievalModel == "" {
		return (*TSCEvidence)(nil).Select(ctx, q, in.Passages)
	}
	return s.evidence.Select(ctx, q, in.Passages)
}
