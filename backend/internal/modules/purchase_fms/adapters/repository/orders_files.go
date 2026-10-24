package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func (r *ordersRepository) File(ctx context.Context, order, id string) (*domain.OrdersFile, error) {
	v, e := r.q.OrdersClientFile(ctx, sqlc.OrdersClientFileParams{OrderID: dtos.UUID(order), ID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &domain.OrdersFile{ID: v.ID.String(), OrderID: v.OrderID.String(), DraftRevision: v.DraftRevision, Name: v.Name, MediaType: v.MediaType, Size: v.ByteSize, SHA256: v.Sha256, Bucket: v.ObjectBucket, ObjectKey: v.ObjectKey, CreatedAt: v.CreatedAt.Time, CreatedBy: v.CreatedByID.String()}, nil
}
func (r *ordersRepository) StoreFile(ctx context.Context, f domain.OrdersFile) error {
	return r.q.OrdersStoreClientFile(ctx, sqlc.OrdersStoreClientFileParams{ID: dtos.UUID(f.ID), OrderID: dtos.UUID(f.OrderID), DraftRevision: f.DraftRevision, Name: f.Name, MediaType: f.MediaType, ByteSize: f.Size, Sha256: f.SHA256, ObjectBucket: f.Bucket, ObjectKey: f.ObjectKey, At: dtos.Stamp(f.CreatedAt), ActorID: dtos.UUID(f.CreatedBy)})
}
func (r *ordersRepository) FileEvent(ctx context.Context, e domain.OrdersFileEvent) error {
	if err := r.q.OrdersClientFileEvent(ctx, sqlc.OrdersClientFileEventParams{ID: dtos.UUID(e.ID), OrderID: dtos.UUID(e.OrderID), FileID: dtos.UUID(e.FileID), Action: e.Action, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	o, err := r.q.OrdersGetByID(ctx, dtos.UUID(e.OrderID))
	if err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_orders", EntityType: "purchase_order_client_file", EntityID: e.FileID, EntityVersion: 1, Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: o.ProjectID.String(), Visibility: "restricted", OccurredAt: e.At}, nil)
}
