package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/ports"
)

type Service struct {
	u ports.UnitOfWork
	a ports.Authorizer
	v ports.Values
}

func New(u ports.UnitOfWork, a ports.Authorizer, v ports.Values) *Service { return &Service{u, a, v} }
func (s *Service) actor(ctx context.Context, t string) (domain.Actor, error) {
	a, e := s.a.Resolve(ctx, t)
	if e != nil {
		return a, e
	}
	if !a.Can("projects.team.read") {
		if !a.Active {
			return a, domain.Unauthorized
		}
		return a, domain.Forbidden
	}
	return a, nil
}
func workspace(ctx context.Context, r ports.Repository, c ports.Context, a domain.Actor, p *domain.Project) (domain.Workspace, error) {
	a, _, e := live(ctx, c, a)
	if e != nil {
		return domain.Workspace{}, e
	}
	profiles, e := r.List(ctx, p.ID)
	if e != nil {
		return domain.Workspace{}, e
	}
	for n := range profiles {
		u, e := c.User(ctx, profiles[n].UserID)
		if e != nil {
			return domain.Workspace{}, e
		}
		if u != nil {
			profiles[n].UserName = u.Name
		}
		b, e := c.Bundle(ctx, profiles[n].BundleID)
		if e != nil {
			return domain.Workspace{}, e
		}
		if b != nil {
			profiles[n].BundleName = b.Name
		}
	}
	return domain.Workspace{Project: *p, Profiles: profiles, CanWrite: domain.Manage(a, *p)}, e
}
func (s *Service) Workspace(ctx context.Context, t, p string) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	if p, e = domain.ID(p); e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		access, e := NewGate(r, c).Check(ctx, a, p, "projects.team.read", false)
		if e != nil {
			return e
		}
		out, e = workspace(ctx, r, c, a, &access.Project)
		return e
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}
func (s *Service) Options(ctx context.Context, t, p string) (out domain.Options, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	if p, e = domain.ID(p); e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		source, e := c.Project(ctx, p, false)
		if e != nil {
			return e
		}
		if source == nil {
			return domain.Missing
		}
		a, _, e = live(ctx, c, a)
		if e != nil {
			return e
		}
		if !domain.Manage(a, *source) {
			return domain.Forbidden
		}
		out, e = c.Options(ctx)
		return e
	})
	if e != nil {
		out = domain.Options{}
	}
	return
}
func (s *Service) Save(ctx context.Context, t, p, id string, input domain.Input) (out domain.Profile, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	if p, e = domain.ID(p); e != nil {
		return out, e
	}
	if id != "" {
		if id, e = domain.ID(id); e != nil {
			return out, e
		}
	}
	input, e = input.Validate()
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		project, e := c.Project(ctx, p, true)
		if e != nil {
			return e
		}
		if project == nil {
			return domain.Missing
		}
		a, _, e = live(ctx, c, a)
		if e != nil {
			return e
		}
		if !domain.Manage(a, *project) {
			return domain.Forbidden
		}
		var before *domain.Profile
		if id != "" {
			before, e = r.Get(ctx, p, id)
			if e != nil {
				return e
			}
			if before == nil {
				return domain.Missing
			}
		}
		if before == nil {
			profiles, e := r.List(ctx, p)
			if e != nil {
				return e
			}
			if len(profiles) >= 1000 {
				return domain.Invalid("profiles", "Project responsibility limit reached")
			}
		}
		if before == nil && input.Version != 0 {
			return domain.Stale
		}
		if before != nil && (before.Version != input.Version || before.UserID != input.UserID || before.BundleID != input.BundleID) {
			return domain.Stale
		}
		if input.Active {
			if !domain.Open(*project) {
				return domain.Invalid("project", "Assign responsibilities only in an active open project")
			}
			u, e := c.User(ctx, input.UserID)
			if e != nil {
				return e
			}
			b, e := c.Bundle(ctx, input.BundleID)
			if e != nil {
				return e
			}
			member, e := c.Member(ctx, input.UserID, input.BundleID)
			if e != nil {
				return e
			}
			if e = domain.ValidateEligibility(input, u, b, member); e != nil {
				return e
			}
		} else if before == nil {
			return domain.Invalid("is_active", "Create an active responsibility before revoking it")
		}
		record := domain.Profile{ID: id, ProjectID: p, UserID: input.UserID, BundleID: input.BundleID, Responsibility: input.Responsibility, Permissions: input.Permissions, Active: input.Active, Version: input.Version + 1}
		action := "revise"
		if before == nil {
			record.ID = s.v.ID()
			action = "assign"
		} else {
			v := domain.Copy(*before)
			before = &v
		}
		if !input.Active {
			action = "revoke"
		}
		out, e = r.Save(ctx, record, a.ID)
		if e != nil {
			return e
		}
		return r.Event(ctx, domain.Event{ID: s.v.ID(), ProfileID: out.ID, ProjectID: p, Action: action, Reason: input.Reason, Before: before, After: out, ActorID: a.ID, At: out.UpdatedAt})
	})
	if e != nil {
		out = domain.Profile{}
	}
	return
}
func (s *Service) History(ctx context.Context, t, p, id string) (out []domain.Event, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return nil, e
	}
	if p, e = domain.ID(p); e != nil {
		return nil, e
	}
	if id, e = domain.ID(id); e != nil {
		return nil, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		if _, e := NewGate(r, c).Check(ctx, a, p, "projects.team.read", false); e != nil {
			return e
		}
		record, e := r.Get(ctx, p, id)
		if e != nil {
			return e
		}
		if record == nil {
			return domain.Missing
		}
		out, e = r.History(ctx, p, id)
		return e
	})
	if e != nil {
		out = nil
	}
	return
}
