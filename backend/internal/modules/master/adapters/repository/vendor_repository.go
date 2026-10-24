package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	"strings"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorUnitOfWork struct {
	m      *database.Manager
	events func(database.Connection) events.Writer
}

func NewVendorUnitOfWork(b database.Beginner, e func(database.Connection) events.Writer) *VendorUnitOfWork {
	return &VendorUnitOfWork{database.NewManager(b), e}
}
func (u *VendorUnitOfWork) Within(ctx context.Context, w func(context.Context, masterPorts.VendorRepository) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return w(ctx, &vendorRepository{NewStore(c).Queries, u.events(c)})
	})
}

type vendorRepository struct {
	q      *db.Queries
	events events.Writer
}

func (r *vendorRepository) List(ctx context.Context) ([]masterDomain.Vendor, error) {
	rows, e := r.q.VendorProfileListSuppliers(ctx)
	if e != nil {
		return nil, e
	}
	result := []masterDomain.Vendor{}
	for _, v := range rows {
		record, e := masterDTO.VendorFromRow(v)
		if e != nil {
			return nil, e
		}
		result = append(result, *record)
	}
	return result, nil
}
func (r *vendorRepository) BySource(ctx context.Context, id string) (*masterDomain.Vendor, error) {
	if e := r.q.VendorProfileLockImport(ctx); e != nil {
		return nil, e
	}
	v, e := r.q.VendorProfileSourceSupplier(ctx, masterDTO.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorFromRow(v)
}
func (r *vendorRepository) Insert(ctx context.Context, v masterDomain.Vendor) (*masterDomain.Vendor, error) {
	data, e := masterDTO.MarshalMasterJSON(v.Data)
	if e != nil {
		return nil, e
	}
	row, e := r.q.VendorProfileInsertSupplier(ctx, db.VendorProfileInsertSupplierParams{ID: masterDTO.UUID(v.ID), Code: v.Code, Name: v.Name, Data: data, Active: v.Active, Actor: masterDTO.UUID(*v.CreatedBy), SourceID: masterDTO.UUID(v.Source.ID), SourceVersion: pgtype.Int4{Int32: v.Source.Version, Valid: v.Source.ID != ""}, SourceCreatedAt: pgtype.Timestamptz{Time: v.Source.CreatedAt, Valid: v.Source.ID != ""}, SourceUpdatedAt: pgtype.Timestamptz{Time: v.Source.UpdatedAt, Valid: v.Source.ID != ""}})
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" {
		return nil, masterDomain.VendorConflict
	}
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorFromRow(row)
}
func (r *vendorRepository) Imported(ctx context.Context, id, event, actor string) error {
	row, e := r.q.VendorProfileGetSupplier(ctx, masterDTO.UUID(id))
	if e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: event, Module: "suppliers", EntityType: "supplier", EntityID: id, EntityVersion: 1, Action: "legacy_imported", ActorID: actor, ActorKind: "user", CorrelationID: event, ScopeKind: "platform", Visibility: "restricted", OccurredAt: row.CreatedAt.Time}, nil)
}

type VendorDirectory struct{ q *db.Queries }

func NewVendorDirectory(c database.Connection) *VendorDirectory {
	return &VendorDirectory{NewStore(c).Queries}
}
func (d *VendorDirectory) Exists(ctx context.Context, id string) (bool, error) {
	_, e := d.q.VendorProfileSupplierExists(ctx, masterDTO.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	return e == nil, e
}

type VendorValues struct{}

func (VendorValues) ID() string {
	return newRecordID()
}

func (VendorValues) Fingerprint(value map[string]any) string {
	encoded, e := masterDTO.MarshalMasterJSON(value)
	if e != nil {
		return ""
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

func (r *vendorRepository) Get(ctx context.Context, id string) (*masterDomain.Vendor, error) {
	v, e := r.q.VendorProfileGetSupplier(ctx, masterDTO.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorFromRow(v)
}
func (r *vendorRepository) Update(ctx context.Context, v masterDomain.Vendor) (*masterDomain.Vendor, error) {
	data, e := masterDTO.MarshalMasterJSON(v.Data)
	if e != nil {
		return nil, e
	}
	row, e := r.q.VendorProfileUpdateSupplier(ctx, db.VendorProfileUpdateSupplierParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Data: data, Active: v.Active, Actor: masterDTO.UUID(*v.UpdatedBy), PreviousVersion: v.Version - 1})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, masterDomain.VendorStale
	}
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorFromRow(row)
}
func (r *vendorRepository) Changed(ctx context.Context, v masterDomain.Vendor, event, action, reason string) error {
	data, e := masterDTO.MarshalMasterJSON(v)
	if e != nil {
		return e
	}
	if e = r.q.VendorProfileRecordChange(ctx, db.VendorProfileRecordChangeParams{ID: masterDTO.UUID(event), SupplierID: masterDTO.UUID(v.ID), Version: v.Version, Action: action, Reason: reason, Snapshot: data, CreatedByID: masterDTO.UUID(*v.UpdatedBy)}); e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: event, Module: "suppliers", EntityType: "supplier", EntityID: v.ID, EntityVersion: int64(v.Version), Action: action, ActorID: *v.UpdatedBy, ActorKind: "user", CorrelationID: event, ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.UpdatedAt}, nil)
}
func (d *VendorDirectory) Snapshot(ctx context.Context, id string) (*masterContracts.VendorSnapshot, error) {
	v, e := d.q.VendorProfileGetSupplier(ctx, masterDTO.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	r, e := masterDTO.VendorFromRow(v)
	if e != nil {
		return nil, e
	}
	gst, _ := r.Data["gst_number"].(string)
	return &masterContracts.VendorSnapshot{ID: r.ID, Version: r.Version, Active: r.Active && r.Data["qualification"] != "suspended", GSTRegistered: strings.TrimSpace(gst) != ""}, nil
}

func (d *VendorDirectory) Candidates(ctx context.Context) ([]masterDomain.Vendor, error) {
	rows, e := (&vendorRepository{q: d.q}).List(ctx)
	if e != nil {
		return nil, e
	}
	out := []masterDomain.Vendor{}
	for _, r := range rows {
		if r.Active {
			out = append(out, r)
		}
	}
	return out, nil
}
