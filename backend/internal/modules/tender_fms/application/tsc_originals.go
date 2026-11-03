package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

func (s *TSC) researchOriginals(ctx context.Context, j domain.TSCJob, q domain.TSCQuestion, key string, version int64) (out domain.TSCResearch, e error) {
	out.QuestionID = q.ID
	out.Passages = []domain.TSCPassage{}
	out.Warning = "Original source PDFs were explicitly requested for this question. Review the proposal and citations; it is not accepted automatically."
	if domain.TSCRequiresHumanReview(q) {
		return out, domain.Invalid("This question requires a human decision")
	}
	provider := s.providerFor("gemini")
	if provider == nil || provider.Model() == "" {
		return out, domain.CaptureFailure(503, "provider_not_configured", "Gemini is not approved and configured")
	}
	original, ok := provider.(ports.TSCOriginalProvider)
	if !ok {
		return out, domain.Invalid("Original-document proposals are unavailable")
	}
	var docs []domain.TSCOriginal
	seenSources := map[string]bool{}
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
		if e := tscLiveJob(ctx, r, d, j, version); e != nil {
			return e
		}
		if e := r.Authorize(ctx, j.ActorID, "originals.write"); e != nil {
			return e
		}
		draft, e := d.Get(ctx, j.DraftID, j.ActorID)
		if e != nil {
			return e
		}
		for _, src := range j.Input.Sources {
			if seenSources[src.Source.ID] {
				continue
			}
			seenSources[src.Source.ID] = true
			if domain.IsTSCReferenceSource(src.Source.Path) {
				continue
			}
			data, e := d.Artifact(ctx, draft, src.Source.ID, "original")
			if e != nil {
				return e
			}
			o, e := domain.OriginalForTSC(src.Source, data)
			if e != nil {
				return e
			}
			docs = append(docs, o)
			if err := domain.ValidateTSCOriginals(docs); err != nil {
				return err
			}
		}
		return domain.ValidateTSCOriginals(docs)
	})
	if e != nil {
		return out, e
	}
	size := 0
	for _, doc := range docs {
		size += doc.Bytes
	}
	out.Diagnostic = domain.TSCQuestionDiagnostic{Method: "original_pdf_package", QuestionID: q.ID, Documents: len(docs), SelectedBytes: size}
	call := domain.TSCCloudCall{DraftID: j.DraftID, JobID: j.ID, ActorID: j.ActorID, Key: "research:" + j.ActorID + ":" + key, Provider: "gemini", Model: original.OriginalModel(), ReservedPaise: domain.TSCOriginalAllowancePaise, Input: domain.TSCCloudIdentity{DraftVersion: j.DraftVersion, AnswerVersion: version, Question: q, Originals: docs, OriginalDocumentsConfirmed: true}}
	var fresh bool
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
		if e := tscLiveJob(ctx, r, d, j, version); e != nil {
			return e
		}
		if e := r.Authorize(ctx, j.ActorID, "originals.write"); e != nil {
			return e
		}
		var err error
		call, fresh, err = r.ReserveCloud(ctx, call)
		return err
	})
	if e != nil {
		return out, e
	}
	if !fresh {
		if call.State != "ready" {
			return out, domain.CaptureFailure(409, "cloud_outcome_unavailable", "This request may already have been charged. It is not sent again")
		}
		a := call.Answer
		a.Accepted = false
		out.Proposal = &a
		originalProposalPassages(&out, a, j.Input.Passages)
		return out, nil
	}
	a, providerError := original.ProposeOriginals(ctx, q, docs, j.Input.Passages)
	a.Accepted = false
	code := ""
	if providerError != nil {
		code = domain.TSCProviderFailure(providerError)
	} else if domain.ValidateTSCAnswer(a, j.Input) != nil {
		providerError = domain.TSCProviderInvalidProposal
		code = "invalid_provider_evidence"
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
		if e := tscLiveJob(ctx, r, d, j, version); e != nil {
			return e
		}
		if e := r.Authorize(ctx, j.ActorID, "originals.write"); e != nil {
			return e
		}
		return r.CompleteCloud(ctx, call, a, code)
	})
	if e != nil {
		return out, e
	}
	if providerError != nil {
		return out, domain.CaptureFailure(502, code, "Original-document proposal could not be verified. Inspect source pages; no paid retry was made")
	}
	a.Accepted = false
	a = domain.ConciseTSCAnswer(a)
	out.Proposal = &a
	originalProposalPassages(&out, a, j.Input.Passages)
	return out, nil
}

func originalProposalPassages(out *domain.TSCResearch, a domain.TSCAnswer, passages []domain.TSCPassage) {
	for _, c := range a.Citations {
		for _, p := range passages {
			if p.ID == c.SourceID {
				out.Passages = append(out.Passages, p)
				break
			}
		}
	}
}
