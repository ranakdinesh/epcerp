package outbound

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"slices"
)

type scopedReadRepository struct {
	*repository
	assignments access.ReadableProjects
}

func (r *scopedReadRepository) ids(ctx context.Context, p domain.Principal) ([]string, error) {
	return r.assignments.Readable(ctx, access.Actor{ID: p.ID, Active: p.Active, Superuser: p.Superuser, Permissions: p.Permissions})
}
func (r *scopedReadRepository) CanReadAssigned(ctx context.Context, p domain.Principal, id string) (bool, error) {
	ids, e := r.ids(ctx, p)
	return slices.Contains(ids, id), e
}
func (r *scopedReadRepository) ListReadable(ctx context.Context, f domain.Filter, p domain.Principal) (domain.Page, error) {
	manager := p.ID
	ids := []string{}
	if p.Portfolio() {
		manager = ""
	} else {
		var e error
		ids, e = r.ids(ctx, p)
		if e != nil {
			return domain.Page{}, e
		}
	}
	return r.repository.listAssigned(ctx, f, manager, ids)
}
func (u *UnitOfWork) WithReadableAssignments(f func(database.Connection) access.ReadableProjects) *UnitOfWork {
	u.assignments = f
	return u
}
func projectReadUUIDs(ids []string) []pgtype.UUID {
	out := []pgtype.UUID{}
	for _, id := range ids {
		out = append(out, uuid(id))
	}
	return out
}
