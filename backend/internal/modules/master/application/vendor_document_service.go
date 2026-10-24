package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorDocumentService struct {
	u       masterPorts.VendorDocumentUnitOfWork
	a       masterPorts.Authorizer
	c       masterPorts.VendorDocumentCipher
	v       masterPorts.VendorDocumentValues
	i       masterPorts.VendorDocumentInspector
	objects masterPorts.VendorDocumentObjects
	bucket  string
}

func NewVendorDocumentService(u masterPorts.VendorDocumentUnitOfWork, a masterPorts.Authorizer, c masterPorts.VendorDocumentCipher, v masterPorts.VendorDocumentValues) *VendorDocumentService {
	return &VendorDocumentService{u: u, a: a, c: c, v: v}
}
func (s *VendorDocumentService) actor(ctx context.Context, t, p string) (sharedIdentity.Principal, error) {
	v, e := s.a.Resolve(ctx, t)
	if e != nil {
		return v, e
	}
	if v.ID == "" || !v.Active {
		return v, masterDomain.VendorDocumentUnauthorized
	}
	if !v.Allows(p) && !(p == "settings.supplier_documents.read" && v.Allows("accounts.vendor_verification.read")) {
		return v, masterDomain.VendorDocumentForbidden
	}
	return v, nil
}
func (s *VendorDocumentService) List(ctx context.Context, t, id string) ([]masterDomain.VendorDocumentDocument, error) {
	if _, e := s.actor(ctx, t, "settings.supplier_documents.read"); e != nil {
		return nil, e
	}
	var result []masterDomain.VendorDocumentDocument
	e := s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorDocumentRepository, d masterPorts.VendorDocumentSuppliers) error {
		exists, e := d.Exists(ctx, id)
		if e != nil {
			return e
		}
		if !exists {
			return masterDomain.VendorDocumentMissing
		}
		result, e = r.List(ctx, id)
		return e
	})
	if e != nil {
		return nil, e
	}
	return result, nil
}
func (s *VendorDocumentService) Import(ctx context.Context, t, supplier string, in masterDomain.VendorDocumentInput) (*masterDomain.VendorDocumentDocument, error) {
	p, e := s.actor(ctx, t, "settings.supplier_documents.import")
	if e != nil {
		return nil, e
	}
	if !masterDomain.VendorDocumentValidID(supplier) {
		return nil, masterDomain.VendorDocumentInvalid("Invalid supplier identity")
	}
	if e = masterDomain.VendorDocumentValidate(in); e != nil {
		return nil, e
	}
	var result *masterDomain.VendorDocumentDocument
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorDocumentRepository, d masterPorts.VendorDocumentSuppliers) error {
		exists, e := d.Exists(ctx, supplier)
		if e != nil {
			return e
		}
		if !exists {
			return masterDomain.VendorDocumentMissing
		}
		existing, e := r.BySource(ctx, in.Source.ID)
		if e != nil {
			return e
		}
		if existing != nil {
			if existing.SupplierID != supplier || existing.SHA256 != s.c.Hash(in.Content) || existing.DocType != in.DocType || existing.Title != in.Title || existing.Filename != in.Filename || existing.ContentType != in.ContentType || existing.Active != *in.Active || !existing.Source.CreatedAt.Equal(in.Source.CreatedAt) || !existing.Source.UpdatedAt.Equal(in.Source.UpdatedAt) {
				return masterDomain.VendorDocumentConflict
			}
			result = existing
			return nil
		}
		encrypted, hash, e := s.c.Seal(in.Content)
		if e != nil {
			return e
		}
		documentID := s.v.ID()
		bucket, key, e := s.store(ctx, supplier, documentID, encrypted)
		if e != nil {
			return e
		}
		v := masterDomain.VendorDocumentDocument{ID: documentID, ObjectBucket: bucket, ObjectKey: key, SupplierID: supplier, DocType: in.DocType, Title: in.Title, Filename: in.Filename, ContentType: in.ContentType, ByteSize: int32(len(in.Content)), SHA256: hash, Active: *in.Active, Encrypted: true, CreatedBy: &p.ID, UpdatedBy: &p.ID, Ciphertext: nil, Source: in.Source}
		result, e = r.Insert(ctx, v)
		if e != nil {
			return e
		}
		return r.Imported(ctx, result.ID, s.v.ID(), p.ID)
	})
	if e != nil {
		return nil, e
	}
	return result, nil
}
func (s *VendorDocumentService) Download(ctx context.Context, t, supplier, id string) (*masterDomain.VendorDocumentDocument, []byte, error) {
	p, e := s.actor(ctx, t, "settings.supplier_documents.read")
	if e != nil {
		return nil, nil, e
	}
	var result *masterDomain.VendorDocumentDocument
	var data []byte
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorDocumentRepository, _ masterPorts.VendorDocumentSuppliers) error {
		v, e := r.Get(ctx, supplier, id)
		if e != nil {
			return e
		}
		if v == nil || (!v.Active && !p.Allows("settings.supplier_documents.import") && !p.Allows("accounts.vendor_verification.read") && (v.Source.ID != "" || v.CreatedBy == nil || *v.CreatedBy != p.ID)) {
			return masterDomain.VendorDocumentMissing
		}
		encrypted := v.Ciphertext
		if v.ObjectKey != "" {
			if s.objects == nil {
				return masterDomain.VendorDocumentInvalid("Document storage unavailable")
			}
			encrypted, e = s.objects.Get(ctx, v.ObjectBucket, v.ObjectKey)
			if e != nil {
				return masterDomain.VendorDocumentIntegrity
			}
		}
		data, e = s.c.Open(encrypted, v.SHA256)
		if e != nil {
			return masterDomain.VendorDocumentIntegrity
		}
		result = v
		return e
	})
	if e != nil {
		return nil, nil, e
	}
	return result, data, nil
}
