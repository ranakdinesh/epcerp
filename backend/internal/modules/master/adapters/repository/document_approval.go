package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dto "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type DocumentApprovalRepository struct {
	q      *db.Queries
	events events.Writer
}

func NewDocumentApprovalRepository(c database.Connection, j events.Writer) *DocumentApprovalRepository {
	return &DocumentApprovalRepository{NewStore(c).Queries, j}
}
func (r *DocumentApprovalRepository) Get(ctx context.Context, id string) (*domain.DocumentApproval, error) {
	v, e := r.q.DocumentApprovalGet(ctx, dto.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &domain.DocumentApproval{DocumentID: id, SupplierID: v.SupplierID.String(), DocumentVersion: v.DocumentVersion, SHA256: v.Sha256, State: v.State, Comment: v.Comment, Version: v.Version}, nil
}
func (r *DocumentApprovalRepository) Save(ctx context.Context, v domain.DocumentApproval, actor, action string) error {
	saved, e := r.q.DocumentApprovalSave(ctx, db.DocumentApprovalSaveParams{DocumentID: dto.UUID(v.DocumentID), SupplierID: dto.UUID(v.SupplierID), DocumentVersion: v.DocumentVersion, Sha256: v.SHA256, State: v.State, Comment: v.Comment, CreatedByID: dto.UUID(actor)})
	if e != nil {
		return e
	}
	payload, e := json.Marshal(saved)
	if e != nil {
		return e
	}
	id := newRecordID()
	if e = r.q.DocumentApprovalJournal(ctx, db.DocumentApprovalJournalParams{ID: dto.UUID(id), DocumentID: saved.DocumentID, Version: saved.Version, Action: action, Snapshot: payload, CreatedByID: dto.UUID(actor)}); e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: id, Module: "supplier_document_approval", EntityType: "supplier_document_approval", EntityID: v.DocumentID, EntityVersion: int64(saved.Version), Action: action, ActorID: actor, ActorKind: "user", CorrelationID: id, ScopeKind: "platform", Visibility: "restricted", OccurredAt: saved.UpdatedAt.Time}, nil)
}
func (r *DocumentApprovalRepository) Reviewer(ctx context.Context, supplier string) (string, error) {
	v, e := r.q.VendorReviewAssignmentGet(ctx, dto.UUID(supplier))
	if errors.Is(e, pgx.ErrNoRows) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	if !v.IsActive {
		return "", nil
	}
	return v.UserID.String(), nil
}
func (r *DocumentApprovalRepository) Task(ctx context.Context, id, supplier, user, kind, comment, actor string) error {
	return r.q.DocumentTaskCreate(ctx, db.DocumentTaskCreateParams{ID: dto.UUID(newRecordID()), DocumentID: dto.UUID(id), SupplierID: dto.UUID(supplier), UserID: dto.UUID(user), Kind: kind, Comment: comment, CreatedByID: dto.UUID(actor)})
}
func (r *DocumentApprovalRepository) Complete(ctx context.Context, id, kind, outcome, comment, actor string) error {
	return r.q.DocumentTasksComplete(ctx, db.DocumentTasksCompleteParams{DocumentID: dto.UUID(id), Kind: kind, Outcome: outcome, Comment: comment, UpdatedByID: dto.UUID(actor)})
}
func (r *DocumentApprovalRepository) Document(ctx context.Context, supplier, id string) (*domain.VendorDocumentDocument, error) {
	v, e := r.q.VendorDocumentsGetDocument(ctx, db.VendorDocumentsGetDocumentParams{SupplierID: dto.UUID(supplier), ID: dto.UUID(id)})
	if e != nil {
		return nil, e
	}
	return dto.VendorDocumentFromRow(v), nil
}

// The decorators expose lifecycle use cases on the exact existing transaction.
type documentWorkflowRepository struct {
	ports.VendorDocumentRepository
	ports.DocumentApprovalLifecycle
}
type verificationWorkflowRepository struct {
	ports.VendorVerificationRepository
	ports.DocumentApprovalLifecycle
}

func NewApprovalUserDirectory(u auth.AssignmentDirectory) ports.VendorVerificationUsers {
	return &vendorReviewUsers{VendorVerificationUserDirectory{u}, u}
}
