package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type OrdersService struct {
	u         ports.OrdersUnitOfWork
	a         ports.OrdersAuthorizer
	v         ports.OrdersValues
	cipher    ports.TechnicalCipher
	inspector ports.TechnicalInspector
	objects   ports.OrdersObjects
	bucket    string
}

func NewOrders(u ports.OrdersUnitOfWork, a ports.OrdersAuthorizer, v ports.OrdersValues) *OrdersService {
	return &OrdersService{u: u, a: a, v: v}
}
func (s *OrdersService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.OrdersUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.OrdersID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *OrdersService) workspace(ctx context.Context, r ports.OrdersRepository, c ports.OrdersContext, a domain.Principal, p, id string) (out domain.OrdersWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.OrdersAccessGuard(a, caps, "read"); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.OrdersInvalid("source", "Order source does not belong to this project/package")
	}
	out.Order, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.OrdersEvent{}
	out.TaskOnly = !caps.Scoped
	if out.Order != nil {
		v := domain.OrdersCopy(*out.Order)
		out.Order = &v
		if !out.Order.Active {
			return out, domain.OrdersMissing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	if out.Source.Guard() != nil || out.Order != nil && out.Order.State == "released" {
		return out, nil
	}
	if domain.OrdersAccessGuard(a, caps, "prepare") == nil {
		out.Actions = append(out.Actions, "prepare")
	}
	if out.Order != nil && domain.OrdersSameSource(out.Order.Source, out.Source) {
		if out.Order.DraftRevision > 0 && (out.Order.ClientApproval.Status == "not_submitted" || out.Order.ClientApproval.Status == "rejected" || out.Order.ClientApproval.Status == "") && domain.OrdersAccessGuard(a, caps, "submit_client") == nil {
			out.Actions = append(out.Actions, "submit_client")
		}
		if out.Order.ClientApproval.Status == "submitted" && domain.OrdersAccessGuard(a, caps, "client_decision") == nil {
			out.Actions = append(out.Actions, "client_decision")
		}
	}
	if out.Order != nil && domain.OrdersClientCleared(*out.Order) && domain.OrdersSameSource(out.Order.Source, out.Source) && domain.OrdersAccessGuard(a, caps, "release") == nil {
		out.Actions = append(out.Actions, "release")
	}
	return out, nil
}
func (s *OrdersService) Workspace(ctx context.Context, token, p, id string) (out domain.OrdersWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.OrdersRepository, c ports.OrdersContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.OrdersWorkspace{}
	}
	return
}
func (s *OrdersService) Execute(ctx context.Context, token, p, id string, q domain.OrdersCommand) (out domain.OrdersWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.OrdersValidate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.OrdersRepository, c ports.OrdersContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.OrdersAccessGuard(a, caps, q.Action); err != nil {
			return err
		}
		before, err := r.Get(ctx, p, id)
		if err != nil {
			return err
		}
		version := int32(0)
		if before != nil {
			version = before.Version
		}
		if q.Version != version {
			return domain.OrdersStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.OrdersInvalid("source", "Order source does not belong to this project/package")
		}
		if err = source.Guard(); err != nil {
			return err
		}
		now := s.v.Now()
		number := ""
		if q.Action == "release" {
			if before == nil {
				return domain.OrdersInvalid("order", "Prepare the complete order before release")
			}
			if !domain.OrdersSameSource(before.Source, source) {
				return domain.OrdersStale
			}
			year, err := domain.OrdersFiscalYear(before.Details.Date)
			if err != nil {
				return err
			}
			sequence, err := r.NextNumber(ctx, year)
			if err != nil {
				return err
			}
			number, err = domain.OrdersNumber(before.Details.Date, sequence)
			if err != nil {
				return err
			}
		}
		if q.ClientDecision != nil && q.ClientDecision.FileID != "" {
			if err = s.clientFile(ctx, r, before, q.ClientDecision.FileID); err != nil {
				return err
			}
		}
		next, err := domain.OrdersApply(before, a, caps, q, source, now, s.v.ID(), number)
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.OrdersEvent{ID: s.v.ID(), OrderID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.OrdersWorkspace{}
	}
	return
}

// Document authorizes and commits export attribution before returning the frozen
// snapshot. Current downstream gates never rewrite a released historical order.
func (s *OrdersService) Document(ctx context.Context, token, p, id string) (out domain.OrdersOrder, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.OrdersRepository, c ports.OrdersContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.OrdersAccessGuard(a, caps, "read"); err != nil {
			return err
		}
		record, err := r.Get(ctx, p, id)
		if err != nil {
			return err
		}
		if record == nil || !record.Active {
			return domain.OrdersMissing
		}
		if err = r.DocumentEvent(ctx, domain.OrdersDocumentEvent{ID: s.v.ID(), OrderID: record.ID, ProjectID: record.ProjectID, Version: record.Version, ActorID: a.ID, At: s.v.Now()}); err != nil {
			return err
		}
		out = domain.OrdersCopy(*record)
		return nil
	})
	if e != nil {
		out = domain.OrdersOrder{}
	}
	return
}
