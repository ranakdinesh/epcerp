package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	db "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound/internal/db"
)

type workInventory struct {
	c database.Connection
	q *db.Queries
}

func NewWorkInventory(c database.Connection) auth.WorkSource { return &workInventory{c, db.New(c)} }
func (r *workInventory) read(ctx context.Context, owner, kind, id string) ([]auth.Responsibility, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("work inventory requires shared transaction")
	}
	rows, e := r.q.ReadWorkInventory(ctx, db.ReadWorkInventoryParams{OwnerID: owner, RecordID: id, Kind: kind})
	if e != nil {
		return nil, e
	}
	if len(rows) > 10000 {
		return nil, errors.New("work inventory exceeds reviewed bounds")
	}
	out := []auth.Responsibility{}
	for _, raw := range rows {
		var v auth.Responsibility
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
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
