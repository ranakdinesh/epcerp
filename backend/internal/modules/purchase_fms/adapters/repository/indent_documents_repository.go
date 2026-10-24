package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type IndentDocumentsFactory func(database.Connection) IndentDocumentsProviders
type IndentDocumentsUnitOfWork = unitOfWork[ports.IndentDocumentsRepository, ports.IndentDocumentsContext]

func NewIndentDocumentsUnitOfWork(b database.Beginner, p IndentDocumentsFactory, j func(database.Connection) events.Writer) *IndentDocumentsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.IndentDocumentsRepository, ports.IndentDocumentsContext) {
		return &indentDocumentsRepository{NewStore(connection), j(connection)}, &indentDocumentsDocumentContext{p(connection)}
	})
}

type indentDocumentsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *indentDocumentsRepository) List(ctx context.Context, p, b string) ([]domain.IndentDocumentsDocument, error) {
	rows, e := r.q.IndentDocumentsListDocuments(ctx, sqlc.IndentDocumentsListDocumentsParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if e != nil {
		return nil, e
	}
	legacyRows, e := r.q.IndentDocumentsLegacyTerms(ctx, sqlc.IndentDocumentsLegacyTermsParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if e != nil {
		return nil, e
	}
	legacy := map[string]*domain.IndentDocumentsTerms{}
	for _, row := range legacyRows {
		var terms domain.IndentDocumentsTerms
		if e = json.Unmarshal(row.Terms, &terms); e != nil {
			return nil, e
		}
		legacy[row.DocumentID.String()] = &terms
	}
	out := []domain.IndentDocumentsDocument{}
	for _, row := range rows {
		v, e := dtos.IndentDocumentsView(row)
		if e != nil {
			return nil, e
		}
		if v.Data.Terms == nil {
			v.Data.Terms = legacy[v.ID]
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *indentDocumentsRepository) Save(ctx context.Context, v domain.IndentDocumentsDocument, a string) (domain.IndentDocumentsDocument, error) {
	v.Data.IsCurrent = false
	// Supplemental legacy terms are a read projection, not a source-JSON rewrite.
	stored := v.Data
	if stored.Terms != nil && stored.Terms.Source == "legacy_template_v1" {
		stored.Terms = nil
	}
	data, e := json.Marshal(stored)
	if e != nil {
		return v, e
	}
	var row sqlc.PurchaseIndentDocument
	if v.Version == 1 {
		row, e = r.q.IndentDocumentsInsertDocument(ctx, sqlc.IndentDocumentsInsertDocumentParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), BoID: dtos.UUID(v.BOID), GroupID: dtos.UUID(v.GroupID), VendorID: dtos.UUID(v.VendorID), ScopeRevision: v.Data.ScopeRevision, Data: data, ActorID: dtos.UUID(a)})
	} else {
		row, e = r.q.IndentDocumentsUpdateDocument(ctx, sqlc.IndentDocumentsUpdateDocumentParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), BoID: dtos.UUID(v.BOID), ExpectedVersion: v.Version - 1, Data: data, ActorID: dtos.UUID(a)})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.IndentDocumentsStale
	}
	if e != nil {
		return v, e
	}
	saved, e := dtos.IndentDocumentsView(row)
	if e == nil && saved.Data.Terms == nil {
		saved.Data.Terms = v.Data.Terms
	}
	return saved, e
}
func (r *indentDocumentsRepository) Event(ctx context.Context, v domain.IndentDocumentsEvent) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	if e = r.q.IndentDocumentsInsertDocumentEvent(ctx, sqlc.IndentDocumentsInsertDocumentEventParams{ID: dtos.UUID(v.ID), DocumentID: dtos.UUID(v.DocumentID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: dtos.UUID(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "purchase_indent_documents", EntityType: "purchase_indent_document", EntityID: v.DocumentID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.After.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *indentDocumentsRepository) History(ctx context.Context, p, b string) ([]domain.IndentDocumentsEvent, error) {
	rows, e := r.q.IndentDocumentsDocumentHistory(ctx, sqlc.IndentDocumentsDocumentHistoryParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if e != nil {
		return nil, e
	}
	out := []domain.IndentDocumentsEvent{}
	for _, row := range rows {
		v := domain.IndentDocumentsEvent{ID: row.ID.String(), DocumentID: row.DocumentID.String(), Action: row.Action, Reason: row.Reason, ActorID: row.CreatedByID.String(), CreatedAt: row.CreatedAt.Time}
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
func (r *indentDocumentsRepository) Notify(ctx context.Context, d domain.IndentDocumentsDocument, recipients []string, a string) error {
	for _, id := range recipients {
		if e := r.q.IndentDocumentsCreateDispatchNotice(ctx, sqlc.IndentDocumentsCreateDispatchNoticeParams{ID: dtos.UUID((Values{}).ID()), DocumentID: dtos.UUID(d.ID), RecipientID: dtos.UUID(id), ActorID: dtos.UUID(a)}); e != nil {
			return e
		}
	}
	return nil
}
func (r *indentDocumentsRepository) Resolve(ctx context.Context, id, a string) error {
	return r.q.IndentDocumentsResolveDispatchNotices(ctx, sqlc.IndentDocumentsResolveDispatchNoticesParams{DocumentID: dtos.UUID(id), ActorID: dtos.UUID(a)})
}
func (r *indentDocumentsRepository) Notices(ctx context.Context, a string) ([]domain.IndentDocumentsNotice, error) {
	rows, e := r.q.IndentDocumentsDispatchNotices(ctx, dtos.UUID(a))
	if e != nil {
		return nil, e
	}
	out := []domain.IndentDocumentsNotice{}
	for _, row := range rows {
		var d domain.IndentDocumentsData
		if e = json.Unmarshal(row.Data, &d); e != nil {
			return nil, e
		}
		out = append(out, domain.IndentDocumentsNotice{ID: row.ID.String(), ProjectID: row.ProjectID.String(), BOID: row.BoID.String(), Number: d.Number, Group: d.GroupName, Vendor: d.Vendor.Name, CreatedAt: row.CreatedAt.Time})
	}
	return out, nil
}

// Read-only publication includes the live current/superseded projection; frozen
// generated scope remains untouched in persistence and event history.
type IndentDocumentsReader struct {
	r      *indentDocumentsRepository
	groups contracts.IndentGroupsReader
}

func NewIndentDocumentsReader(c database.Connection, g contracts.IndentGroupsReader) *IndentDocumentsReader {
	return &IndentDocumentsReader{r: &indentDocumentsRepository{q: NewStore(c)}, groups: g}
}
func (r *IndentDocumentsReader) List(ctx context.Context, p, b string) ([]domain.IndentDocumentsDocument, error) {
	docs, e := r.r.List(ctx, p, b)
	if e != nil {
		return nil, e
	}
	record, e := r.groups.Get(ctx, p, b)
	if e != nil {
		return nil, e
	}
	revisions := map[string]int32{}
	if record != nil {
		for _, g := range record.Groups {
			revisions[g.ID] = g.ScopeRevision
		}
	}
	for i := range docs {
		docs[i].Data.IsCurrent = docs[i].Data.ScopeRevision == revisions[docs[i].GroupID]
	}
	return docs, nil
}
func (r *IndentDocumentsReader) Notices(ctx context.Context, recipient string) ([]domain.IndentDocumentsNotice, error) {
	return r.r.Notices(ctx, recipient)
}
