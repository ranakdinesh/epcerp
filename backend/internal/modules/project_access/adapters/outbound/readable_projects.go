package outbound

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
)

type readableProjects struct {
	c database.Connection
	p Providers
}

func NewReadableProjects(c database.Connection, p Providers) contracts.ReadableProjects {
	return &readableProjects{c, p}
}
func (r *readableProjects) Readable(ctx context.Context, a contracts.Actor) ([]string, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Project assignments require the consumer transaction")
	}
	rows, e := db.New(r.c).AssignedProjectCandidates(ctx, uuid(a.ID))
	if e != nil {
		return nil, e
	}
	if len(rows) > 1000 {
		return nil, errors.New("Assigned project reporting limit exceeded")
	}
	out := []string{}
	gate := NewGate(r.c, r.p)
	for _, id := range rows {
		_, e := gate.Check(ctx, a, id.String(), "projects.dashboard.read", false)
		if errors.Is(e, contracts.ErrForbidden) || errors.Is(e, contracts.ErrMissing) {
			continue
		}
		if e != nil {
			return nil, e
		}
		out = append(out, id.String())
	}
	return out, nil
}
