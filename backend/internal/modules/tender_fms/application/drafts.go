package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

// Drafts owns unassigned source intake, independently of named capture/provisioning.
type Drafts struct {
	u      ports.DraftUnit
	a      ports.Authorizer
	limits ports.CommandLimits
}

func NewDrafts(u ports.DraftUnit, a ports.Authorizer) *Drafts { return &Drafts{u: u, a: a} }
func (s *Drafts) Create(ctx context.Context, token, key, name string) (out domain.DocumentDraft, e error) {
	actor, e := s.subject(ctx, token)
	if e != nil {
		return out, e
	}
	if e = s.admit(actor, true); e != nil {
		return out, e
	}
	if _, e = domain.ID(key); e != nil {
		return out, e
	}
	name, e = domain.DraftName(name)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.DraftRepository) error {
		if e := r.Authorize(ctx, actor, true); e != nil {
			return e
		}
		var e error
		out, e = r.Create(ctx, actor, key, name)
		return e
	})
	return
}
func (s *Drafts) List(ctx context.Context, token string, q domain.DraftQuery) (out []domain.DocumentDraft, total int64, e error) {
	actor, e := s.subject(ctx, token)
	if e != nil {
		return nil, 0, e
	}
	if e = s.admit(actor, false); e != nil {
		return nil, 0, e
	}
	if q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || len(q.Search) > 200 {
		return nil, 0, domain.Invalid("Invalid page")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.DraftRepository) error {
		if e := r.Authorize(ctx, actor, false); e != nil {
			return e
		}
		var e error
		out, total, e = r.List(ctx, actor, q)
		return e
	})
	return
}
func (s *Drafts) Within(ctx context.Context, token, id string, write bool, f func(context.Context, ports.DraftRepository, domain.DocumentDraft) error) error {
	actor, e := s.subject(ctx, token)
	if e != nil {
		return e
	}
	if e = s.admit(actor, write); e != nil {
		return e
	}
	if _, e = domain.ID(id); e != nil {
		return e
	}
	return s.u.Within(ctx, func(ctx context.Context, r ports.DraftRepository) error {
		if e := r.Authorize(ctx, actor, write); e != nil {
			return e
		}
		d, e := r.Get(ctx, id, actor)
		if e != nil {
			return e
		}
		return f(ctx, r, d)
	})
}
func (s *Drafts) Get(ctx context.Context, token, id string) (out domain.DocumentDraft, e error) {
	e = s.Within(ctx, token, id, false, func(ctx context.Context, r ports.DraftRepository, d domain.DocumentDraft) error {
		out = d
		return r.Populate(ctx, &out)
	})
	return
}
func (s *Drafts) Attach(ctx context.Context, token, id, receipt string) error {
	if _, e := domain.ID(receipt); e != nil {
		return e
	}
	return s.Within(ctx, token, id, true, func(ctx context.Context, r ports.DraftRepository, d domain.DocumentDraft) error {
		return r.Attach(ctx, d, receipt, d.OwnerID)
	})
}
func (s *Drafts) Artifact(ctx context.Context, token, id, document, kind string) (out []byte, e error) {
	if _, e = domain.ID(document); e != nil {
		return nil, e
	}
	if kind != "original" && kind != "text" && kind != "result" && kind != "history" {
		return nil, domain.Missing
	}
	e = s.Within(ctx, token, id, false, func(ctx context.Context, r ports.DraftRepository, d domain.DocumentDraft) error {
		var e error
		out, e = r.Artifact(ctx, d, document, kind)
		return e
	})
	return
}
func (s *Drafts) Retry(ctx context.Context, token, id, document string, version int32) error {
	if _, e := domain.ID(document); e != nil {
		return e
	}
	return s.Within(ctx, token, id, true, func(ctx context.Context, r ports.DraftRepository, d domain.DocumentDraft) error {
		return r.Retry(ctx, d, document, version)
	})
}

func (s *Drafts) subject(ctx context.Context, token string) (string, error) {
	p, e := s.a.Resolve(ctx, token)
	if e != nil {
		return "", e
	}
	if !p.Active || p.ID == "" {
		return "", domain.Unauthorized
	}
	return p.ID, nil
}

func (s *Drafts) WithLimits(l ports.CommandLimits) *Drafts { s.limits = l; return s }
func (s *Drafts) admit(actor string, write bool) error {
	op := "read"
	if write {
		op = "capture"
	}
	if s.limits != nil && !s.limits.Allow(actor, op) {
		return domain.CaptureFailure(429, "rate_limited", "Too many document draft requests; wait and retry")
	}
	return nil
}
