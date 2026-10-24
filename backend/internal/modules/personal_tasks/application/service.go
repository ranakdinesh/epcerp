package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/ports"
)

type Service struct {
	u ports.UnitOfWork
	a ports.Authorizer
	v ports.Values
}

func New(u ports.UnitOfWork, a ports.Authorizer, v ports.Values) *Service { return &Service{u, a, v} }
func (s *Service) Page(ctx context.Context, token string, q domain.Query) (out domain.Page, e error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active || a.ID == "" {
		return out, domain.Unauthorized
	}
	if !a.Can("work.tasks.view") || !a.Can("work.tasks.read") {
		return out, domain.Forbidden
	}
	q, e = q.Validate()
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Reader) error {
		if err := r.Authorize(ctx, a); err != nil {
			return err
		}
		rows, total, err := r.Assigned(ctx, a, q)
		if err != nil {
			return err
		}
		out = domain.Page{Rows: []domain.Task{}, Total: total, Page: q.Page, Size: q.Size}
		if qr, ok := r.(ports.QuotationReader); ok {
			out.Quotations, err = qr.Quotations(ctx, a)
			if err != nil {
				return err
			}
		}
		if sr, ok := r.(ports.SupplierReviewReader); ok {
			out.SupplierReviews, err = sr.SupplierReviews(ctx, a)
			if err != nil {
				return err
			}
		}
		for _, t := range rows {
			out.Rows = append(out.Rows, t.WithClock(s.v.Now()))
		}
		return nil
	})
	if e != nil {
		out = domain.Page{}
	}
	return out, e
}
