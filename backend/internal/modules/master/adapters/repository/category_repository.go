package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type CategoryUnitOfWork struct {
	manager *database.Manager
	journal func(database.Connection) events.Writer
}

func NewCategoryUnitOfWork(b database.Beginner, j func(database.Connection) events.Writer) *CategoryUnitOfWork {
	return &CategoryUnitOfWork{database.NewManager(b), j}
}
func (u *CategoryUnitOfWork) Within(ctx context.Context, f func(context.Context, masterPorts.CategoryRepository) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &categoryRepository{NewStore(c).Queries, u.journal(c)})
	})
}

type categoryRepository struct {
	q       *db.Queries
	journal events.Writer
}

func (r *categoryRepository) Lock(ctx context.Context) error { return r.q.CategoriesLockImport(ctx) }
func (r *categoryRepository) List(ctx context.Context) (masterDomain.CategoryCatalogue, error) {
	out := masterDomain.CategoryCatalogue{Categories: []masterDomain.Category{}, Subcategories: []masterDomain.Category{}, Units: []masterDomain.Category{}}
	cats, e := r.q.CategoriesListCategories(ctx)
	if e != nil {
		return out, e
	}
	for _, v := range cats {
		out.Categories = append(out.Categories, masterDomain.Category{ID: v.ID.String(), Code: v.Code, Name: v.Name, Active: v.IsActive, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: masterDTO.Actor(v.CreatedByID), UpdatedBy: masterDTO.Actor(v.UpdatedByID), Fingerprint: v.SourceFingerprint.String, UnitID: v.UnitID.String()})
	}
	units, e := r.q.CategoriesListUnits(ctx)
	if e != nil {
		return out, e
	}
	for _, v := range units {
		out.Units = append(out.Units, masterDomain.Category{ID: v.ID.String(), Code: v.Code, Name: v.Name, Active: v.IsActive, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: masterDTO.Actor(v.CreatedByID), UpdatedBy: masterDTO.Actor(v.UpdatedByID), Fingerprint: v.SourceFingerprint.String})
	}
	types, e := r.q.CategoriesListSubcategories(ctx)
	if e != nil {
		return out, e
	}
	for _, v := range types {
		var config masterDomain.CategoryConfiguration
		if e = masterDTO.UnmarshalMasterJSON(v.Configuration, &config); e != nil {
			return out, e
		}
		out.Subcategories = append(out.Subcategories, masterDomain.Category{ID: v.ID.String(), Code: v.Code, Name: v.Name, Active: v.IsActive, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: masterDTO.Actor(v.CreatedByID), UpdatedBy: masterDTO.Actor(v.UpdatedByID), Fingerprint: v.SourceFingerprint.String, CategoryID: v.CategoryID.String(), UnitID: v.UnitID.String(), Configuration: &config})
	}
	return out, nil
}
func (r *categoryRepository) Insert(ctx context.Context, kind string, in masterDomain.CategoryImportRecord) error {
	v := in.Record
	var source []byte
	var e error
	if in.Source != nil {
		source, e = masterDTO.MarshalMasterJSON(in.Source)
		if e != nil {
			return e
		}
	}
	fp := pgtype.Text{String: v.Fingerprint, Valid: v.Fingerprint != ""}
	switch kind {
	case "category":
		e = r.q.CategoriesInsertCategory(ctx, db.CategoriesInsertCategoryParams{ID: masterDTO.UUID(v.ID), Code: v.Code, Name: v.Name, IsActive: v.Active, ActorID: masterDTO.UUID(in.Actor), LegacySource: source, SourceFingerprint: fp, UnitID: masterDTO.UUID(v.UnitID)})
	case "unit":
		e = r.q.CategoriesInsertUnit(ctx, db.CategoriesInsertUnitParams{ID: masterDTO.UUID(v.ID), Code: v.Code, Name: v.Name, IsActive: v.Active, ActorID: masterDTO.UUID(in.Actor), LegacySource: source, SourceFingerprint: fp})
	case "subcategory":
		var config []byte
		config, e = masterDTO.MarshalMasterJSON(v.Configuration)
		if e != nil {
			return e
		}
		e = r.q.CategoriesInsertSubcategory(ctx, db.CategoriesInsertSubcategoryParams{ID: masterDTO.UUID(v.ID), Code: v.Code, Name: v.Name, IsActive: v.Active, ActorID: masterDTO.UUID(in.Actor), LegacySource: source, SourceFingerprint: fp, CategoryID: masterDTO.UUID(v.CategoryID), UnitID: masterDTO.UUID(v.UnitID), Configuration: config})
	default:
		return masterDomain.CategoryInvalid("Unknown catalogue record kind")
	}
	if e != nil {
		var pg *pgconn.PgError
		if errors.As(e, &pg) && (pg.Code == "23505" || pg.Code == "23503") {
			return masterDomain.CategoryConflict
		}
		return e
	}
	action := "import"
	if in.Reason != "" {
		action = "create"
		all, e := r.List(ctx)
		if e != nil {
			return e
		}
		list := all.Units
		if kind == "category" {
			list = all.Categories
		} else if kind == "subcategory" {
			list = all.Subcategories
		}
		for _, row := range list {
			if row.ID == v.ID {
				v = row
				break
			}
		}
		snapshot, e := masterDTO.MarshalMasterJSON(v)
		if e != nil {
			return e
		}
		if e = r.q.CategoriesInsertCreationHistory(ctx, db.CategoriesInsertCreationHistoryParams{ID: masterDTO.UUID(CategoryValues{}.ID()), Kind: kind, SubjectID: masterDTO.UUID(v.ID), Action: "create", BeforeSnapshot: []byte("{}"), Reason: in.Reason, AfterSnapshot: snapshot, ActorID: masterDTO.UUID(in.Actor)}); e != nil {
			return e
		}
	}
	id := CategoryValues{}.ID()
	return r.journal.Record(ctx, events.Event{ID: id, Module: "category_catalogue", EntityType: kind, EntityID: v.ID, EntityVersion: 1, Action: action, ActorID: in.Actor, ActorKind: "user", CorrelationID: id, ScopeKind: "platform", Visibility: "restricted", OccurredAt: time.Now().UTC()}, nil)
}

