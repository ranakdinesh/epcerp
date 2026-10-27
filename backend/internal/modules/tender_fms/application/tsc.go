package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

// TSC owns draft answers only. It cannot approve eligibility or a bid.
type TSC struct {
	unit      ports.TSCUnit
	auth      ports.Authorizer
	provider  ports.TSCProvider
	exporter  ports.TSCExporter
	limits    ports.CommandLimits
	providers map[string]ports.TSCProvider
}

func NewTSC(u ports.TSCUnit, a ports.Authorizer, p ports.TSCProvider, e ports.TSCExporter) *TSC {
	return &TSC{unit: u, auth: a, provider: p, exporter: e}
}
func (s *TSC) WithLimits(l ports.CommandLimits) *TSC { s.limits = l; return s }
func (s *TSC) within(ctx context.Context, token, id, action string, f func(context.Context, ports.TSCRepository, domain.DocumentDraft, string) error) error {
	p, e := s.auth.Resolve(ctx, token)
	if e != nil {
		return e
	}
	if !p.Active || p.ID == "" {
		return domain.Unauthorized
	}
	if _, e = domain.ID(id); e != nil {
		return e
	}
	if s.limits != nil && !s.limits.Allow(p.ID, "tsc_"+action) {
		return domain.CaptureFailure(429, "rate_limited", "Too many TSC requests")
	}
	return s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
		if e := d.Authorize(ctx, p.ID, action == "write"); e != nil {
			return e
		}
		if e := r.Authorize(ctx, p.ID, action); e != nil {
			return e
		}
		draft, e := d.Get(ctx, id, p.ID)
		if e != nil {
			return e
		}
		return f(ctx, r, draft, p.ID)
	})
}
func (s *TSC) Create(ctx context.Context, token, id, key, mode string, version int64) (out domain.TSCJob, e error) {
	if _, e = domain.ID(key); e != nil {
		return out, e
	}
	if mode != "manual" && mode != "ollama" && mode != "gemini" && mode != "openai" {
		return out, domain.Invalid("Choose an available draft preparation provider")
	}
	model := ""
	provider := s.providerFor(mode)
	if mode != "manual" && provider != nil {
		model = provider.Model()
	}
	e = s.within(ctx, token, id, "write", func(ctx context.Context, r ports.TSCRepository, d domain.DocumentDraft, actor string) error {
		old, e := r.FindOperation(ctx, d.ID, actor, key)
		if e != nil {
			return e
		}
		if old != nil {
			if old.Mode != mode || old.DraftVersion != version {
				return domain.CaptureFailure(409, "idempotency_conflict", "Operation key belongs to another generation request")
			}
			out = *old
			return nil
		}
		if mode != "manual" && model == "" {
			return domain.CaptureFailure(503, "provider_not_configured", "Selected provider is not enabled and configured; use a manual draft")
		}
		if d.Version != version {
			return domain.CaptureFailure(409, "stale_source", "Tender sources changed; refresh before generation")
		}
		in, e := r.Snapshot(ctx, d)
		if e != nil {
			return e
		}
		if mode != "manual" {
			in.ProposalVersion = domain.TSCProposalVersion
			if domain.TSCCloudAllowance(mode) > 0 {
				in.ProposalVersion += "/" + domain.TSCCloudProposalVersion
			}
		}
		out, e = r.Create(ctx, d, actor, key, mode, model, in)
		return e
	})
	return
}
func (s *TSC) List(ctx context.Context, token, id string) (out []domain.TSCJob, e error) {
	e = s.within(ctx, token, id, "read", func(ctx context.Context, r ports.TSCRepository, d domain.DocumentDraft, _ string) error {
		var e error
		out, e = r.List(ctx, d.ID)
		return e
	})
	return
}
func (s *TSC) Get(ctx context.Context, token, id, job string) (out domain.TSCJob, e error) {
	if _, e = domain.ID(job); e != nil {
		return out, e
	}
	e = s.within(ctx, token, id, "read", func(ctx context.Context, r ports.TSCRepository, d domain.DocumentDraft, _ string) error {
		var e error
		out, e = r.Get(ctx, d.ID, job)
		out.Stale = out.DraftVersion != d.Version
		return e
	})
	return
}
func (s *TSC) Edit(ctx context.Context, token, id, job string, version int64, a domain.TSCAnswer, reason string) (out domain.TSCJob, e error) {
	if _, e = domain.ID(job); e != nil {
		return out, e
	}
	if (len(reason) > 0 && len(reason) < 3) || len(reason) > 1000 {
		return out, domain.Invalid("Provide a correction reason of 3 to 1000 bytes")
	}
	e = s.within(ctx, token, id, "write", func(ctx context.Context, r ports.TSCRepository, d domain.DocumentDraft, actor string) error {
		current, e := r.Get(ctx, id, job)
		if e != nil {
			return e
		}
		if current.State != "ready" || current.DraftVersion != d.Version {
			return domain.CaptureFailure(409, "stale_source", "Generate a new draft from current sources before editing")
		}
		var previous domain.TSCAnswer
		for _, answer := range current.Result.Answers {
			if answer.QuestionID == a.QuestionID {
				previous = answer
				break
			}
		}
		if previous.Accepted && !domain.SameTSCAnswerContent(previous, a) {
			return domain.CaptureFailure(409, "answer_accepted", "Uncheck Accepted before changing this answer")
		}
		a.Origin = "manual"
		if domain.SameTSCAnswerContent(previous, a) {
			a.Origin = previous.Origin
		}
		if reason == "" {
			reason = "Reviewer updated checklist answer"
			if a.Accepted {
				reason = "Reviewer accepted checklist answer"
			} else if previous.Accepted {
				reason = "Reviewer reopened checklist answer"
			}
		}
		if e := domain.ValidateTSCAnswer(a, current.Input); e != nil {
			return e
		}
		out, e = r.Edit(ctx, current, version, a, reason, actor)
		return e
	})
	return
}
func (s *TSC) Export(ctx context.Context, token, id, job string) (out []byte, e error) {
	if _, e = domain.ID(job); e != nil {
		return nil, e
	}
	var snapshot domain.TSCJob
	e = s.within(ctx, token, id, "export", func(ctx context.Context, r ports.TSCRepository, d domain.DocumentDraft, actor string) error {
		var e error
		snapshot, e = r.Get(ctx, id, job)
		if e != nil {
			return e
		}
		if snapshot.State != "ready" || snapshot.DraftVersion != d.Version {
			return domain.CaptureFailure(409, "stale_source", "Only a ready draft matching current source versions can be exported")
		}
		return r.ExportRequested(ctx, snapshot, actor)
	})
	if e != nil {
		return nil, e
	}
	return s.exporter.Export(snapshot)
}

