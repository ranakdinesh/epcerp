package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	workprojects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
)

type workInventory struct {
	parents workprojects.ContextReader
	c       database.Connection
	q       *sqlc.Queries
}

func NewWorkInventory(c database.Connection, parents workprojects.ContextReader) auth.WorkSource {
	return &workInventory{parents: parents, c: c, q: NewStore(c)}
}
func (r *workInventory) read(ctx context.Context, owner, kind, id string) ([]auth.Responsibility, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("work inventory requires shared transaction")
	}
	rows, e := r.q.ReadWorkInventory(ctx, sqlc.ReadWorkInventoryParams{OwnerID: owner, RecordID: id, Kind: kind})
	if e != nil {
		return nil, e
	}
	if len(rows) > 10000 {
		return nil, errors.New("work inventory exceeds reviewed bounds")
	}
	if r.parents == nil {
		return nil, errors.New("project work requires its bound parent context")
	}
	parents := map[string]*workprojects.Context{}
	out := []auth.Responsibility{}
	for _, raw := range rows {
		var v auth.Responsibility
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		parent, found := parents[v.ProjectID]
		if !found {
			parent, e = r.parents.Get(ctx, v.ProjectID, false)
			if e != nil {
				return nil, e
			}
			parents[v.ProjectID] = parent
		}
		if parent == nil {
			return nil, errors.New("project work parent context missing")
		}
		// Closed/deactivated project execution is historical. Retain every original
		// assignment and actor; a departed employee must not be rewritten into it.
		if !parent.Active || parent.Lifecycle == "closed" {
			continue
		}
		if v.Kind == "purchasing_owner" {
			v.Title = parent.Name + " · purchasing ownership"
		} else {
			v.Title = parent.Name + " · " + v.Title
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *workInventory) Inventory(ctx context.Context, user string) ([]auth.Responsibility, error) {
	if user == "" {
		return nil, errors.New("employee required")
	}
	return r.read(ctx, user, "", "")
}
func (r *workInventory) Current(ctx context.Context, kind, id string) ([]auth.Responsibility, error) {
	if kind == "" || id == "" {
		return nil, errors.New("reference required")
	}
	return r.read(ctx, "", kind, id)
}
