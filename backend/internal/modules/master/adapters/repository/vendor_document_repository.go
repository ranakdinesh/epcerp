package repository

import (
	"context"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorDocumentUnitOfWork struct {
	approval  func(database.Connection) masterPorts.DocumentApprovalLifecycle
	m         *database.Manager
	suppliers func(database.Connection) masterContracts.VendorDirectory
	events    func(database.Connection) events.Writer
}

func NewVendorDocumentUnitOfWork(b database.Beginner, d func(database.Connection) masterContracts.VendorDirectory, e func(database.Connection) events.Writer) *VendorDocumentUnitOfWork {
	return &VendorDocumentUnitOfWork{m: database.NewManager(b), suppliers: d, events: e}
}
func (u *VendorDocumentUnitOfWork) Within(ctx context.Context, w func(context.Context, masterPorts.VendorDocumentRepository, masterPorts.VendorDocumentSuppliers) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		var repository masterPorts.VendorDocumentRepository = &vendorDocumentRepository{NewStore(c).Queries, u.events(c)}
		if u.approval != nil {
			repository = &documentWorkflowRepository{repository, u.approval(c)}
		}
		return w(ctx, repository, u.suppliers(c))
	})
}

type vendorDocumentRepository struct {
	q      *db.Queries
	events events.Writer
}

func (r *vendorDocumentRepository) List(ctx context.Context, id string) ([]masterDomain.VendorDocumentDocument, error) {
	rows, e := r.q.VendorDocumentsListDocuments(ctx, masterDTO.UUID(id))
	if e != nil {
		return nil, e
	}
	result := []masterDomain.VendorDocumentDocument{}
	for _, v := range rows {
		result = append(result, masterDomain.VendorDocumentDocument{ApprovalState: v.ApprovalState, ApprovalComment: v.ApprovalComment, Version: v.Version, ID: v.ID.String(), SupplierID: v.SupplierID.String(), DocType: v.DocType, Title: v.Title, Filename: v.OriginalFilename, ContentType: v.ContentType, ByteSize: v.ByteSize, SHA256: v.Sha256, Active: v.IsActive, Encrypted: true, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: masterDTO.Actor(v.CreatedByID), UpdatedBy: masterDTO.Actor(v.UpdatedByID), Source: masterDomain.VendorDocumentSource{ID: v.SourceID.String(), CreatedAt: v.SourceCreatedAt.Time, UpdatedAt: v.SourceUpdatedAt.Time}})
	}
	return result, nil
}
func (r *vendorDocumentRepository) Get(ctx context.Context, supplier, id string) (*masterDomain.VendorDocumentDocument, error) {
	v, e := r.q.VendorDocumentsGetDocument(ctx, db.VendorDocumentsGetDocumentParams{SupplierID: masterDTO.UUID(supplier), ID: masterDTO.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorDocumentFromRow(v), nil
}
func (r *vendorDocumentRepository) BySource(ctx context.Context, id string) (*masterDomain.VendorDocumentDocument, error) {
	if e := r.q.VendorDocumentsLockImport(ctx); e != nil {
		return nil, e
	}
	v, e := r.q.VendorDocumentsSourceDocument(ctx, masterDTO.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorDocumentFromRow(v), nil
}
func (r *vendorDocumentRepository) Insert(ctx context.Context, v masterDomain.VendorDocumentDocument) (*masterDomain.VendorDocumentDocument, error) {
	row, e := r.q.VendorDocumentsInsertDocument(ctx, db.VendorDocumentsInsertDocumentParams{ID: masterDTO.UUID(v.ID), SupplierID: masterDTO.UUID(v.SupplierID), DocType: v.DocType, Title: v.Title, Filename: v.Filename, ContentType: v.ContentType, ByteSize: v.ByteSize, EncryptedContent: v.Ciphertext, ObjectBucket: pgtype.Text{String: v.ObjectBucket, Valid: v.ObjectBucket != ""}, ObjectKey: pgtype.Text{String: v.ObjectKey, Valid: v.ObjectKey != ""}, Sha256: v.SHA256, Active: v.Active, Actor: masterDTO.UUID(*v.CreatedBy), SourceID: masterDTO.UUID(v.Source.ID), SourceCreatedAt: pgtype.Timestamptz{Time: v.Source.CreatedAt, Valid: v.Source.ID != ""}, SourceUpdatedAt: pgtype.Timestamptz{Time: v.Source.UpdatedAt, Valid: v.Source.ID != ""}})
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorDocumentFromRow(row), nil
}
func (r *vendorDocumentRepository) Imported(ctx context.Context, id, event, actor string) error {
	var at pgtype.Timestamptz

	at, e := r.q.VendorDocumentsDocumentCreatedAt(ctx, masterDTO.UUID(id))
	if e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: event, Module: "supplier_documents", EntityType: "supplier_document", EntityID: id, EntityVersion: 1, Action: "legacy_imported", ActorID: actor, ActorKind: "user", CorrelationID: event, ScopeKind: "platform", Visibility: "restricted", OccurredAt: at.Time}, nil)
}

type VendorDocumentValues struct{}

func (VendorDocumentValues) ID() string {
	return newRecordID()
}

func (r *vendorDocumentRepository) Archive(ctx context.Context, supplier, id string, version int32, actor string) (*masterDomain.VendorDocumentDocument, error) {
	v, e := r.q.VendorDocumentsArchiveDocument(ctx, db.VendorDocumentsArchiveDocumentParams{Supplier: masterDTO.UUID(supplier), ID: masterDTO.UUID(id), Version: version, Actor: masterDTO.UUID(actor)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorDocumentFromRow(v), nil
}
func (r *vendorDocumentRepository) Changed(ctx context.Context, v masterDomain.VendorDocumentDocument, event, action, reason string) error {
	if e := r.q.VendorDocumentsRecordChange(ctx, db.VendorDocumentsRecordChangeParams{ID: masterDTO.UUID(event), DocumentID: masterDTO.UUID(v.ID), Version: v.Version, Action: action, Reason: reason, CreatedByID: masterDTO.UUID(*v.UpdatedBy)}); e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: event, Module: "supplier_documents", EntityType: "supplier_document", EntityID: v.ID, EntityVersion: int64(v.Version), Action: action, ActorID: *v.UpdatedBy, ActorKind: "user", CorrelationID: event, ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.UpdatedAt}, nil)
}

func (u *VendorDocumentUnitOfWork) WithApprovalLifecycle(f func(database.Connection) masterPorts.DocumentApprovalLifecycle) *VendorDocumentUnitOfWork {
	u.approval = f
	return u
}
