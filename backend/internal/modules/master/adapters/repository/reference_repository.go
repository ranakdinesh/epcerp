package repository

import (
	"context"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type ReferenceUnitOfWork struct{ manager *database.Manager }

func NewReferenceUnitOfWork(b database.Beginner) *ReferenceUnitOfWork {
	return &ReferenceUnitOfWork{database.NewManager(b)}
}
func (u *ReferenceUnitOfWork) Within(ctx context.Context, work func(context.Context, masterPorts.ReferenceRepository) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error { return work(ctx, NewReferenceRepository(c)) })
}

type ReferenceRepository struct{ queries *db.Queries }

var _ masterPorts.ReferenceRepository = (*ReferenceRepository)(nil)

func NewReferenceRepository(c database.Connection) *ReferenceRepository {
	return &ReferenceRepository{NewStore(c).Queries}
}

func (r *ReferenceRepository) Get(ctx context.Context, k, id string) (*masterDomain.Reference, error) {
	u := masterDTO.UUID(id)
	if !u.Valid {
		return nil, masterDomain.ReferenceInvalid("Input should be a valid UUID")
	}
	v, e := r.queries.ReferencesGetReference(ctx, db.ReferencesGetReferenceParams{ReferenceID: u, Kind: k})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	record := masterDTO.ReferenceFromRow(v)
	return &record, nil
}
func (r *ReferenceRepository) List(ctx context.Context, k, state string, activeOnly bool) ([]masterDomain.Reference, error) {
	rows, e := r.queries.ReferencesListReferences(ctx, db.ReferencesListReferencesParams{Kind: k, StateFilter: masterDTO.UUID(state), ActiveOnly: activeOnly})
	if e != nil {
		return nil, e
	}
	records := []masterDomain.Reference{}
	for _, v := range rows {
		records = append(records, masterDTO.ReferenceFromRow(db.ReferencesGetReferenceRow(v)))
	}
	return records, nil
}
func (r *ReferenceRepository) LockMutations(ctx context.Context) error {
	return r.queries.ReferencesLockMutations(ctx)
}
func (r *ReferenceRepository) RetireCities(ctx context.Context, id, actorID string) error {
	return r.queries.ReferencesRetireCities(ctx, db.ReferencesRetireCitiesParams{StateID: masterDTO.UUID(id), ActorID: masterDTO.UUID(actorID)})
}
func referenceConflict(e error) error {
	var pg *pgconn.PgError
	if errors.As(e, &pg) && (pg.Code == "23505" || pg.Code == "23503") {
		return masterDomain.ReferenceDuplicate
	}
	return e
}
func (r *ReferenceRepository) Save(ctx context.Context, k string, v masterDomain.Reference, create bool) error {
	var e error
	if create {
		switch k {
		case "client-types":
			e = r.queries.ReferencesInsertClientType(ctx, db.ReferencesInsertClientTypeParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, IsActive: v.Active, CreatedByID: masterDTO.UUID(*v.CreatedBy), UpdatedByID: masterDTO.UUID(*v.UpdatedBy)})
		case "industries":
			e = r.queries.ReferencesInsertIndustry(ctx, db.ReferencesInsertIndustryParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, IsActive: v.Active, CreatedByID: masterDTO.UUID(*v.CreatedBy), UpdatedByID: masterDTO.UUID(*v.UpdatedBy)})
		case "states":
			e = r.queries.ReferencesInsertState(ctx, db.ReferencesInsertStateParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, IsActive: v.Active, CreatedByID: masterDTO.UUID(*v.CreatedBy), UpdatedByID: masterDTO.UUID(*v.UpdatedBy)})
		case "cities":
			e = r.queries.ReferencesInsertCity(ctx, db.ReferencesInsertCityParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, IsActive: v.Active, CreatedByID: masterDTO.UUID(*v.CreatedBy), UpdatedByID: masterDTO.UUID(*v.UpdatedBy), StateID: masterDTO.UUID(v.StateID)})
		default:
			return masterDomain.ReferenceUnknown
		}
	} else {
		switch k {
		case "client-types":
			e = r.queries.ReferencesUpdateClientType(ctx, db.ReferencesUpdateClientTypeParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, ActorID: masterDTO.UUID(*v.UpdatedBy)})
		case "industries":
			e = r.queries.ReferencesUpdateIndustry(ctx, db.ReferencesUpdateIndustryParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, ActorID: masterDTO.UUID(*v.UpdatedBy)})
		case "states":
			e = r.queries.ReferencesUpdateState(ctx, db.ReferencesUpdateStateParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, ActorID: masterDTO.UUID(*v.UpdatedBy)})
		case "cities":
			e = r.queries.ReferencesUpdateCity(ctx, db.ReferencesUpdateCityParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, ActorID: masterDTO.UUID(*v.UpdatedBy), StateID: masterDTO.UUID(v.StateID)})
		default:
			return masterDomain.ReferenceUnknown
		}
	}
	return referenceConflict(e)
}
func (r *ReferenceRepository) Status(ctx context.Context, k, id, actorID string, active bool) error {
	switch k {
	case "client-types":
		return r.queries.ReferencesStatusClientType(ctx, db.ReferencesStatusClientTypeParams{ID: masterDTO.UUID(id), ActorID: masterDTO.UUID(actorID), Active: active})
	case "industries":
		return r.queries.ReferencesStatusIndustry(ctx, db.ReferencesStatusIndustryParams{ID: masterDTO.UUID(id), ActorID: masterDTO.UUID(actorID), Active: active})
	case "states":
		return r.queries.ReferencesStatusState(ctx, db.ReferencesStatusStateParams{ID: masterDTO.UUID(id), ActorID: masterDTO.UUID(actorID), Active: active})
	case "cities":
		return r.queries.ReferencesStatusCity(ctx, db.ReferencesStatusCityParams{ID: masterDTO.UUID(id), ActorID: masterDTO.UUID(actorID), Active: active})
	default:
		return masterDomain.ReferenceUnknown
	}
}

type ReferenceSystemValues struct{}

func (ReferenceSystemValues) ID() string {
	return newRecordID()
}
