package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type ManufacturerUnitOfWork struct {
	manager   *database.Manager
	catalogue func(database.Connection) masterContracts.CategoryEligibilityReader
	events    func(database.Connection) events.Writer
}

func NewManufacturerUnitOfWork(b database.Beginner, c func(database.Connection) masterContracts.CategoryEligibilityReader, e func(database.Connection) events.Writer) *ManufacturerUnitOfWork {
	return &ManufacturerUnitOfWork{database.NewManager(b), c, e}
}
func (u *ManufacturerUnitOfWork) Within(ctx context.Context, work func(context.Context, masterPorts.ManufacturerRepository, masterPorts.ManufacturerEligibility) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return work(ctx, &manufacturerRepository{NewStore(c).Queries, u.events(c)}, &manufacturerEligibility{u.catalogue(c)})
	})
}

type manufacturerEligibility struct {
	reader masterContracts.CategoryEligibilityReader
}

func (e *manufacturerEligibility) Validate(ctx context.Context, data masterDomain.ManufacturerData) error {
	records, err := e.reader.FindEligibility(ctx, data.Eligible)
	if err != nil {
		return err
	}
	active := []string{}
	counts := map[string]int{}
	for _, v := range records {
		counts[v.Code]++
		if v.Active {
			active = append(active, v.Code)
		}
	}
	for _, code := range data.Eligible {
		if counts[code] != 1 {
			return masterDomain.ManufacturerInvalid("Eligibility code is missing or ambiguous")
		}
	}
	return masterDomain.ManufacturerValidateEligibility(data, active)
}
func (e *manufacturerEligibility) Options(ctx context.Context, search string, codes []string, skip int32, kind, category string) (masterDomain.ManufacturerSelection, error) {
	var rows []masterContracts.CategoryEligibilityChoice
	var total int64
	var err error
	if len(codes) > 0 {
		rows, err = e.reader.FindEligibility(ctx, codes)
		total = int64(len(rows))
	} else {
		rows, total, err = e.reader.BrowseEligibility(ctx, search, kind, category, skip)
	}
	out := masterDomain.ManufacturerSelection{Rows: []masterDomain.ManufacturerChoice{}, Total: total}
	for _, v := range rows {
		out.Rows = append(out.Rows, masterDomain.ManufacturerChoice{Code: v.Code, Name: v.Name, Kind: v.Kind, ParentName: v.ParentName, ParentCode: v.ParentCode, Active: v.Active})
	}
	return out, err
}

type manufacturerRepository struct {
	q      *db.Queries
	events events.Writer
}

var _ masterPorts.ManufacturerRepository = (*manufacturerRepository)(nil)

