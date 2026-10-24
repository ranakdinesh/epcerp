package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/ports"
)

type Service struct {
	Unit ports.Unit
	Auth ports.Auth
}

func (s Service) List(ctx context.Context, token string, year int) ([]domain.Holiday, error) {
	p, e := s.Auth.Resolve(ctx, token)
	if e != nil {
		return nil, e
	}
	if !p.CanRead {
		return nil, domain.Forbidden
	}
	if year < 1 || year > 9999 {
		return nil, domain.Invalid
	}
	var rows []domain.Holiday
	e = s.Unit.Within(ctx, func(ctx context.Context, r ports.Repository, _ func(context.Context, string) (int, error)) error {
		var e error
		rows, e = r.List(ctx, year)
		return e
	})
	return rows, e
}
func (s Service) Save(ctx context.Context, token string, c domain.Change) (domain.Holiday, error) {
	p, e := s.Auth.Resolve(ctx, token)
	if e != nil {
		return domain.Holiday{}, e
	}
	if !p.CanWrite {
		return domain.Holiday{}, domain.Forbidden
	}
	c, e = c.Validate()
	if e != nil {
		return domain.Holiday{}, e
	}
	var row domain.Holiday
	e = s.Unit.Within(ctx, func(ctx context.Context, r ports.Repository, refresh func(context.Context, string) (int, error)) error {
		var e error
		row, e = r.Save(ctx, c, p.ID)
		if e != nil {
			return e
		}
		_, e = refresh(ctx, p.ID)
		return e
	})
	return row, e
}

type Reader struct{ Repo ports.Repository }

func (r Reader) Count(ctx context.Context, start, end string) (domain.Count, error) {
	dates, e := r.Repo.Dates(ctx)
	if e != nil {
		return domain.Count{}, e
	}
	return domain.Working(start, end, dates)
}
