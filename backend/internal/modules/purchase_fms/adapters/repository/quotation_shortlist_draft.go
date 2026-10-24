package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func quotationDraft(v sqlc.PurchaseQuotationShortlistDraft) (d domain.QuotationShortlistDraft, e error) {
	d = domain.QuotationShortlistDraft{Active: v.IsActive, ProjectID: v.ProjectID.String(), BOID: v.BoID.String(), GroupID: v.GroupID.String(), Version: v.Version, Comment: v.Comment, UpdatedBy: v.UpdatedByID.String(), UpdatedAt: v.UpdatedAt.Time}
	e = json.Unmarshal(v.Proposals, &d.Proposals)
	return
}
func (r *quotationApprovalsRepository) Drafts(ctx context.Context, p, b string) ([]domain.QuotationShortlistDraft, error) {
	rows, e := r.q.QuotationDraftList(ctx, sqlc.QuotationDraftListParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationShortlistDraft{}
	for _, row := range rows {
		d, e := quotationDraft(row)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, nil
}
func (r *quotationApprovalsRepository) draftHistory(ctx context.Context, row sqlc.PurchaseQuotationShortlistDraft, action, actor string) error {
	d, e := quotationDraft(row)
	if e != nil {
		return e
	}
	raw, e := json.Marshal(d)
	if e != nil {
		return e
	}
	id := uuid.NewString()
	if e = r.q.QuotationDraftHistory(ctx, sqlc.QuotationDraftHistoryParams{ID: dtos.UUID(id), ProjectID: row.ProjectID, BoID: row.BoID, GroupID: row.GroupID, Version: row.Version, Action: action, Snapshot: raw, ActorID: dtos.UUID(actor)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: id, Module: "purchase_fms", EntityType: "quotation_shortlist_draft", EntityID: d.GroupID, EntityVersion: int64(d.Version), Action: action, ActorID: actor, ActorKind: "user", CorrelationID: id, ScopeKind: "project", ScopeID: d.ProjectID, Visibility: "restricted", OccurredAt: d.UpdatedAt}, nil)
}
func (r *quotationApprovalsRepository) SaveDraft(ctx context.Context, d domain.QuotationShortlistDraft, actor string) (domain.QuotationShortlistDraft, error) {
	raw, e := json.Marshal(d.Proposals)
	if e != nil {
		return d, e
	}
	var row sqlc.PurchaseQuotationShortlistDraft
	if d.Version == 0 {
		row, e = r.q.QuotationDraftInsert(ctx, sqlc.QuotationDraftInsertParams{ProjectID: dtos.UUID(d.ProjectID), BoID: dtos.UUID(d.BOID), GroupID: dtos.UUID(d.GroupID), Proposals: raw, Comment: d.Comment, ActorID: dtos.UUID(actor)})
	} else {
		row, e = r.q.QuotationDraftUpdate(ctx, sqlc.QuotationDraftUpdateParams{ProjectID: dtos.UUID(d.ProjectID), BoID: dtos.UUID(d.BOID), GroupID: dtos.UUID(d.GroupID), Proposals: raw, Comment: d.Comment, ActorID: dtos.UUID(actor), ExpectedVersion: d.Version})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return d, domain.QuotationApprovalsStale
	}
	if e != nil {
		return d, e
	}
	if e = r.draftHistory(ctx, row, "saved", actor); e != nil {
		return d, e
	}
	return quotationDraft(row)
}
func (r *quotationApprovalsRepository) CompleteDraft(ctx context.Context, p, b, g, actor string) error {
	rows, e := r.q.QuotationDraftComplete(ctx, sqlc.QuotationDraftCompleteParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b), GroupID: dtos.UUID(g), ActorID: dtos.UUID(actor)})
	if e != nil {
		return e
	}
	for _, row := range rows {
		if e = r.draftHistory(ctx, row, "submitted", actor); e != nil {
			return e
		}
	}
	return nil
}
