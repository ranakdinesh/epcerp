package outbound

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"strings"
)

type contextReader struct{ r *repository }

func NewContextReader(c database.Connection) contracts.ContextReader {
	return &contextReader{&repository{q: db.New(c)}}
}
func (r *contextReader) Get(ctx context.Context, id string, lock bool) (*domain.Project, error) {
	return r.r.Get(ctx, id, lock)
}
func (r *contextReader) Page(ctx context.Context, f domain.Filter, actor string, ids []string, portfolio bool) (domain.Page, error) {
	var e error
	if f, e = f.Validate(); e != nil {
		return domain.Page{}, e
	}
	assigned := []pgtype.UUID{}
	for _, id := range ids {
		assigned = append(assigned, uuid(id))
	}
	search := ""
	if f.Search != "" {
		search = "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.Search) + "%"
	}
	rows, e := r.r.q.ListProcessProjects(ctx, db.ListProcessProjectsParams{Sort: f.Sort, Direction: f.Direction, Portfolio: portfolio, ActorID: uuid(actor), AssignedIds: assigned, Search: search, PageSize: f.PageSize, PageOffset: (f.Page - 1) * f.PageSize})
	if e != nil {
		return domain.Page{}, e
	}
	total, e := r.r.q.CountProcessProjects(ctx, db.CountProcessProjectsParams{Portfolio: portfolio, ActorID: uuid(actor), AssignedIds: assigned, Search: search})
	out := domain.Page{Items: []domain.Project{}, Total: total, Page: f.Page, PageSize: f.PageSize}
	for _, v := range rows {
		out.Items = append(out.Items, project(v))
	}
	return out, e
}
