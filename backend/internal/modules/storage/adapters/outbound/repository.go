package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/ports"
	"time"
)

type UnitOfWork struct{ manager *database.Manager }

func NewUnitOfWork(beginner database.Beginner) *UnitOfWork {
	return &UnitOfWork{database.NewManager(beginner)}
}
func (u *UnitOfWork) Within(ctx context.Context, work func(context.Context, ports.Repository) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, connection database.Connection) error {
		return work(ctx, &repository{db.New(connection)})
	})
}

type repository struct{ queries *db.Queries }

var _ ports.Repository = (*repository)(nil)

func uuid(value string) pgtype.UUID { var id pgtype.UUID; _ = id.Scan(value); return id }
func actor(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	value := id.String()
	return &value
}
func instant(stamp pgtype.Timestamptz) *time.Time {
	if !stamp.Valid {
		return nil
	}
	value := stamp.Time.UTC()
	return &value
}
func document(row db.StoredDocument) domain.Document {
	return domain.Document{ID: row.ID.String(), EntityType: row.EntityType, EntityID: row.EntityID.String(), DocType: row.DocType, Title: row.Title, Filename: row.OriginalFilename, ContentType: row.ContentType, ByteSize: int(row.ByteSize), Bucket: row.StorageBucket, Key: row.StorageKey, SHA256: row.Sha256Hash, Encrypted: row.IsEncrypted, Algorithm: row.EncryptionAlgorithm, Active: row.IsActive, CreatedAt: instant(row.CreatedAt), CreatedBy: actor(row.CreatedByID), UpdatedAt: instant(row.UpdatedAt), UpdatedBy: actor(row.UpdatedByID)}
}
func (r *repository) Save(ctx context.Context, doc domain.Document) (domain.Document, error) {
	row, err := r.queries.InsertDocument(ctx, db.InsertDocumentParams{ID: uuid(doc.ID), EntityType: doc.EntityType, EntityID: uuid(doc.EntityID), DocType: doc.DocType, Title: doc.Title, OriginalFilename: doc.Filename, ContentType: doc.ContentType, ByteSize: int32(doc.ByteSize), StorageBucket: doc.Bucket, StorageKey: doc.Key, Sha256Hash: doc.SHA256, EncryptionAlgorithm: doc.Algorithm, CreatedAt: pgtype.Timestamptz{Time: *doc.CreatedAt, Valid: true}, CreatedByID: uuid(*doc.CreatedBy)})
	return document(row), err
}
func (r *repository) Get(ctx context.Context, id string, lock bool) (*domain.Document, error) {
	var row db.StoredDocument
	var err error
	if lock {
		row, err = r.queries.LockDocument(ctx, uuid(id))
	} else {
		row, err = r.queries.GetDocument(ctx, uuid(id))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	doc := document(row)
	return &doc, nil
}
func (r *repository) List(ctx context.Context, entityType, entityID string) ([]domain.Document, error) {
	rows, err := r.queries.ListDocuments(ctx, db.ListDocumentsParams{EntityType: entityType, EntityID: uuid(entityID)})
	if err != nil {
		return nil, err
	}
	docs := []domain.Document{}
	for _, row := range rows {
		docs = append(docs, document(row))
	}
	return docs, nil
}
func (r *repository) Deactivate(ctx context.Context, id, actorID string) (*domain.Document, error) {
	row, err := r.queries.DeactivateDocument(ctx, db.DeactivateDocumentParams{ID: uuid(id), UpdatedByID: uuid(actorID)})
	if err != nil {
		return nil, err
	}
	doc := document(row)
	return &doc, nil
}
func (r *repository) Event(ctx context.Context, event domain.Event) error {
	snapshot, err := json.Marshal(event.Snapshot)
	if err != nil {
		return err
	}
	return r.queries.InsertEvent(ctx, db.InsertEventParams{ID: uuid(event.ID), DocumentID: uuid(event.DocumentID), Action: event.Action, ActorID: uuid(event.ActorID), MetadataSnapshot: snapshot})
}