func manufacturerConflict(e error) error {
	var pg *pgconn.PgError
	if errors.As(e, &pg) && (pg.Code == "23505" || pg.Code == "23503") {
		return masterDomain.ManufacturerConflict
	}
	return e
}
func (r *manufacturerRepository) Lock(ctx context.Context) error {
	return r.q.ManufacturersLockNames(ctx)
}
func (r *manufacturerRepository) List(ctx context.Context) ([]masterDomain.Manufacturer, error) {
	rows, e := r.q.ManufacturersListManufacturers(ctx)
	if e != nil {
		return nil, e
	}
	records := []masterDomain.Manufacturer{}
	for _, v := range rows {
		c, e := masterDTO.ManufacturerFromRow(v)
		if e != nil {
			return nil, e
		}
		records = append(records, c)
	}
	return records, nil
}
func (r *manufacturerRepository) Get(ctx context.Context, id string, lock bool) (*masterDomain.Manufacturer, error) {
	var row db.Manufacturer
	var e error
	if lock {
		row, e = r.q.ManufacturersLockManufacturer(ctx, masterDTO.UUID(id))
	} else {
		row, e = r.q.ManufacturersGetManufacturer(ctx, masterDTO.UUID(id))
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := masterDTO.ManufacturerFromRow(row)
	return &v, e
}
func (r *manufacturerRepository) Create(ctx context.Context, v masterDomain.Manufacturer) (masterDomain.Manufacturer, error) {
	data, e := masterDTO.MarshalMasterJSON(v.Data)
	if e != nil {
		return masterDomain.Manufacturer{}, e
	}
	row, e := r.q.ManufacturersInsertManufacturer(ctx, db.ManufacturersInsertManufacturerParams{ID: masterDTO.UUID(v.ID), Code: v.Code, Name: v.Name, Data: data, IsActive: v.Active, ActorID: masterDTO.UUID(*v.CreatedBy)})
	if e != nil {
		return masterDomain.Manufacturer{}, manufacturerConflict(e)
	}
	return masterDTO.ManufacturerFromRow(row)
}
func (r *manufacturerRepository) Update(ctx context.Context, v masterDomain.Manufacturer, profile bool) (masterDomain.Manufacturer, error) {
	var row db.Manufacturer
	var e error
	if profile {
		data, err := masterDTO.MarshalMasterJSON(v.Data)
		if err != nil {
			return masterDomain.Manufacturer{}, err
		}
		row, e = r.q.ManufacturersProfileManufacturer(ctx, db.ManufacturersProfileManufacturerParams{ID: masterDTO.UUID(v.ID), Name: v.Name, Data: data, Version: v.Version, ActorID: masterDTO.UUID(*v.UpdatedBy)})
	} else {
		row, e = r.q.ManufacturersStatusManufacturer(ctx, db.ManufacturersStatusManufacturerParams{ID: masterDTO.UUID(v.ID), Active: v.Active, Version: v.Version, ActorID: masterDTO.UUID(*v.UpdatedBy)})
	}
	if e != nil {
		return masterDomain.Manufacturer{}, manufacturerConflict(e)
	}
	return masterDTO.ManufacturerFromRow(row)
}
func (r *manufacturerRepository) Event(ctx context.Context, v masterDomain.ManufacturerEvent) error {
	before, e := masterDTO.MarshalMasterJSON(v.Before)
	if e != nil {
		return e
	}
	after, e := masterDTO.MarshalMasterJSON(v.After)
	if e != nil {
		return e
	}
	if e = r.q.ManufacturersInsertEvent(ctx, db.ManufacturersInsertEventParams{ID: masterDTO.UUID(v.ID), SubjectID: masterDTO.UUID(v.SubjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: masterDTO.UUID(*v.Actor)}); e != nil {
		return e
	}
	source, e := r.q.ManufacturersGetManufacturer(ctx, masterDTO.UUID(v.SubjectID))
	if e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: v.ID, Module: "manufacturers", EntityType: "manufacturer", EntityID: v.SubjectID, EntityVersion: int64(source.Version), Action: v.Action, ActorID: *v.Actor, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "platform", Visibility: "restricted", OccurredAt: source.UpdatedAt.Time}, nil)
}
func (r *manufacturerRepository) History(ctx context.Context, id string) ([]masterDomain.ManufacturerEvent, error) {
	rows, e := r.q.ManufacturersHistory(ctx, masterDTO.UUID(id))
	if e != nil {
		return nil, e
	}
	events := []masterDomain.ManufacturerEvent{}
	for _, v := range rows {
		before, e := masterDTO.ManufacturerDataFromJSON(v.Before)
		if e != nil {
			return nil, e
		}
		after, e := masterDTO.ManufacturerDataFromJSON(v.After)
		if e != nil {
			return nil, e
		}
		events = append(events, masterDomain.ManufacturerEvent{ID: v.ID.String(), SubjectID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, Before: before, After: after, CreatedAt: v.CreatedAt.Time.UTC(), Actor: masterDTO.Actor(v.CreatedByID)})
	}
	return events, nil
}

type ManufacturerSystemValues struct{}

func (ManufacturerSystemValues) ID() string {
	return newRecordID()
}

func (r *manufacturerRepository) FindImport(ctx context.Context, source string) (*masterDomain.Manufacturer, string, error) {
	v, e := r.q.ManufacturersFindImport(ctx, masterDTO.UUID(source))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, "", nil
	}
	if e != nil {
		return nil, "", e
	}
	record, e := r.Get(ctx, v.GoID.String(), false)
	return record, v.SourceFingerprint, e
}
func (r *manufacturerRepository) RecordImport(ctx context.Context, id string, source masterDomain.ManufacturerLegacy, fingerprint, actor string) error {
	raw, e := masterDTO.MarshalMasterJSON(source)
	if e != nil {
		return e
	}
	return r.q.ManufacturersInsertImport(ctx, db.ManufacturersInsertImportParams{SourceID: masterDTO.UUID(source.Source.ID), GoID: masterDTO.UUID(id), SourceSnapshot: raw, SourceFingerprint: fingerprint, ActorID: masterDTO.UUID(actor)})
}
func (ManufacturerSystemValues) Fingerprint(v masterDomain.ManufacturerLegacy) string {
	raw, _ := masterDTO.MarshalMasterJSON(v)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
