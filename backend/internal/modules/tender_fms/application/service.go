package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

type Service struct {
	u ports.UnitOfWork
	a ports.Authorizer
	v ports.Values
}

func New(u ports.UnitOfWork, a ports.Authorizer, v ports.Values) *Service { return &Service{u, a, v} }
func (s *Service) actor(ctx context.Context, t string, write bool) (domain.Principal, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil || !p.Active || p.ID == "" {
		return p, domain.Unauthorized
	}
	required := []string{"tenders.register.view", "tenders.register.read", "tenders.processing.read"}
	if write {
		required = append(required, "tenders.register.write", "tenders.processing.write")
	}
	for _, code := range required {
		if !p.Allows(code) {
			return p, domain.Forbidden
		}
	}
	return p, nil
}
func authorize(p domain.Principal, v domain.Intake, action string) error {
	// Named records have separate metadata/source policy; legacy routes never bypass it.
	if v.RecordKind == "named" {
		return domain.Missing
	}
	if !p.Can(v.OwnerID, "read") {
		return domain.Missing
	}
	if action == "write" && !p.Can(v.OwnerID, "write") {
		return domain.Forbidden
	}
	return nil
}
func (s *Service) List(ctx context.Context, t string) (out []domain.Intake, e error) {
	p, e := s.actor(ctx, t, false)
	if e != nil {
		return out, e
	}
	owner := p.ID
	if p.Allows("tenders.portfolio.read") {
		owner = ""
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, d ports.Documents) error {
		out, e = r.List(ctx, owner)
		if e != nil {
			return e
		}
		for i := range out {
			out[i].Documents, e = d.List(ctx, out[i].ID)
			if e != nil {
				return e
			}
		}
		return nil
	})
	return
}
func (s *Service) Upload(ctx context.Context, t, existing, filename string, data []byte) (out domain.Intake, e error) {
	p, e := s.actor(ctx, t, true)
	if e != nil {
		return out, e
	}
	if existing != "" {
		existing, e = domain.ID(existing)
		if e != nil {
			return out, e
		}
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, d ports.Documents) error {
		if err := r.Lock(ctx); err != nil {
			return err
		}
		name, hash, err := d.Validate(filename, data)
		if err != nil {
			return err
		}
		if existing != "" {
			out, err = r.Get(ctx, existing)
			if err != nil {
				return err
			}
			if err = authorize(p, out, "write"); err != nil {
				return err
			}
		} else {
			found, err := r.FindInitial(ctx, p.ID, hash)
			if err != nil {
				return err
			}
			if found != nil {
				out = *found
			} else {
				ready, e := r.OwnerActive(ctx, p.ID)
				if e != nil {
					return e
				}
				if !ready {
					return domain.Unauthorized
				}
				out, err = r.Create(ctx, s.v.ID(), name, p.ID, hash)
				if err != nil {
					return err
				}
			}
		}
		source, err := d.Submit(ctx, out.ID, p.ID, name, data)
		if err != nil {
			return err
		}
		if err = r.Changed(ctx, out, source.ID, p.ID, "source_attached"); err != nil {
			return err
		}
		out, err = r.Get(ctx, out.ID)
		if err != nil {
			return err
		}
		out.Documents, err = d.List(ctx, out.ID)
		return err
	})
	return
}
func (s *Service) Artifact(ctx context.Context, t, id, doc, kind string) (out domain.Artifact, e error) {
	p, e := s.actor(ctx, t, false)
	if e != nil {
		return out, e
	}
	id, e = domain.ID(id)
	if e != nil {
		return out, e
	}
	doc, e = domain.ID(doc)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, d ports.Documents) error {
		v, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
		if err = authorize(p, v, "read"); err != nil {
			return err
		}
		out, err = d.Artifact(ctx, id, doc, kind)
		if err != nil {
			return err
		}
		return r.Changed(ctx, v, doc, p.ID, "source_"+kind+"_accessed")
	})
	return
}
func (s *Service) Retry(ctx context.Context, t, id, doc string, version int32) (out domain.Source, e error) {
	p, e := s.actor(ctx, t, true)
	if e != nil {
		return out, e
	}
	id, e = domain.ID(id)
	if e != nil {
		return out, e
	}
	doc, e = domain.ID(doc)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, d ports.Documents) error {
		v, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
		if err = authorize(p, v, "write"); err != nil {
			return err
		}
		out, err = d.Retry(ctx, id, doc, version, p.ID)
		if err != nil {
			return err
		}
		return r.Changed(ctx, v, doc, p.ID, "reading_retried")
	})
	return
}
func (s *Service) History(ctx context.Context, t, id string) (out []domain.Event, e error) {
	p, e := s.actor(ctx, t, false)
	if e != nil {
		return out, e
	}
	id, e = domain.ID(id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, d ports.Documents) error {
		v, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
		if err = authorize(p, v, "read"); err != nil {
			return err
		}
		out, err = r.History(ctx, id)
		if err != nil {
			return err
		}
		sources, err := d.List(ctx, id)
		if err != nil {
			return err
		}
		for _, source := range sources {
			events, err := d.History(ctx, id, source.ID)
			if err != nil {
				return err
			}
			out = append(out, events...)
		}
		return nil
	})
	return
}

// AuthorizeUpload rejects unauthorized large requests before body buffering.
func (s *Service) AuthorizeUpload(ctx context.Context, t string) error {
	_, e := s.actor(ctx, t, true)
	return e
}
