package application

import (
	"context"
	"errors"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func (s *OrdersService) WithEvidence(cipher ports.TechnicalCipher, inspector ports.TechnicalInspector, objects ports.OrdersObjects, bucket string) *OrdersService {
	s.cipher = cipher
	s.inspector = inspector
	s.objects = objects
	s.bucket = bucket
	return s
}
func (s *OrdersService) clientFile(ctx context.Context, r ports.OrdersRepository, o *domain.OrdersOrder, id string) error {
	if o == nil {
		return domain.OrdersMissing
	}
	files, ok := r.(ports.OrdersEvidenceRepository)
	if !ok {
		return errors.New("PO evidence repository unavailable")
	}
	f, e := files.File(ctx, o.ID, id)
	if e != nil {
		return e
	}
	if f == nil || f.OrderID != o.ID || f.DraftRevision != o.DraftRevision {
		return domain.OrdersInvalid("file_id", "Use signed PO evidence uploaded for this exact draft revision")
	}
	if s.objects == nil || s.cipher == nil {
		return errors.New("Private PO object storage unavailable")
	}
	encrypted, e := s.objects.Get(ctx, f.Bucket, f.ObjectKey)
	if e != nil {
		return e
	}
	data, e := s.cipher.Open(encrypted, f.SHA256)
	if e != nil {
		return e
	}
	if int64(len(data)) != f.Size {
		return domain.OrdersInvalid("file_id", "Signed file integrity check failed")
	}
	return s.inspector.Inspect(f.Name, f.MediaType, data)
}
func (s *OrdersService) AuthorizeClientUpload(ctx context.Context, token, p, id string, version int32) error {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return e
	}
	return s.u.Within(ctx, func(ctx context.Context, r ports.OrdersRepository, c ports.OrdersContext) error {
		_, e = s.uploadOrder(ctx, r, c, a, p, id, version)
		return e
	})
}
func (s *OrdersService) uploadOrder(ctx context.Context, r ports.OrdersRepository, c ports.OrdersContext, a domain.Principal, p, id string, version int32) (*domain.OrdersOrder, error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return nil, e
	}
	if e = domain.OrdersAccessGuard(a, caps, "client_decision"); e != nil {
		return nil, e
	}
	o, e := r.Get(ctx, p, id)
	if e != nil {
		return nil, e
	}
	if o == nil || !o.Active {
		return nil, domain.OrdersMissing
	}
	if o.Version != version {
		return nil, domain.OrdersStale
	}
	if o.State != "draft" || o.ClientApproval.Status != "submitted" {
		return nil, domain.OrdersInvalid("state", "Submit the draft to the client before attaching their signed response")
	}
	source, e := c.Source(ctx, p, id)
	if e != nil {
		return nil, e
	}
	if e = source.Guard(); e != nil {
		return nil, e
	}
	if !domain.OrdersSameSource(o.Source, source) {
		return nil, domain.OrdersStale
	}
	return o, nil
}
func (s *OrdersService) UploadClientFile(ctx context.Context, token, p, id string, version int32, name, mime string, data []byte) (out domain.OrdersFile, e error) {
	if e = s.AuthorizeClientUpload(ctx, token, p, id, version); e != nil {
		return out, e
	}
	if s.inspector == nil || s.objects == nil || s.cipher == nil || s.bucket == "" {
		return out, errors.New("Private PO evidence storage unavailable")
	}
	if e = s.inspector.Inspect(name, mime, data); e != nil {
		return out, e
	}
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.OrdersRepository, c ports.OrdersContext) error {
		o, err := s.uploadOrder(ctx, r, c, a, p, id, version)
		if err != nil {
			return err
		}
		files, ok := r.(ports.OrdersEvidenceRepository)
		if !ok {
			return errors.New("PO evidence repository unavailable")
		}
		encrypted, hash, err := s.cipher.Seal(data)
		if err != nil {
			return err
		}
		fileID := s.v.ID()
		key := "purchase-orders/" + p + "/" + o.ID + "/" + fileID
		if err = s.objects.Put(ctx, s.bucket, key, encrypted); err != nil {
			return err
		}
		out = domain.OrdersFile{ID: fileID, OrderID: o.ID, DraftRevision: o.DraftRevision, Name: name, MediaType: mime, Size: int64(len(data)), SHA256: hash, Bucket: s.bucket, ObjectKey: key, CreatedAt: s.v.Now(), CreatedBy: a.ID}
		if err = files.StoreFile(ctx, out); err != nil {
			return err
		}
		return files.FileEvent(ctx, domain.OrdersFileEvent{ID: s.v.ID(), OrderID: o.ID, FileID: fileID, ActorID: a.ID, Action: "uploaded", At: out.CreatedAt})
	})
	if e != nil {
		out = domain.OrdersFile{}
	}
	return
}
func (s *OrdersService) ClientFile(ctx context.Context, token, p, id, fileID string) (out domain.OrdersFile, data []byte, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, nil, e
	}
	if e = domain.OrdersID(fileID); e != nil {
		return out, nil, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.OrdersRepository, c ports.OrdersContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.OrdersAccessGuard(a, caps, "read"); err != nil {
			return err
		}
		o, err := r.Get(ctx, p, id)
		if err != nil {
			return err
		}
		if o == nil {
			return domain.OrdersMissing
		}
		files, ok := r.(ports.OrdersEvidenceRepository)
		if !ok || s.objects == nil || s.cipher == nil {
			return errors.New("PO evidence unavailable")
		}
		f, err := files.File(ctx, o.ID, fileID)
		if err != nil {
			return err
		}
		if f == nil {
			return domain.OrdersMissing
		}
		encrypted, err := s.objects.Get(ctx, f.Bucket, f.ObjectKey)
		if err != nil {
			return err
		}
		data, err = s.cipher.Open(encrypted, f.SHA256)
		if err != nil {
			return err
		}
		out = *f
		return files.FileEvent(ctx, domain.OrdersFileEvent{ID: s.v.ID(), OrderID: o.ID, FileID: fileID, ActorID: a.ID, Action: "downloaded", At: s.v.Now()})
	})
	if e != nil {
		out = domain.OrdersFile{}
		data = nil
	}
	return
}
