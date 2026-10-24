package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/ports"
)

type Service struct {
	u ports.UnitOfWork
	a ports.Authorizer
	v ports.Values
}

func New(u ports.UnitOfWork, a ports.Authorizer, v ports.Values) *Service { return &Service{u, a, v} }
func (s *Service) actor(ctx context.Context, t, permission string) (domain.Actor, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, domain.Unauthorized
	}
	if !p.Can(permission) {
		return p, domain.Forbidden
	}
	return p, nil
}
func (s *Service) List(ctx context.Context, t, project string) (out []domain.Record, e error) {
	p, e := s.actor(ctx, t, "projects.items.read")
	if e != nil {
		return
	}
	if project, e = domain.ID(project); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		if e := c.Scope(ctx, p, project, "projects.items.read", false); e != nil {
			return e
		}
		out, e = r.List(ctx, project)
		return e
	})
	return
}
func (s *Service) History(ctx context.Context, t, project, id string) (out []domain.Event, e error) {
	p, e := s.actor(ctx, t, "projects.items.read")
	if e != nil {
		return
	}
	if project, e = domain.ID(project); e != nil {
		return
	}
	if id, e = domain.ID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		if e := c.Scope(ctx, p, project, "projects.items.read", false); e != nil {
			return e
		}
		v, e := r.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if v == nil {
			return domain.Missing
		}
		out, e = r.History(ctx, project, id)
		return e
	})
	return
}
func (s *Service) Save(ctx context.Context, t, project, id string, input domain.Save) (out domain.Record, e error) {
	p, e := s.actor(ctx, t, "projects.items.write")
	if e != nil {
		return
	}
	if project, e = domain.ID(project); e != nil {
		return
	}
	if id != "" {
		if id, e = domain.ID(id); e != nil {
			return
		}
	}
	input, e = input.Validate()
	if e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		if e := c.Scope(ctx, p, project, "projects.items.write", true); e != nil {
			return e
		}
		out, e = NewWriter(r, c, s.v).Save(ctx, project, id, input, p.ID)
		return e
	})
	return
}

// Writer is transaction-bound for BO publication. The consuming use case must
// enforce project material-write permission and the same scope before calling it.
type Writer struct {
	r ports.Repository
	c ports.Context
	v ports.Values
}

func NewWriter(r ports.Repository, c ports.Context, v ports.Values) *Writer { return &Writer{r, c, v} }
func (w *Writer) Save(ctx context.Context, project, id string, input domain.Save, actor string) (domain.Record, error) {
	var e error
	input, e = input.Validate()
	if e != nil {
		return domain.Record{}, e
	}
	existing, e := w.r.Get(ctx, project, id)
	if e != nil {
		return domain.Record{}, e
	}
	if id != "" && existing == nil {
		return domain.Record{}, domain.Missing
	}
	if existing != nil && existing.Version != input.Version {
		return domain.Record{}, domain.Stale
	}
	if existing != nil && existing.ItemID != input.ItemID {
		return domain.Record{}, domain.Invalid("item_id", "A project item's SKU cannot be replaced")
	}
	if existing != nil {
		committed, e := w.c.Committed(ctx, project, id)
		if e != nil {
			return domain.Record{}, e
		}
		if e = domain.CommitmentsGuard(input.Quantity, input.Active, committed); e != nil {
			return domain.Record{}, e
		}
	}
	var snapshot domain.Snapshot
	if existing == nil {
		snapshot, e = w.c.Item(ctx, input.ItemID, true)
	} else {
		snapshot = existing.Snapshot
		q, _ := domain.Quantity(input.Quantity)
		old, _ := domain.Quantity(existing.Quantity)
		if input.Active && (!existing.Active || q > old) {
			snapshot, e = w.c.Item(ctx, input.ItemID, true)
		}
	}
	if e != nil {
		return domain.Record{}, e
	}
	if input.Active {
		if e = w.c.Policy(ctx, project, snapshot); e != nil {
			return domain.Record{}, e
		}
	}
	if e = input.ValidateUnit(snapshot.UOM); e != nil {
		return domain.Record{}, e
	}
	demands, e := w.r.List(ctx, project)
	if e != nil {
		return domain.Record{}, e
	}
	for _, d := range demands {
		if d.ID == id {
			continue
		}
		if d.ItemID == input.ItemID {
			return domain.Record{}, domain.Invalid("item_id", "This SKU already exists in the project; update its existing requirement")
		}
		if d.EngineeringKey == snapshot.EngineeringKey {
			return domain.Record{}, domain.Invalid("item_id", "This engineering configuration already exists; add the make as an alternative")
		}
	}
	for _, id := range input.Alternatives {
		other, e := w.c.Item(ctx, id, true)
		if e != nil {
			return domain.Record{}, e
		}
		if e = w.c.Policy(ctx, project, other); e != nil {
			return domain.Record{}, e
		}
		if other.EngineeringKey != snapshot.EngineeringKey {
			return domain.Record{}, domain.Invalid("alternatives", "An alternative must have the same engineering configuration")
		}
	}
	record := domain.Record{ID: id, ProjectID: project, ItemID: input.ItemID, EngineeringKey: snapshot.EngineeringKey, Snapshot: snapshot, Quantity: input.Quantity, RequiredDate: input.RequiredDate, Allocations: input.Allocations, Alternatives: input.Alternatives, Active: input.Active, Version: 1}
	action := "create"
	if existing == nil {
		if input.Version != 1 {
			return record, domain.Stale
		}
		record.ID = w.v.ID()
	} else {
		record.Version = existing.Version + 1
		action = "revise"
	}
	saved, e := w.r.Save(ctx, record, actor)
	if e != nil {
		return record, e
	}
	if e = w.r.Event(ctx, domain.Event{ID: w.v.ID(), SubjectID: saved.ID, Action: action, Reason: input.Reason, Before: existing, After: saved, ActorID: actor, CreatedAt: saved.UpdatedAt}); e != nil {
		return record, e
	}
	return saved, nil
}

func (s *Service) Catalogue(ctx context.Context, t, project string) (out []domain.CatalogueChoice, e error) {
	a, e := s.actor(ctx, t, "projects.items.read")
	if e != nil {
		return
	}
	if project, e = domain.ID(project); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		if e := c.Scope(ctx, a, project, "projects.items.read", false); e != nil {
			return e
		}
		out, e = c.Catalogue(ctx, project)
		return e
	})
	return
}
