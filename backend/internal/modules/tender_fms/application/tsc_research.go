package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

func (s *TSC) WithProviders(p map[string]ports.TSCProvider) *TSC { s.providers = p; return s }
func (s *TSC) providerFor(mode string) ports.TSCProvider {
	if mode == "ollama" {
		return s.provider
	}
	return s.providers[mode]
}

func (s *TSC) Options(ctx context.Context, token, id string) (out domain.TSCOptions, e error) {
	out.LimitPaise = domain.TSCCloudBudgetPaise
	e = s.within(ctx, token, id, "read", func(ctx context.Context, r ports.TSCRepository, _ domain.DocumentDraft, _ string) error {
		var e error
		out.ReservedPaise, e = r.CloudBudget(ctx, id)
		if e != nil {
			return e
		}
		out.Providers = []domain.TSCProviderOption{{Mode: "manual", Available: true}}
		for _, mode := range []string{"gemini", "openai", "ollama"} {
			p := s.providerFor(mode)
			model := ""
			if p != nil {
				model = p.Model()
			}
			out.Providers = append(out.Providers, domain.TSCProviderOption{Mode: mode, Model: model, Available: model != "", Cloud: domain.TSCCloudAllowance(mode) > 0, AllowancePaise: domain.TSCCloudAllowance(mode)})
		}
		return nil
	})
	return
}

func (s *TSC) Research(ctx context.Context, token, id, jobID, question, query, mode, key string, version int64) (out domain.TSCResearch, e error) {
	if len(query) > 300 {
		return out, domain.Invalid("Search terms must be at most 300 bytes")
	}
	if _, e = domain.ID(jobID); e != nil {
		return out, e
	}
	if mode != "search" && mode != "gemini" && mode != "openai" && mode != "ollama" {
		return out, domain.Invalid("Choose source search or a configured proposal provider")
	}
	action := "read"
	if mode != "search" {
		action = "write"
		if _, e = domain.ID(key); e != nil {
			return out, e
		}
	}
	var snapshot domain.TSCJob
	var q domain.TSCQuestion
	e = s.within(ctx, token, id, action, func(ctx context.Context, r ports.TSCRepository, d domain.DocumentDraft, actor string) error {
		var e error
		snapshot, e = r.Get(ctx, id, jobID)
		if e != nil {
			return e
		}
		if snapshot.State != "ready" || snapshot.DraftVersion != d.Version || snapshot.Version != version {
			return domain.CaptureFailure(409, "stale_source", "Refresh the current TSC before researching this question")
		}
		snapshot.ActorID = actor
		for _, candidate := range snapshot.Input.Questions {
			if candidate.ID == question {
				q = candidate
			}
		}
		if q.ID == "" {
			return domain.Invalid("Unknown TSC question")
		}
		if mode != "search" {
			for _, a := range snapshot.Result.Answers {
				if a.QuestionID == question && a.Accepted {
					return domain.CaptureFailure(409, "answer_accepted", "Uncheck Accepted before requesting another proposal")
				}
			}
		}
		return nil
	})
	if e != nil {
		return out, e
	}
	q = domain.TSCResearchQuestion(q, query)
	out.QuestionID = question
	out.Passages = domain.RetrieveTSCEvidence(q, snapshot.Input.Passages)
	out.Warning = "Search covers the retained reading snapshot. Missing excerpts do not establish absence; inspect original pages, referenced documents and conflicting clauses."
	if mode == "search" {
		return out, nil
	}
	if domain.TSCRequiresHumanReview(q) {
		a := domain.EmptyTSCAnswer(q)
		out.Proposal = &a
		return out, nil
	}
	provider := s.providerFor(mode)
	if provider == nil || provider.Model() == "" {
		return out, domain.CaptureFailure(503, "provider_not_configured", "Selected provider is not enabled and configured")
	}
	var answer domain.TSCAnswer
	if domain.TSCCloudAllowance(mode) > 0 {
		answer, e = s.cloudPropose(ctx, snapshot, q, mode, "research:"+snapshot.ActorID+":"+key, version)
	} else {
		answer, e = provider.Propose(ctx, q, snapshot.Input.Passages)
	}
	if e != nil {
		return out, e
	}
	// Re-authorise and compare current answer and source versions after network I/O.
	e = s.within(ctx, token, id, "write", func(ctx context.Context, r ports.TSCRepository, d domain.DocumentDraft, _ string) error {
		j, e := r.Get(ctx, id, jobID)
		if e != nil {
			return e
		}
		if j.Version != version || d.Version != snapshot.DraftVersion {
			return domain.CaptureFailure(409, "stale_source", "TSC or source changed while researching; refresh before using the proposal")
		}
		return nil
	})
	if e != nil {
		return out, e
	}
	answer.Accepted = false
	answer = domain.ConciseTSCAnswer(answer)
	if e = domain.ValidateTSCAnswer(answer, snapshot.Input); e != nil {
		return out, e
	}
	out.Proposal = &answer
	return
}

