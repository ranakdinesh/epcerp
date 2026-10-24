package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/ports"
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
	if !a.Active || a.ID == "" {
		return a, domain.Unauthorized
	}
	if !a.Can("projects.budgets.read") {
		return a, domain.Forbidden
	}
	return a, nil
}
func (s *Service) workspace(ctx context.Context, r ports.Repository, c ports.Context, finance ports.Approvals, a domain.Actor, p string) (domain.Workspace, error) {
	scope, e := c.Source(ctx, p)
	if e != nil {
		return domain.Workspace{}, e
	}
	record, e := r.Get(ctx, p)
	if e != nil {
		return domain.Workspace{}, e
	}
	caps, e := c.Capabilities(ctx, a, p)
	if e != nil {
		return domain.Workspace{}, e
	}
	out := domain.Workspace{Record: record, Project: scope.Project, Owners: scope.Owners, Items: scope.Items, CanWrite: caps.CanWrite && scope.Project.Available && a.Can("projects.budgets.write"), CanApprove: caps.CanApprove && scope.Project.Available && a.Can("projects.budgets.approve") && finance != nil, History: []domain.Event{}}
	if record != nil {
		report, e := domain.BuildReport(record.Data)
		if e != nil {
			return out, e
		}
		out.Report = &report
		if record.Approved != nil {
			report, e := domain.BuildReport(record.Approved.Data)
			if e != nil {
				return out, e
			}
			out.ApprovedReport = &report
		}
	}
	out.History, e = r.History(ctx, p)
	return out, e
}
func (s *Service) Workspace(ctx context.Context, t, p string) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	if p, e = domain.ID(p); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context, f ports.Approvals) error {
		_, e := c.Scope(ctx, a, p, "projects.budgets.read", false)
		if e != nil {
			return e
		}
		out, e = s.workspace(ctx, r, c, f, a, p)
		return e
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}
func (s *Service) Execute(ctx context.Context, t, p string, input domain.Command) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	if p, e = domain.ID(p); e != nil {
		return
	}
	input, e = input.Validate()
	if e != nil {
		return
	}
	permission := "projects.budgets.write"
	if input.Action == "approve" {
		permission = "projects.budgets.approve"
	}
	if !a.Can(permission) {
		return out, domain.Forbidden
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context, finance ports.Approvals) error {
		write, e := c.Scope(ctx, a, p, permission, true)
		if e != nil {
			return e
		}
		if !write {
			return domain.Forbidden
		}
		scope, e := c.Source(ctx, p)
		if e != nil {
			return e
		}
		if !scope.Project.Available {
			return domain.Invalid("project", "Project is inactive, held or closed")
		}
		record, e := r.Get(ctx, p)
		if e != nil {
			return e
		}
		var before *domain.Budget
		var previous *domain.Data
		if record != nil {
			copy := domain.Copy(*record)
			before = &copy
			data := domain.CopyData(record.Data)
			previous = &data
			if !record.Active {
				return domain.Missing
			}
		} else {
			record = &domain.Budget{ProjectID: p, Revision: 1, State: "draft", Active: true, Data: domain.Data{Heads: []domain.Head{}, Allocations: []domain.Allocation{}}}
		}
		if record.Version != input.Version {
			return domain.Stale
		}
		if input.Action == "revise" {
			if record.State != "approved" || record.Approved == nil {
				return domain.Invalid("state", "Only an approved budget can start a revision")
			}
			record.Revision++
			record.State = "draft"
		} else {
			if record.State != "draft" {
				return domain.Invalid("state", "Start a revision before changing an approved budget")
			}
			if scope.Project.ContractValue == nil {
				return domain.Invalid("contract_value", "Ask an administrator to enter the total project value first")
			}
			data := domain.CopyData(record.Data)
			if input.Action == "save" {
				data = domain.CopyData(*input.Data)
			}
			if input.Action == "price_items" {
				ids := map[string]string{}
				for _, item := range scope.Items {
					ids[item.Key()] = s.v.ID()
				}
				data, e = domain.PriceItems(previous, input.Prices, scope.Items, a.ID, *scope.Project.ContractValue, scope.Project.Currency, s.v.ID(), ids)
				if e != nil {
					return e
				}
			}
			total, e := domain.Money(data.Total)
			if e != nil {
				return domain.Invalid("data", "Save a valid budget draft first")
			}
			value, e := domain.Money(*scope.Project.ContractValue)
			if e != nil {
				return e
			}
			if total > value {
				return domain.Invalid("total", "Project budget cannot exceed the total project value")
			}
			data, e = domain.Validate(data, scope, previous)
			if e != nil {
				return e
			}
			if input.Action == "approve" {
				if total <= 0 || !activeHead(data) {
					return domain.Invalid("data", "Approve a positive project budget with at least one head")
				}
				if finance == nil {
					return domain.Invalid("approval", "Finance approval and Director notification are not available")
				}
				staged := domain.Copy(*record)
				staged.Data = data
				decision, e := finance.DecideBudget(ctx, a, staged, input.Reason)
				if e != nil {
					return e
				}
				if _, e = domain.ID(decision); e != nil {
					return e
				}
				record.State = "approved"
				record.Approved = &domain.Approved{Revision: record.Revision, Data: domain.CopyData(data), ApprovedBy: a.ID, ApprovedAt: s.v.Now(), DecisionID: decision}
			} else {
				data = domain.Attribute(data, previous, a.ID, s.v.Now())
			}
			record.Data = data
		}
		record.Version++
		saved, e := r.Save(ctx, *record, a.ID)
		if e != nil {
			return e
		}
		if e = r.Event(ctx, domain.Event{ID: s.v.ID(), ProjectID: p, Action: input.Action, Reason: input.Reason, Before: before, After: saved, ActorID: a.ID, CreatedAt: saved.UpdatedAt}); e != nil {
			return e
		}
		out, e = s.workspace(ctx, r, c, finance, a, p)
		return e
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}
func activeHead(d domain.Data) bool {
	for _, h := range d.Heads {
		if h.Active {
			return true
		}
	}
	return false
}
