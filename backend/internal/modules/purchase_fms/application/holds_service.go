package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type HoldsService struct {
	u ports.HoldsUnitOfWork
	a ports.HoldsAuthorizer
	v ports.HoldsValues
}

func NewHolds(u ports.HoldsUnitOfWork, a ports.HoldsAuthorizer, v ports.HoldsValues) *HoldsService {
	return &HoldsService{u, a, v}
}
func (s *HoldsService) actor(ctx context.Context, t string, actions ...string) (domain.Principal, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, domain.HoldsUnauthorized
	}
	for _, permission := range actions {
		if !p.Can(permission) {
			return p, domain.HoldsForbidden
		}
	}
	return p, nil
}

func (s *HoldsService) List(ctx context.Context, t, project string) (out []domain.HoldsRecord, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_holds.read")
	if e != nil {
		return
	}
	if project, e = domain.HoldsID(project); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.HoldsRepository, c ports.HoldsContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_holds.read", false); e != nil {
			return e
		}
		out, e = r.List(ctx, project)
		return e
	})
	return
}
func (s *HoldsService) History(ctx context.Context, t, project, id string) (out []domain.HoldsEvent, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_holds.read")
	if e != nil {
		return
	}
	if project, e = domain.HoldsID(project); e != nil {
		return
	}
	if id, e = domain.HoldsID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.HoldsRepository, c ports.HoldsContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_holds.read", false); e != nil {
			return e
		}
		record, e := r.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if record == nil {
			return domain.HoldsMissing
		}
		out, e = r.History(ctx, project, id)
		return e
	})
	return
}
func (s *HoldsService) Create(ctx context.Context, t, project string, input domain.HoldsCreate) (out domain.HoldsRecord, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_holds.write")
	if e != nil {
		return
	}
	if project, e = domain.HoldsID(project); e != nil {
		return
	}
	input, e = input.Validate(s.v.Now())
	if e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.HoldsRepository, c ports.HoldsContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_holds.write", true); e != nil {
			return e
		}
		if e := c.Validate(ctx, project, input); e != nil {
			return e
		}
		now := s.v.Now()
		data := domain.HoldsData{Scope: input.Scope, CategoryCode: input.CategoryCode, DemandIDs: input.DemandIDs, Reason: input.Reason, Evidence: input.Evidence, StartedAt: input.StartedAt.UTC(), ExpectedReleaseAt: input.ExpectedReleaseAt, DoerID: input.DoerID, ResolverID: input.ResolverID, RecordedAt: now}
		record := domain.HoldsRecord{ID: s.v.ID(), ProjectID: project, Data: data, Active: true, Version: 1}
		out, e = r.Save(ctx, record, p.ID)
		if e != nil {
			return e
		}
		return r.Event(ctx, domain.HoldsEvent{ID: s.v.ID(), SubjectID: out.ID, Action: "hold", Reason: input.Reason, After: out, ActorID: p.ID, CreatedAt: out.UpdatedAt})
	})
	return
}
func (s *HoldsService) Release(ctx context.Context, t, project, id string, input domain.HoldsDecision) (out domain.HoldsRecord, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_holds.release")
	if e != nil {
		return
	}
	if project, e = domain.HoldsID(project); e != nil {
		return
	}
	if id, e = domain.HoldsID(id); e != nil {
		return
	}
	input.Reason, e = domain.HoldsText("reason", input.Reason)
	if e != nil {
		return
	}
	input.Evidence, e = domain.HoldsText("evidence", input.Evidence)
	if e != nil {
		return
	}
	if input.Version < 1 {
		return out, domain.HoldsInvalid("version", "Use the saved version")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.HoldsRepository, c ports.HoldsContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_holds.release", true); e != nil {
			return e
		}
		record, e := r.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if record == nil {
			return domain.HoldsMissing
		}
		if record.Version != input.Version {
			return domain.HoldsStale
		}
		if !record.Active {
			return domain.HoldsInvalid("", "Hold already released")
		}
		before := *record
		now := s.v.Now()
		record.Data.ReleasedAt = &now
		record.Data.ReleaseEvidence = &input.Evidence
		record.Active = false
		record.Version++
		out, e = r.Save(ctx, *record, p.ID)
		if e != nil {
			return e
		}
		return r.Event(ctx, domain.HoldsEvent{ID: s.v.ID(), SubjectID: id, Action: "release", Reason: input.Reason, Before: &before, After: out, ActorID: p.ID, CreatedAt: out.UpdatedAt})
	})
	return
}