func (s *TSC) cloudPropose(ctx context.Context, j domain.TSCJob, q domain.TSCQuestion, mode, key string, expectedVersion int64) (domain.TSCAnswer, error) {
	fallback := domain.EmptyTSCAnswer(q)
	p := s.providerFor(mode)
	if p == nil || p.Model() == "" {
		return fallback, domain.CaptureFailure(503, "provider_not_configured", "Cloud provider is not configured")
	}
	selected := domain.RetrieveTSCEvidence(q, j.Input.Passages)
	if len(selected) == 0 {
		return fallback, nil
	}
	call := domain.TSCCloudCall{DraftID: j.DraftID, JobID: j.ID, ActorID: j.ActorID, Key: key, Input: domain.TSCCloudIdentity{DraftVersion: j.DraftVersion, AnswerVersion: expectedVersion, Question: q, Passages: selected}, Provider: mode, Model: p.Model(), ReservedPaise: domain.TSCCloudAllowance(mode)}
	var fresh bool
	e := s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
		if e := tscLiveJob(ctx, r, d, j, expectedVersion); e != nil {
			return e
		}
		var e error
		call, fresh, e = r.ReserveCloud(ctx, call)
		return e
	})
	if e != nil {
		return fallback, e
	}
	if !fresh {
		if call.State == "ready" {
			return call.Answer, nil
		}
		return fallback, domain.CaptureFailure(409, "cloud_outcome_unavailable", "This request may already have been charged. Its allowance is retained; use source search or start a deliberate new request")
	}
	answer, providerError := p.Propose(ctx, q, j.Input.Passages)
	code := ""
	if providerError != nil {
		code = domain.TSCProviderFailure(providerError)
	} else if domain.ValidateTSCAnswer(answer, j.Input) != nil {
		providerError = domain.TSCProviderInvalidProposal
		code = "invalid_provider_evidence"
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
		if e := tscLiveJob(ctx, r, d, j, expectedVersion); e != nil {
			return e
		}
		return r.CompleteCloud(ctx, call, answer, code)
	})
	if e != nil {
		return fallback, e
	}
	if providerError != nil {
		return fallback, domain.CaptureFailure(502, code, "Provider did not return a usable source-linked answer. The request allowance is retained; there is no automatic paid retry")
	}
	return answer, nil
}

func tscLiveJob(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository, j domain.TSCJob, expectedVersion int64) error {
	if e := d.Authorize(ctx, j.ActorID, true); e != nil {
		return e
	}
	if e := r.Authorize(ctx, j.ActorID, "write"); e != nil {
		return e
	}
	current, e := d.Get(ctx, j.DraftID, j.ActorID)
	if e != nil {
		return e
	}
	if current.Version != j.DraftVersion {
		return domain.Forbidden
	}
	if expectedVersion > 0 {
		now, e := r.Get(ctx, j.DraftID, j.ID)
		if e != nil {
			return e
		}
		if now.Version != expectedVersion || now.State != "ready" {
			return domain.CaptureFailure(409, "tsc_state_changed", "TSC changed during research")
		}
	}
	return nil
}
