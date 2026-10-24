package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/ports"
)

type Service struct {
	unit    ports.UnitOfWork
	auth    ports.Authorizer
	objects ports.Objects
	crypto  ports.Encryptor
	values  ports.Values
	bucket  string
}

func New(unit ports.UnitOfWork, auth ports.Authorizer, objects ports.Objects, crypto ports.Encryptor, values ports.Values, bucket string) *Service {
	return &Service{unit, auth, objects, crypto, values, bucket}
}
func (s *Service) actor(ctx context.Context, token, permission string) (domain.Principal, error) {
	actor, err := s.auth.Resolve(ctx, token)
	if err != nil {
		return domain.Principal{}, err
	}
	if !actor.Active || actor.ID == "" {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	if !actor.Allows(permission) {
		return domain.Principal{}, domain.Denied(permission)
	}
	return actor, nil
}
func (s *Service) Upload(ctx context.Context, token string, request domain.Upload) (domain.Document, error) {
	actor, err := s.actor(ctx, token, "documents.storage.write")
	if err != nil {
		return domain.Document{}, err
	}
	if err = domain.Validate(request); err != nil {
		return domain.Document{}, err
	}
	entityID, err := domain.CanonicalID(request.EntityID)
	if err != nil {
		return domain.Document{}, err
	}
	filename := domain.SanitizeFilename(request.Filename)
	hash := s.crypto.Hash(request.Data)
	ciphertext, err := s.crypto.Encrypt(request.Data)
	if err != nil {
		return domain.Document{}, err
	}
	documentID := s.values.ID()
	// Use the entity type as the private object-key prefix.
	key, err := domain.StorageKey(request.EntityType, entityID, documentID, filename)
	if err != nil {
		return domain.Document{}, err
	}
	title := domain.UploadTitle(request.Title, filename)
	now := domain.At(s.values.Now())
	document := domain.Document{ID: documentID, EntityType: domain.Normalize(request.EntityType), EntityID: entityID, DocType: domain.Normalize(request.DocType), Title: title, Filename: filename, ContentType: domain.Normalize(request.ContentType), ByteSize: len(request.Data), Bucket: s.bucket, Key: key, SHA256: hash, Encrypted: true, Algorithm: s.crypto.Algorithm(), Active: true, CreatedAt: now, CreatedBy: &actor.ID, UpdatedAt: now, UpdatedBy: &actor.ID}
	err = s.unit.Within(ctx, func(ctx context.Context, repo ports.Repository) error {
		saved, err := repo.Save(ctx, document)
		if err != nil {
			return err
		}
		document = saved
		err = repo.Event(ctx, domain.Event{ID: s.values.ID(), DocumentID: document.ID, Action: "UPLOAD", ActorID: actor.ID, Snapshot: map[string]any{"entity_type": request.EntityType, "entity_id": entityID, "doc_type": request.DocType, "byte_size": len(request.Data), "sha256": hash, "algorithm": s.crypto.Algorithm()}})
		if err != nil {
			return err
		}
		// DB metadata and audit remain uncommitted until object persistence succeeds.
		// An uncertain PUT/COMMIT can leave an orphan; never delete potentially committed evidence.
		return s.objects.Put(ctx, document.Bucket, document.Key, ciphertext)
	})
	if err != nil {
		return domain.Document{}, err
	}
	return document, nil
}
func (s *Service) Metadata(ctx context.Context, token, id string) (*domain.Document, error) {
	_, err := s.actor(ctx, token, "documents.storage.read")
	if err != nil {
		return nil, err
	}
	var doc *domain.Document
	err = s.unit.Within(ctx, func(ctx context.Context, repo ports.Repository) error {
		var err error
		doc, err = repo.Get(ctx, id, false)
		if err != nil {
			return err
		}
		if doc == nil || !doc.Active {
			return domain.ErrInactive
		}
		return nil
	})
	return doc, err
}
func (s *Service) List(ctx context.Context, token, entityType, entityID string) ([]domain.Document, error) {
	_, err := s.actor(ctx, token, "documents.storage.read")
	if err != nil {
		return nil, err
	}
	var docs []domain.Document
	err = s.unit.Within(ctx, func(ctx context.Context, repo ports.Repository) error {
		var err error
		docs, err = repo.List(ctx, domain.Normalize(entityType), entityID)
		return err
	})
	return docs, err
}
func (s *Service) Download(ctx context.Context, token, id string) (domain.Document, []byte, error) {
	actor, err := s.actor(ctx, token, "documents.storage.read")
	if err != nil {
		return domain.Document{}, nil, err
	}
	var doc domain.Document
	var data []byte
	err = s.unit.Within(ctx, func(ctx context.Context, repo ports.Repository) error {
		record, err := repo.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if record == nil || !record.Active {
			return domain.Invalid("Document " + id + " was not found or is inactive.")
		}
		doc = *record
		data, err = s.objects.Get(ctx, doc.Bucket, doc.Key)
		if err != nil {
			return err
		}
		if doc.Encrypted {
			data, err = s.crypto.Decrypt(data)
			if err != nil {
				return domain.ErrCiphertext
			}
		}
		if s.crypto.Hash(data) != doc.SHA256 {
			return domain.ErrIntegrity
		}
		return repo.Event(ctx, domain.Event{ID: s.values.ID(), DocumentID: doc.ID, Action: "DOWNLOAD", ActorID: actor.ID, Snapshot: map[string]any{"byte_size": len(data)}})
	})
	if err != nil {
		return domain.Document{}, nil, err
	}
	return doc, data, nil
}
func (s *Service) Deactivate(ctx context.Context, token, id string) error {
	actor, err := s.actor(ctx, token, "documents.storage.write")
	if err != nil {
		return err
	}
	return s.unit.Within(ctx, func(ctx context.Context, repo ports.Repository) error {
		record, err := repo.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if record == nil {
			return domain.ErrMissing
		}
		if _, err = repo.Deactivate(ctx, id, actor.ID); err != nil {
			return err
		}
		return repo.Event(ctx, domain.Event{ID: s.values.ID(), DocumentID: id, Action: "DEACTIVATE", ActorID: actor.ID, Snapshot: map[string]any{}})
	})
}
