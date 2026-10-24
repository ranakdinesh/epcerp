package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type ClientRulesService struct {
	u ports.ClientRulesUnitOfWork
	a ports.ClientRulesAuthorizer
	v ports.ClientRulesValues
}

func NewClientRules(u ports.ClientRulesUnitOfWork, a ports.ClientRulesAuthorizer, v ports.ClientRulesValues) *ClientRulesService {
	return &ClientRulesService{u, a, v}
}
func (s *ClientRulesService) actor(ctx context.Context, t, permission string) (domain.Principal, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, domain.ClientRulesUnauthorized
	}
	if !p.Can(permission) {
		return p, domain.ClientRulesForbidden
	}
	return p, nil
}
func (s *ClientRulesService) List(ctx context.Context, t, client string) (out []domain.ClientRulesRecord, e error) {
	if _, e = s.actor(ctx, t, "settings.purchase_rules.read"); e != nil {
		return
	}
	if client, e = domain.ClientRulesID(client); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.ClientRulesRepository, _ ports.ClientRulesContext) error {
		out, e = r.List(ctx, client)
		return e
	})
	return
}
func (s *ClientRulesService) History(ctx context.Context, t, client, id string) (out []domain.ClientRulesEvent, e error) {
	if _, e = s.actor(ctx, t, "settings.purchase_rules.read"); e != nil {
		return
	}
	if client, e = domain.ClientRulesID(client); e != nil {
		return
	}
	if id, e = domain.ClientRulesID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.ClientRulesRepository, _ ports.ClientRulesContext) error {
		v, e := r.Get(ctx, client, id)
		if e != nil {
			return e
		}
		if v == nil {
			return domain.ClientRulesMissing
		}
		out, e = r.History(ctx, client, id)
		return e
	})
	return
}
func (s *ClientRulesService) Save(ctx context.Context, t, client, id string, input domain.ClientRulesSave) (out domain.ClientRulesRecord, e error) {
	p, e := s.actor(ctx, t, "settings.purchase_rules.write")
	if e != nil {
		return out, e
	}
	if client, e = domain.ClientRulesID(client); e != nil {
		return
	}
	if id != "" {
		if id, e = domain.ClientRulesID(id); e != nil {
			return
		}
	}
	input, e = input.Validate()
	if e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.ClientRulesRepository, c ports.ClientRulesContext) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		if e := c.Validate(ctx, client, input.Data); e != nil {
			return e
		}
		action := "create"
		var before *domain.ClientRulesRecord
		record := domain.ClientRulesRecord{ID: id, ClientID: client, Data: input.Data, Active: input.Active, Version: 1}
		if id == "" {
			if input.Version != 1 {
				return domain.ClientRulesStale
			}
			record.ID = s.v.ID()
		} else {
			before, e = r.Get(ctx, client, id)
			if e != nil {
				return e
			}
			if before == nil {
				return domain.ClientRulesMissing
			}
			if before.Version != input.Version {
				return domain.ClientRulesStale
			}
			record.Version = before.Version + 1
			action = "revise"
		}
		out, e = r.Save(ctx, record, p.ID)
		if e != nil {
			return e
		}
		return r.Event(ctx, domain.ClientRulesEvent{ID: s.v.ID(), SubjectID: out.ID, Action: action, Reason: input.Reason, Before: before, After: out, ActorID: p.ID, CreatedAt: out.UpdatedAt})
	})
	return
}