func (s *TSC) ExportEvidence(ctx context.Context, token, id, job string) (out []byte, e error) {
	if _, e = domain.ID(job); e != nil {
		return nil, e
	}
	e = s.within(ctx, token, id, "export", func(ctx context.Context, r ports.TSCRepository, d domain.DocumentDraft, actor string) error {
		snapshot, err := r.Get(ctx, id, job)
		if err != nil {
			return err
		}
		if snapshot.State != "ready" || snapshot.DraftVersion != d.Version {
			return domain.CaptureFailure(409, "stale_source", "Only a ready snapshot matching current sources can be exported")
		}
		if err = r.ExportRequested(ctx, snapshot, actor); err != nil {
			return err
		}
		out = domain.EvidenceMarkdown(snapshot.Input)
		return nil
	})
	return
}

// Once claims durably, releases the transaction, calls the provider, then fences
// completion on the same claim and current source/permission state.
func (s *TSC) Once(ctx context.Context) (worked bool, e error) {
	var job *domain.TSCJob
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, _ ports.DraftRepository) error {
		var e error
		job, e = r.Claim(ctx)
		return e
	})
	if e != nil || job == nil {
		return false, e
	}
	worked = true
	result := domain.TSCResult{Schema: domain.TSCVersion, Answers: []domain.TSCAnswer{}, Warnings: job.Input.Warnings}
	code := ""
	provider := s.providerFor(job.Mode)
	if job.Mode != "manual" && (provider == nil || job.Model != provider.Model()) {
		code = "configured_model_changed"
	}
	candidates := domain.CollectTSCCandidates(job.Input)
	for _, q := range job.Input.Questions {
		if code != "" {
			break
		}
		e = s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
			if e := d.Authorize(ctx, job.ActorID, true); e != nil {
				return e
			}
			if e := r.Authorize(ctx, job.ActorID, "write"); e != nil {
				return e
			}
			current, e := d.Get(ctx, job.DraftID, job.ActorID)
			if e != nil {
				return e
			}
			if current.Version != job.DraftVersion {
				return domain.Forbidden
			}
			return r.Renew(ctx, *job)
		})
		if e != nil {
			code = "source_or_access_changed"
			break
		}
		a := domain.EmptyTSCAnswer(q)
		diagnostic := domain.TSCSearchDiagnostic(q, job.Input.Passages)
		candidate, hasCandidate := candidates[q.ID]
		if job.Mode != "manual" && hasCandidate {
			a = candidate
		}
		if job.Mode != "manual" && !hasCandidate && !domain.TSCRequiresHumanReview(q) {
			if domain.TSCCloudAllowance(job.Mode) > 0 {
				a, e = s.cloudPropose(ctx, *job, q, job.Mode, "generation:"+job.ID+":"+q.ID, 0)
			} else {
				a, e = provider.Propose(ctx, q, job.Input.Passages)
			}
			if e != nil {
				if domain.TSCCloudAllowance(job.Mode) > 0 {
					a = domain.EmptyTSCAnswer(q)
					a.Origin = "cloud_review_required"
					a.Text = ""
					result.Warnings = append(result.Warnings, q.ID+": cloud request not completed; no automatic paid retry")
					result.Answers = append(result.Answers, a)
					continue
				}
				if errors.Is(e, domain.TSCProviderInvalidProposal) {
					a = domain.EmptyTSCAnswer(q)
					a.Status = "needs_review"
					a.Text = ""
					a.Origin = "rejected_ollama_proposal"
					result.Warnings = append(result.Warnings, q.ID+": invalid model proposal discarded; source-linked review required")
				} else {
					code = domain.TSCProviderFailure(e)
					break
				}
			}
			a.QuestionID = q.ID
			a.Accepted = false
			if job.Mode == "ollama" && a.Origin != "rejected_ollama_proposal" {
				a.Origin = "ollama_proposal"
			}
			if e = domain.ValidateTSCAnswer(a, job.Input); e != nil {
				code = "invalid_provider_evidence"
				break
			}
		}
		a = domain.ConciseTSCAnswer(a)
		diagnostic.Outcome = a.Origin + ":" + a.Status
		result.Diagnostics = append(result.Diagnostics, diagnostic)
		result.Answers = append(result.Answers, a)
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
		if code == "" {
			if e := d.Authorize(ctx, job.ActorID, true); e != nil {
				code = "source_or_access_changed"
			}
			if e := r.Authorize(ctx, job.ActorID, "write"); e != nil {
				code = "source_or_access_changed"
			}
			current, e := d.Get(ctx, job.DraftID, job.ActorID)
			if e != nil || current.Version != job.DraftVersion {
				code = "source_or_access_changed"
			}
		}
		return r.Finish(ctx, *job, result, code)
	})
	return
}
