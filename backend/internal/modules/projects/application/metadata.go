package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/ports"
)

// MetadataService owns correction of the Project Master, not ownership handovers.
type MetadataService struct {
	unit ports.UnitOfWork
	auth ports.Authorizer
}

func NewMetadata(u ports.UnitOfWork, a ports.Authorizer) *MetadataService {
	return &MetadataService{u, a}
}
func (s *MetadataService) actor(ctx context.Context, token string) (domain.Principal, error) {
	p, e := s.auth.Resolve(ctx, token)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, domain.ErrUnauthorized
	}
	if !p.Allows("projects.master.read") {
		return p, domain.ErrForbidden
	}
	return p, nil
}
func (s *MetadataService) Correct(ctx context.Context, token, id string, input domain.Correction) (*domain.Project, error) {
	actor, e := s.actor(ctx, token)
	if e != nil {
		return nil, e
	}
	if !actor.Allows("projects.master.write") {
		return nil, domain.ErrForbidden
	}
	id, e = domain.ID(id)
	if e != nil {
		return nil, e
	}
	input, e = input.Validate()
	if e != nil {
		return nil, e
	}
	var saved domain.Project
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Clients, _ ports.Users, calendar ports.Calendar, purchasing ports.Purchasing) error {
		p, e := r.Get(ctx, id, true)
		if e != nil {
			return e
		}
		if p == nil {
			return domain.ErrMissing
		}
		if !domain.CanCorrect(actor, *p) {
			return domain.ErrForbidden
		}
		if p.Version != input.ExpectedVersion {
			return domain.ErrStale
		}
		if !p.Active || p.Lifecycle != "draft" {
			return &domain.Invalid{Field: "", Message: "Only active draft project masters can be corrected"}
		}
		if input.ProjectType == "other" && p.ProjectType != "other" {
			return &domain.Invalid{Field: "project_type", Message: "Select a governed business type"}
		}
		client, e := c.Get(ctx, input.ClientID)
		if e != nil {
			return e
		}
		if client == nil || (!client.Active && input.ClientID != p.ClientID) {
			return &domain.Invalid{Field: "client_id", Message: "Select an active client"}
		}
		// Technical legacy operations identity belongs to the existing record, never the form.
		input.Capture, e = calculate(ctx, input.Capture, calendar)
		if e != nil {
			return e
		}
		input.OperationsManagerID = p.OperationsManagerID
		p.UpdatedBy = actor.ID
		saved, e = r.UpdateMetadata(ctx, *p, input)
		if e != nil {
			return e
		}
		saved.ProcurementExecutiveID, e = purchasing.Owner(ctx, id)
		return e
	})
	if e != nil {
		return nil, e
	}
	return &saved, nil
}
func (s *MetadataService) History(ctx context.Context, token, id string) ([]domain.MetadataChange, error) {
	actor, e := s.actor(ctx, token)
	if e != nil {
		return nil, e
	}
	id, e = domain.ID(id)
	if e != nil {
		return nil, e
	}
	var out []domain.MetadataChange
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, _ ports.Clients, _ ports.Users, _ ports.Calendar, _ ports.Purchasing) error {
		p, e := r.Get(ctx, id, false)
		if e != nil {
			return e
		}
		if p == nil {
			return domain.ErrMissing
		}
		if allowed, e := canReadProject(ctx, r, actor, *p); e != nil {
			return e
		} else if !allowed {
			return domain.ErrForbidden
		}
		out, e = r.MetadataHistory(ctx, id)
		return e
	})
	return out, e
}

func (s *MetadataService) Options(ctx context.Context, token, id string) ([]domain.Client, error) {
	actor, e := s.actor(ctx, token)
	if e != nil {
		return nil, e
	}
	id, e = domain.ID(id)
	if e != nil {
		return nil, e
	}
	var out []domain.Client
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Clients, _ ports.Users, _ ports.Calendar, _ ports.Purchasing) error {
		p, e := r.Get(ctx, id, false)
		if e != nil {
			return e
		}
		if p == nil {
			return domain.ErrMissing
		}
		if !domain.CanCorrect(actor, *p) {
			return domain.ErrForbidden
		}
		out, e = c.List(ctx)
		if e != nil {
			return e
		}
		found := false
		for _, v := range out {
			if v.ID == p.ClientID {
				found = true
			}
		}
		if !found {
			v, e := c.Get(ctx, p.ClientID)
			if e != nil {
				return e
			}
			if v != nil {
				out = append(out, *v)
			}
		}
		return nil
	})
	return out, e
}

func (s *MetadataService) Preview(ctx context.Context, token, id string, capture domain.Capture) (domain.Calculation, error) {
	actor, e := s.actor(ctx, token)
	if e != nil {
		return domain.Calculation{}, e
	}
	id, e = domain.ID(id)
	if e != nil {
		return domain.Calculation{}, e
	}
	var out domain.Capture
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, _ ports.Clients, _ ports.Users, calendar ports.Calendar, _ ports.Purchasing) error {
		p, e := r.Get(ctx, id, false)
		if e != nil {
			return e
		}
		if p == nil {
			return domain.ErrMissing
		}
		if !domain.CanCorrect(actor, *p) {
			return domain.ErrForbidden
		}
		capture, e = capture.ValidateDraft()
		if e != nil {
			return e
		}
		out, e = calculate(ctx, capture, calendar)
		return e
	})
	return out.Calculation, e
}
