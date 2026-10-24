package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type QuotationsFactory func(database.Connection) QuotationsProviders
type QuotationsUnitOfWork = unitOfWork[ports.QuotationsRepository, ports.QuotationsContext]

func NewQuotationsUnitOfWork(b database.Beginner, p QuotationsFactory, j func(database.Connection) events.Writer) *QuotationsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.QuotationsRepository, ports.QuotationsContext) {
		return &quotationsRepository{NewStore(connection), j(connection)}, &quotationsQuoteContext{p(connection)}
	})
}

type quotationsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *quotationsRepository) Get(ctx context.Context, p, b string) (*domain.QuotationsRecord, error) {
	row, e := r.q.QuotationsGetRecord(ctx, sqlc.QuotationsGetRecordParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := dtos.QuotationsRecord(row)
	return &v, e
}
func (r *quotationsRepository) Save(ctx context.Context, v domain.QuotationsRecord, a string) (domain.QuotationsRecord, error) {
	data, e := json.Marshal(v.Quotes)
	if e != nil {
		return v, e
	}
	var row sqlc.PurchaseGroupQuotation
	if v.Version == 1 {
		row, e = r.q.QuotationsInsertRecord(ctx, sqlc.QuotationsInsertRecordParams{BoID: dtos.UUID(v.BOID), ProjectID: dtos.UUID(v.ProjectID), Quotes: data, ActorID: dtos.UUID(a)})
	} else {
		row, e = r.q.QuotationsUpdateRecord(ctx, sqlc.QuotationsUpdateRecordParams{BoID: dtos.UUID(v.BOID), ProjectID: dtos.UUID(v.ProjectID), Quotes: data, ActorID: dtos.UUID(a), ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.QuotationsStale
	}
	if e != nil {
		return v, e
	}
	return dtos.QuotationsRecord(row)
}
func (r *quotationsRepository) Event(ctx context.Context, v domain.QuotationsEvent) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	old := map[string]int32{}
	if v.Before != nil {
		for _, q := range v.Before.Quotes {
			old[q.DocumentID] = q.Revision
		}
	}
	for _, q := range v.After.Quotes {
		if q.Revision <= old[q.DocumentID] {
			continue
		}
		data, e := json.Marshal(q)
		if e != nil {
			return e
		}
		var attachment pgtype.UUID
		if q.AttachmentID != nil {
			attachment = dtos.UUID(*q.AttachmentID)
		}
		if e = r.q.QuotationsInsertRevision(ctx, sqlc.QuotationsInsertRevisionParams{DocumentID: dtos.UUID(q.DocumentID), Revision: q.Revision, BoID: dtos.UUID(v.BOID), AttachmentID: attachment, Quote: data, ActorID: dtos.UUID(v.ActorID)}); e != nil {
			return e
		}
	}
	if e = r.q.QuotationsInsertEvent(ctx, sqlc.QuotationsInsertEventParams{ID: dtos.UUID(v.ID), BoID: dtos.UUID(v.BOID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: dtos.UUID(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "group_quotations", EntityType: "purchase_group_quotations", EntityID: v.BOID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.After.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *quotationsRepository) History(ctx context.Context, p, b string) ([]domain.QuotationsEvent, error) {
	rows, e := r.q.QuotationsHistory(ctx, sqlc.QuotationsHistoryParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationsEvent{}
	for _, row := range rows {
		v := domain.QuotationsEvent{ID: row.ID.String(), BOID: row.BoID.String(), Action: row.Action, Reason: row.Reason, ActorID: row.CreatedByID.String(), CreatedAt: row.CreatedAt.Time}
		if e = json.Unmarshal(row.BeforeSnapshot, &v.Before); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(row.AfterSnapshot, &v.After); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *quotationsRepository) Revisions(ctx context.Context, p, b string) ([]domain.QuotationsQuote, error) {
	rows, e := r.q.QuotationsRevisions(ctx, sqlc.QuotationsRevisionsParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationsQuote{}
	for _, row := range rows {
		var v domain.QuotationsQuote
		if e = json.Unmarshal(row, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *quotationsRepository) Attachments(ctx context.Context, d string) ([]domain.QuotationsAttachment, error) {
	rows, e := r.q.QuotationsAttachments(ctx, dtos.UUID(d))
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationsAttachment{}
	for _, row := range rows {
		out = append(out, domain.QuotationsAttachment{ID: row.ID.String(), DocumentID: row.DocumentID.String(), Filename: row.Filename, ContentType: row.ContentType, ByteSize: row.ByteSize, SHA256: row.Sha256, Active: row.IsActive, CreatedAt: row.CreatedAt.Time, CreatedBy: row.CreatedByID.String()})
	}
	return out, nil
}
func (r *quotationsRepository) Attachment(ctx context.Context, d, id string) (*domain.QuotationsAttachment, error) {
	row, e := r.q.QuotationsAttachment(ctx, sqlc.QuotationsAttachmentParams{DocumentID: dtos.UUID(d), ID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v := dtos.QuotationsAttachment(row)
	return &v, nil
}
func (r *quotationsRepository) InsertAttachment(ctx context.Context, v domain.QuotationsAttachment) (domain.QuotationsAttachment, error) {
	row, e := r.q.QuotationsInsertAttachment(ctx, sqlc.QuotationsInsertAttachmentParams{ID: dtos.UUID(v.ID), DocumentID: dtos.UUID(v.DocumentID), Filename: v.Filename, ContentType: v.ContentType, ByteSize: v.ByteSize, Sha256: v.SHA256, Ciphertext: v.Ciphertext, ActorID: dtos.UUID(v.CreatedBy)})
	if e != nil {
		return v, e
	}
	return dtos.QuotationsAttachment(row), nil
}
func (r *quotationsRepository) AttachmentEvent(ctx context.Context, v domain.QuotationsAttachment, id, action, a string) error {
	if e := r.q.QuotationsEvidenceEvent(ctx, sqlc.QuotationsEvidenceEventParams{ID: dtos.UUID(id), AttachmentID: dtos.UUID(v.ID), Action: action, ActorID: dtos.UUID(a)}); e != nil {
		return e
	}
	scope, e := r.q.QuotationsEvidenceScope(ctx, dtos.UUID(v.DocumentID))
	if e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: id, Module: "group_quotations", EntityType: "purchase_quotation_attachment", EntityID: v.ID, EntityVersion: 1, Action: action, ActorID: a, ActorKind: "user", CorrelationID: id, ScopeKind: "project", ScopeID: scope.ProjectID.String(), Visibility: "restricted", OccurredAt: time.Now().UTC()}, nil)
}

var _ ports.QuotationsRepository = (*quotationsRepository)(nil)

func (r *quotationsRepository) PendingDocument(ctx context.Context, p, b, d string) (bool, error) {
	return r.q.QuotationPendingDocument(ctx, sqlc.QuotationPendingDocumentParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b), DocumentID: dtos.UUID(d)})
}
