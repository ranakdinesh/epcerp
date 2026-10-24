package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dto "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
	"time"
)

type VendorDocumentRelocationUnitOfWork struct {
	m      *database.Manager
	events func(database.Connection) events.Writer
}

func NewVendorDocumentRelocationUnitOfWork(b database.Beginner, j func(database.Connection) events.Writer) *VendorDocumentRelocationUnitOfWork {
	return &VendorDocumentRelocationUnitOfWork{database.NewManager(b), j}
}
func (u *VendorDocumentRelocationUnitOfWork) Within(ctx context.Context, work func(context.Context, ports.VendorDocumentRelocationRepository) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return work(ctx, &vendorDocumentRelocationRepository{NewStore(c).Queries, u.events(c)})
	})
}

type vendorDocumentRelocationRepository struct {
	q      *db.Queries
	events events.Writer
}

func (r *vendorDocumentRelocationRepository) NextInline(ctx context.Context) (*domain.VendorDocumentDocument, error) {
	v, err := r.q.VendorDocumentsNextInline(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return dto.VendorDocumentFromRow(v), nil
}
func (r *vendorDocumentRelocationRepository) Relocate(ctx context.Context, d domain.VendorDocumentDocument, bucket, key, event string) error {
	if err := r.q.VendorDocumentsRelocate(ctx, db.VendorDocumentsRelocateParams{ID: dto.UUID(d.ID), ObjectBucket: pgtype.Text{String: bucket, Valid: true}, ObjectKey: pgtype.Text{String: key, Valid: true}}); err != nil {
		return err
	}
	return r.events.Record(ctx, events.Event{ID: event, Module: "supplier_documents", EntityType: "supplier_document", EntityID: d.ID, EntityVersion: int64(d.Version), Action: "storage_relocated", ActorKind: "system", CorrelationID: event, ScopeKind: "platform", Visibility: "restricted", OccurredAt: time.Now().UTC()}, nil)
}
