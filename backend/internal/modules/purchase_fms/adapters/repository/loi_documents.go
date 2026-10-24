package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"strings"
	"time"
)

type loiDocumentsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (c *awardsContext) Specifications(ctx context.Context, id string) (string, error) {
	item, e := c.p.Indents.Items.Approved(ctx, id)
	if e != nil {
		return "", e
	}
	lines := []string{}
	for _, field := range item.Snapshot.Configuration.Fields {
		v := item.Specifications[field.Code]
		if v == "" {
			continue
		}
		for _, option := range field.Options {
			if option.Code == v {
				v = option.Label
				break
			}
		}
		lines = append(lines, field.Label+": "+v)
	}
	if item.Model != "" {
		lines = append(lines, "Model: "+item.Model)
	}
	return strings.Join(lines, "\n"), nil
}

func loiDocument(v sqlc.PurchaseLoiDocument) (out domain.LOIDocument, e error) {
	out = domain.LOIDocument{ID: v.ID.String(), ProjectID: v.ProjectID.String(), HandoffID: v.HandoffID.String(), AttemptID: v.AttemptID.String(), Version: v.Version, Reference: v.Reference, Date: v.DocumentDate.Time.Format("2006-01-02"), Format: v.Format, RequestHash: v.RequestHash, SHA256: v.Sha256, Bucket: v.ObjectBucket, ObjectKey: v.ObjectKey, Size: v.ByteSize, CreatedBy: v.CreatedByID.String(), CreatedAt: v.CreatedAt.Time}
	e = json.Unmarshal(v.Content, &out.Content)
	out.Notes = out.Content.Notes
	return
}
func (r *loiDocumentsRepository) List(ctx context.Context, p, a string) ([]domain.LOIDocument, error) {
	rows, e := r.q.LOIDocumentsList(ctx, sqlc.LOIDocumentsListParams{ProjectID: dtos.UUID(p), AttemptID: dtos.UUID(a)})
	if e != nil {
		return nil, e
	}
	out := []domain.LOIDocument{}
	for _, row := range rows {
		v, e := loiDocument(row)
		if e != nil {
			return nil, e
		}
		v.Review, e = r.Review(ctx, v.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *loiDocumentsRepository) Get(ctx context.Context, p, a, id string) (*domain.LOIDocument, error) {
	row, e := r.q.LOIDocumentsGet(ctx, sqlc.LOIDocumentsGetParams{ProjectID: dtos.UUID(p), AttemptID: dtos.UUID(a), ID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := loiDocument(row)
	if e == nil {
		v.Review, e = r.Review(ctx, v.ID)
	}
	return &v, e
}

func (r *loiDocumentsRepository) Review(ctx context.Context, id string) (*domain.LOIReview, error) {
	v, e := r.q.LOIReviewGet(ctx, dtos.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &domain.LOIReview{ID: v.ID.String(), DocumentID: v.DocumentID.String(), CreatedBy: v.CreatedByID.String(), Reviewer: v.Reviewer, CreatedAt: v.CreatedAt.Time, LOIReviewCommand: domain.LOIReviewCommand{DetailsCorrect: v.DetailsCorrect, PricesCorrect: v.PricesCorrect, FormatCorrect: v.FormatCorrect}}, nil
}

func (r *loiDocumentsRepository) CompleteReview(ctx context.Context, p string, v domain.LOIReview) error {
	if e := r.q.LOIReviewSave(ctx, sqlc.LOIReviewSaveParams{ID: dtos.UUID(v.ID), DocumentID: dtos.UUID(v.DocumentID), DetailsCorrect: v.DetailsCorrect, PricesCorrect: v.PricesCorrect, FormatCorrect: v.FormatCorrect, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "purchase_awards", EntityType: "loi_document_review", EntityID: v.ID, EntityVersion: 1, Action: "completed", ActorID: v.CreatedBy, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: p, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *loiDocumentsRepository) Save(ctx context.Context, v domain.LOIDocument) error {
	content, e := json.Marshal(v.Content)
	if e != nil {
		return e
	}
	date, e := time.Parse("2006-01-02", v.Date)
	if e != nil {
		return e
	}
	return r.q.LOIDocumentsSave(ctx, sqlc.LOIDocumentsSaveParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), HandoffID: dtos.UUID(v.HandoffID), AttemptID: dtos.UUID(v.AttemptID), Version: v.Version, Reference: v.Reference, DocumentDate: pgtype.Date{Time: date, Valid: true}, Format: v.Format, RequestHash: v.RequestHash, Content: content, Sha256: v.SHA256, ObjectBucket: v.Bucket, ObjectKey: v.ObjectKey, ByteSize: v.Size, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
}
func (r *loiDocumentsRepository) Event(ctx context.Context, v domain.LOIDocument, action, actor string) error {
	id := (Values{}).ID()
	at := (Values{}).Now()
	if action == "generated" {
		at = v.CreatedAt
	}
	if e := r.q.LOIDocumentsEvent(ctx, sqlc.LOIDocumentsEventParams{ID: dtos.UUID(id), DocumentID: dtos.UUID(v.ID), Action: action, At: dtos.Stamp(at), ActorID: dtos.UUID(actor)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: id, Module: "purchase_awards", EntityType: "vendor_loi_document_event", EntityID: id, EntityVersion: 1, Action: action, ActorID: actor, ActorKind: "user", CorrelationID: id, ScopeKind: "project", ScopeID: v.ProjectID, Visibility: "restricted", OccurredAt: at}, nil)
}
func (c *awardsContext) Metadata(ctx context.Context, p, vendor string) (out domain.LOIContent, e error) {
	project, e := c.p.Indents.Projects.Get(ctx, p, false)
	if e != nil {
		return out, e
	}
	if project == nil {
		return out, domain.WorkflowMissing
	}
	out.Project = project.Name
	out.ProjectNumber = project.Number
	client, e := c.p.Indents.Clients.Get(ctx, project.ClientID)
	if e != nil {
		return out, e
	}
	if client != nil {
		out.Client = client.Name
	}
	rows, e := c.p.Sourcing.Suppliers.Candidates(ctx)
	if e != nil {
		return out, e
	}
	for _, v := range rows {
		if v.ID != vendor {
			continue
		}
		out.Vendor = v.Name
		out.VendorCode = v.Code
		out.Address = sourcingText(v.Data["address"])
		if out.Address == "" {
			out.Address = sourcingText(v.Data["location"])
		}
		out.ContactPerson = sourcingText(v.Data["contact_person"])
		if out.ContactPerson == "" {
			out.ContactPerson = sourcingText(v.Data["contact_person_name"])
		}
		out.Phone = sourcingText(v.Data["phone"])
		out.Email = sourcingText(v.Data["email"])
		if out.Email == "" {
			out.Email = sourcingText(v.Data["contact"])
		}
		out.GST = sourcingText(v.Data["gst_number"])
		return out, nil
	}
	return out, domain.WorkflowMissing
}