type CategoryValues struct{}

func (CategoryValues) ID() string {
	return newRecordID()
}

func (CategoryValues) Fingerprint(s masterDomain.CategorySource) string {
	b, _ := masterDTO.MarshalMasterJSON(s)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (r *categoryRepository) Update(ctx context.Context, kind string, in masterDomain.CategoryCorrection) error {
	v := in.After
	var count int64
	var e error
	switch kind {
	case "category":
		count, e = r.q.CategoriesUpdateCategory(ctx, db.CategoriesUpdateCategoryParams{ID: masterDTO.UUID(v.ID), Code: v.Code, Name: v.Name, ActorID: masterDTO.UUID(in.Actor), ExpectedVersion: in.Before.Version})
	case "subcategory":
		count, e = r.q.CategoriesUpdateSubcategory(ctx, db.CategoriesUpdateSubcategoryParams{ID: masterDTO.UUID(v.ID), Code: v.Code, Name: v.Name, ActorID: masterDTO.UUID(in.Actor), ExpectedVersion: in.Before.Version})
	case "unit":
		count, e = r.q.CategoriesUpdateUnit(ctx, db.CategoriesUpdateUnitParams{ID: masterDTO.UUID(v.ID), Code: v.Code, Name: v.Name, ActorID: masterDTO.UUID(in.Actor), ExpectedVersion: in.Before.Version})
	default:
		return masterDomain.CategoryInvalid("Unknown catalogue record kind")
	}
	if e != nil {
		var pg *pgconn.PgError
		if errors.As(e, &pg) && pg.Code == "23505" {
			return masterDomain.CategoryConflict
		}
		return e
	}
	if count != 1 {
		return masterDomain.CategoryStale
	}
	all, e := r.List(ctx)
	if e != nil {
		return e
	}
	list := all.Units
	if kind == "category" {
		list = all.Categories
	} else if kind == "subcategory" {
		list = all.Subcategories
	}
	for _, row := range list {
		if row.ID == v.ID {
			v = row
			break
		}
	}
	before, e := masterDTO.MarshalMasterJSON(in.Before)
	if e != nil {
		return e
	}
	after, e := masterDTO.MarshalMasterJSON(v)
	if e != nil {
		return e
	}
	if e = r.q.CategoriesInsertCreationHistory(ctx, db.CategoriesInsertCreationHistoryParams{ID: masterDTO.UUID(CategoryValues{}.ID()), Kind: kind, SubjectID: masterDTO.UUID(v.ID), Action: "update", Reason: in.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: masterDTO.UUID(in.Actor)}); e != nil {
		return e
	}
	id := CategoryValues{}.ID()
	return r.journal.Record(ctx, events.Event{ID: id, Module: "category_catalogue", EntityType: kind, EntityID: v.ID, EntityVersion: int64(v.Version), Action: "update", ActorID: in.Actor, ActorKind: "user", CorrelationID: id, ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.UpdatedAt}, nil)
}
