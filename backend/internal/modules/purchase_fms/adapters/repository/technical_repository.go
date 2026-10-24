package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type TechnicalContextFactory func(database.Connection) ports.TechnicalContext
type TechnicalUnitOfWork = unitOfWork[ports.TechnicalRepository, ports.TechnicalContext]

func NewTechnicalUnitOfWork(b database.Beginner, c TechnicalContextFactory, j func(database.Connection) events.Writer) *TechnicalUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.TechnicalRepository, ports.TechnicalContext) {
		return &technicalRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type technicalRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.TechnicalRepository = (*technicalRepository)(nil)

func (r *technicalRepository) Get(ctx context.Context, p, id string) (*domain.TechnicalCase, error) {
	v, e := r.q.TechnicalGetCase(ctx, sqlc.TechnicalGetCaseParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.TechnicalRecord(v)
	return &out, e
}
func (r *technicalRepository) Save(ctx context.Context, v domain.TechnicalCase) (domain.TechnicalCase, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.TechnicalCase{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.TechnicalCase{}, e
	}
	var row sqlc.PurchaseTechnicalCase
	if v.Version == 1 {
		row, e = r.q.TechnicalInsertCase(ctx, sqlc.TechnicalInsertCaseParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), SourceCaseID: dtos.UUID(v.Source.CaseID), State: v.State, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.TechnicalUpdateCase(ctx, sqlc.TechnicalUpdateCaseParams{ID: dtos.UUID(v.ID), Revision: v.Revision, State: v.State, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.TechnicalCase{}, domain.TechnicalStale
	}
	if e != nil {
		return domain.TechnicalCase{}, e
	}
	return dtos.TechnicalRecord(row)
}
func (r *technicalRepository) Event(ctx context.Context, e domain.TechnicalEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.TechnicalEvent(ctx, sqlc.TechnicalEventParams{ID: dtos.UUID(e.ID), CaseID: dtos.UUID(e.CaseID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_technical", EntityType: "technical_case", EntityID: e.CaseID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *technicalRepository) History(ctx context.Context, id string) ([]domain.TechnicalEvent, error) {
	rows, e := r.q.TechnicalHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.TechnicalEvent{}
	for _, v := range rows {
		row := domain.TechnicalEvent{ID: v.ID.String(), CaseID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
		if e = json.Unmarshal(v.BeforeSnapshot, &row.Before); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(v.AfterSnapshot, &row.After); e != nil {
			return nil, e
		}
		out = append(out, row)
	}
	return out, nil
}
func (r *technicalRepository) Files(ctx context.Context, id string) ([]domain.TechnicalFile, error) {
	rows, e := r.q.TechnicalFiles(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.TechnicalFile{}
	for _, v := range rows {
		out = append(out, domain.TechnicalFile{ID: v.ID.String(), CaseID: v.CaseID.String(), Purpose: v.Purpose, Name: v.Name, MediaType: v.MediaType, Size: v.ByteSize, SHA256: v.Sha256, Active: v.IsActive})
	}
	return out, nil
}
func (r *technicalRepository) Evidence(ctx context.Context, id, file string) (*domain.TechnicalEvidence, error) {
	v, e := r.q.TechnicalEvidence(ctx, sqlc.TechnicalEvidenceParams{CaseID: dtos.UUID(id), ID: dtos.UUID(file)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &domain.TechnicalEvidence{Bucket: v.ObjectBucket, ObjectKey: v.ObjectKey, File: domain.TechnicalFile{ID: v.ID.String(), CaseID: v.CaseID.String(), Purpose: v.Purpose, Name: v.Name, MediaType: v.MediaType, Size: v.ByteSize, SHA256: v.Sha256, Active: v.IsActive}, Ciphertext: v.Ciphertext, CreatedBy: v.CreatedByID.String(), CreatedAt: v.CreatedAt.Time.UTC()}, nil
}
func (r *technicalRepository) StoreEvidence(ctx context.Context, v domain.TechnicalEvidence) error {
	f := v.File
	return r.q.TechnicalStoreEvidence(ctx, sqlc.TechnicalStoreEvidenceParams{ID: dtos.UUID(f.ID), CaseID: dtos.UUID(f.CaseID), Purpose: f.Purpose, Name: f.Name, MediaType: f.MediaType, ByteSize: f.Size, Sha256: f.SHA256, Ciphertext: v.Ciphertext, ObjectBucket: v.Bucket, ObjectKey: v.ObjectKey, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
}
func (r *technicalRepository) EvidenceEvent(ctx context.Context, e domain.TechnicalFileEvent) error {
	if err := r.q.TechnicalEvidenceEvent(ctx, sqlc.TechnicalEvidenceEventParams{ID: dtos.UUID(e.ID), CaseID: dtos.UUID(e.CaseID), FileID: dtos.UUID(e.FileID), Action: e.Action, Reason: e.Reason, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	p, err := r.q.TechnicalCaseProject(ctx, dtos.UUID(e.CaseID))
	if err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_technical", EntityType: "technical_file_event", EntityID: e.ID, EntityVersion: 1, Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: p.String(), Visibility: "restricted", OccurredAt: e.At}, nil)
}
