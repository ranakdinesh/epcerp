package application

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

func (s *CategoryService) Browse(ctx context.Context, t string, q masterDomain.CategoryBrowse) (out masterDomain.CategoryPage, e error) {
	if _, e = s.actor(ctx, t, "settings.categories.read"); e != nil {
		return
	}
	if e = masterDomain.CategoryValidateBrowse(q); e != nil {
		return
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.CategoryRepository) error { out, e = r.Browse(ctx, q); return e })
	return
}
func (s *CategoryService) Detail(ctx context.Context, t, kind, id string) (out masterDomain.CategorySummary, e error) {
	if _, e = s.actor(ctx, t, "settings.categories.read"); e != nil {
		return
	}
	if e = masterDomain.CategoryValidateDetail(kind, id); e != nil {
		return
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.CategoryRepository) error {
		out, e = r.Detail(ctx, kind, id)
		return e
	})
	return
}
