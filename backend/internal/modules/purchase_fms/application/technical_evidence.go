package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

// EvidenceService owns actual encrypted uploads and audited reads. Uploads do not
// mark a checklist received or grant approval; those remain versioned commands.
type TechnicalEvidenceService struct {
	service   *TechnicalService
	cipher    ports.TechnicalCipher
	objects   ports.OrdersObjects
	bucket    string
	inspector ports.TechnicalInspector
}

func NewTechnicalEvidence(s *TechnicalService, cipher ports.TechnicalCipher, inspector ports.TechnicalInspector) *TechnicalEvidenceService {
	return &TechnicalEvidenceService{service: s, cipher: cipher, inspector: inspector}
}
func (s *TechnicalEvidenceService) WithObjects(objects ports.OrdersObjects, bucket string) *TechnicalEvidenceService {
	s.objects = objects
	s.bucket = bucket
	return s
}
func (s *TechnicalEvidenceService) AuthorizeUpload(ctx context.Context, token, p, id string, q domain.TechnicalUpload) error {
	a, e := s.service.actor(ctx, token, p, id)
	if e != nil {
		return e
	}
	return s.service.u.Within(ctx, func(ctx context.Context, r ports.TechnicalRepository, c ports.TechnicalContext) error {
		_, e := s.uploadScope(ctx, r, c, a, p, id, q)
		return e
	})
}
func (s *TechnicalEvidenceService) uploadScope(ctx context.Context, r ports.TechnicalRepository, c ports.TechnicalContext, a domain.Principal, p, id string, q domain.TechnicalUpload) (*domain.TechnicalCase, error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return nil, e
	}
	record, e := r.Get(ctx, p, id)
	if e != nil {
		return nil, e
	}
	if record == nil {
		return nil, domain.TechnicalMissing
	}
	source, e := c.Source(ctx, p, id)
	if e != nil {
		return nil, e
	}
	if source.ProjectID != p || source.PackageID != id || record.ProjectID != p || record.PackageID != id {
		return nil, domain.TechnicalMissing
	}
	return record, domain.TechnicalUploadGuard(*record, a, caps, source, q)
}
func (s *TechnicalEvidenceService) Upload(ctx context.Context, token, p, id string, q domain.TechnicalUpload) (out domain.TechnicalFile, e error) {
	a, e := s.service.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q.Reason, e = domain.TechnicalText("reason", q.Reason, 2000, true)
	if e != nil {
		return out, e
	}
	if len(q.Data) == 0 || len(q.Data) > domain.TechnicalMaxFileBytes {
		return out, domain.TechnicalInvalid("file", "Upload nonempty PDF or image evidence of at most 10 MiB")
	}
	// Reject authorization before content inspection; repeat live scope inside the
	// storing transaction so revocation/kit revision cannot race the upload.
	if e = s.AuthorizeUpload(ctx, token, p, id, q); e != nil {
		return out, e
	}
	if e = s.inspector.Inspect(q.Name, q.MediaType, q.Data); e != nil {
		return out, e
	}
	e = s.service.u.Within(ctx, func(ctx context.Context, r ports.TechnicalRepository, c ports.TechnicalContext) error {
		record, err := s.uploadScope(ctx, r, c, a, p, id, q)
		if err != nil {
			return err
		}
		encrypted, hash, err := s.cipher.Seal(q.Data)
		if err != nil {
			return err
		}
		out = domain.TechnicalFile{ID: s.service.v.ID(), CaseID: record.ID, Purpose: q.Purpose, Name: q.Name, MediaType: q.MediaType, Size: int64(len(q.Data)), SHA256: hash, Active: true}
		now := s.service.v.Now()
		evidence := domain.TechnicalEvidence{File: out, Ciphertext: encrypted, CreatedBy: a.ID, CreatedAt: now}
		if s.objects != nil {
			key := "purchase-technical/" + p + "/" + record.ID + "/" + out.ID
			if err = s.objects.Put(ctx, s.bucket, key, encrypted); err != nil {
				return err
			}
			evidence.Bucket = s.bucket
			evidence.ObjectKey = key
			evidence.Ciphertext = nil
		}
		if err = r.StoreEvidence(ctx, evidence); err != nil {
			return err
		}
		return r.EvidenceEvent(ctx, domain.TechnicalFileEvent{ID: s.service.v.ID(), CaseID: record.ID, FileID: out.ID, Action: "uploaded", ActorID: a.ID, Reason: q.Reason, At: now})
	})
	if e != nil {
		out = domain.TechnicalFile{}
	}
	return
}
func (s *TechnicalEvidenceService) Download(ctx context.Context, token, p, id, fileID string) (out domain.TechnicalFile, data []byte, e error) {
	a, e := s.service.actor(ctx, token, p, id)
	if e != nil {
		return out, nil, e
	}
	if e = domain.TechnicalID(fileID); e != nil {
		return out, nil, e
	}
	e = s.service.u.Within(ctx, func(ctx context.Context, r ports.TechnicalRepository, c ports.TechnicalContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if !caps.Read {
			return domain.TechnicalForbidden
		}
		record, err := r.Get(ctx, p, id)
		if err != nil {
			return err
		}
		if record == nil || !record.Active || record.ProjectID != p || record.PackageID != id {
			return domain.TechnicalMissing
		}
		v, err := r.Evidence(ctx, record.ID, fileID)
		if err != nil {
			return err
		}
		if v == nil || !v.File.Active || v.File.CaseID != record.ID {
			return domain.TechnicalMissing
		}
		encrypted := v.Ciphertext
		if v.ObjectKey != "" {
			if s.objects == nil {
				return domain.TechnicalInvalid("file", "Private technical object storage unavailable")
			}
			encrypted, err = s.objects.Get(ctx, v.Bucket, v.ObjectKey)
			if err != nil {
				return err
			}
		}
		data, err = s.cipher.Open(encrypted, v.File.SHA256)
		if err != nil {
			return err
		}
		out = v.File
		return r.EvidenceEvent(ctx, domain.TechnicalFileEvent{ID: s.service.v.ID(), CaseID: record.ID, FileID: fileID, Action: "downloaded", ActorID: a.ID, Reason: "Authorized technical evidence read", At: s.service.v.Now()})
	})
	if e != nil {
		out = domain.TechnicalFile{}
		data = nil
	}
	return
}
