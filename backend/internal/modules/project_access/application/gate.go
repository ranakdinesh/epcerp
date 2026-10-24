package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/ports"
)

// Gate is a transaction-bound read service. Auth/Projects retain their facts;
// the gate publishes neither a financial decision nor a workflow state bypass.
type Gate struct {
	r ports.Repository
	c ports.Context
}

func NewGate(r ports.Repository, c ports.Context) *Gate { return &Gate{r, c} }
func live(ctx context.Context, c ports.Context, a domain.Actor) (domain.Actor, *domain.User, error) {
	if !a.Active || a.ID == "" {
		return a, nil, domain.Unauthorized
	}
	u, e := c.User(ctx, a.ID)
	if e != nil {
		return a, nil, e
	}
	if u == nil || !u.Active {
		return a, u, domain.Unauthorized
	}
	return domain.Actor{ID: u.ID, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}, u, nil
}
func (g *Gate) Check(ctx context.Context, a domain.Actor, project, code string, write bool) (domain.Access, error) {
	p, e := g.c.Project(ctx, project, write)
	if e != nil {
		return domain.Access{}, e
	}
	if p == nil {
		return domain.Access{}, domain.Missing
	}
	a, u, e := live(ctx, g.c, a)
	if e != nil {
		return domain.Access{}, e
	}
	if !domain.ScopedCode(code) || !a.Can(code) {
		return domain.Access{}, domain.Forbidden
	}
	allowed := domain.Portfolio(a, write) || (a.ID == p.ManagerID && !domain.FinancialDecision(code))
	if !allowed {
		profiles, e := g.r.List(ctx, project)
		if e != nil {
			return domain.Access{}, e
		}
		for _, profile := range profiles {
			if !profile.Active || profile.UserID != a.ID {
				continue
			}
			b, e := g.c.Bundle(ctx, profile.BundleID)
			if e != nil {
				return domain.Access{}, e
			}
			if b == nil {
				continue
			}
			member, e := g.c.Member(ctx, a.ID, profile.BundleID)
			if e != nil {
				return domain.Access{}, e
			}
			if domain.Effective(a, *u, *b, member, profile, code) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return domain.Access{}, domain.Forbidden
	}
	if write && !domain.Open(*p) {
		return domain.Access{}, domain.Invalid("project", "Project is inactive, held or closed")
	}
	return domain.Access{Project: *p, CanWrite: domain.Open(*p) && !(!write && domain.Portfolio(a, false) && !domain.Portfolio(a, true) && a.ID != p.ManagerID)}, nil
}
func (g *Gate) Owners(ctx context.Context, project string) ([]domain.Owner, error) {
	p, e := g.c.Project(ctx, project, false)
	if e != nil {
		return nil, e
	}
	if p == nil {
		return nil, domain.Missing
	}
	options, e := g.c.Options(ctx)
	if e != nil {
		return nil, e
	}
	profiles, e := g.r.List(ctx, project)
	if e != nil {
		return nil, e
	}
	out := []domain.Owner{}
	for _, u := range options.Users {
		current, e := g.c.User(ctx, u.ID)
		if e != nil {
			return nil, e
		}
		if current == nil {
			continue
		}
		u = *current
		if !u.Active {
			continue
		}
		eligible := u.Superuser || u.ID == p.ManagerID || u.ID == p.ProcurementExecutiveID
		if !eligible {
			for _, profile := range profiles {
				if !profile.Active || profile.UserID != u.ID {
					continue
				}
				b, e := g.c.Bundle(ctx, profile.BundleID)
				if e != nil {
					return nil, e
				}
				if b == nil {
					continue
				}
				member, e := g.c.Member(ctx, u.ID, b.ID)
				if e != nil {
					return nil, e
				}
				a := domain.Actor{ID: u.ID, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}
				for _, code := range profile.Permissions {
					if domain.Effective(a, u, *b, member, profile, code) {
						eligible = true
						break
					}
				}
				if eligible {
					break
				}
			}
		}
		if eligible {
			out = append(out, domain.Owner{ID: u.ID, Name: u.Name})
		}
	}
	return out, nil
}
