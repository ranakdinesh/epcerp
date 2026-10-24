package application

import (
	"context"
	"errors"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

func (s *VendorDocumentService) WithInspector(i masterPorts.VendorDocumentInspector) *VendorDocumentService {
	s.i = i
	return s
}
func (s *VendorDocumentService) Upload(ctx context.Context, t, id string, in masterDomain.VendorDocumentInput, reason string) (*masterDomain.VendorDocumentDocument, error) {
	p, e := s.actor(ctx, t, "settings.supplier_documents.write")
	if e != nil {
		return nil, e
	}
	if !masterDomain.VendorDocumentValidID(id) {
		return nil, masterDomain.VendorDocumentMissing
	}
	if reason == "" {
		reason = "Supplier document uploaded"
	}
	if !masterDomain.VendorDocumentValidReason(reason) {
		return nil, masterDomain.VendorDocumentFieldError("reason", "Invalid upload reason")
	}
	if e = masterDomain.VendorDocumentValidateUpload(in); e != nil {
		return nil, e
	}
	if s.i == nil {
		return nil, masterDomain.VendorDocumentInvalid("Document uploads unavailable")
	}
	kind, e := s.inspect(in.Content)
	if e != nil {
		return nil, e
	}
	encrypted, hash, e := s.c.Seal(in.Content)
	if e != nil {
		return nil, e
	}
	var result *masterDomain.VendorDocumentDocument
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorDocumentRepository, d masterPorts.VendorDocumentSuppliers) error {
		ok, e := d.Exists(ctx, id)
		if e != nil {
			return e
		}
		if !ok {
			return masterDomain.VendorDocumentMissing
		}
		// A new singleton identity document replaces that type, preserving unrelated files.
		if in.DocType != "other" || in.ReplacesID != "" {
			existing, err := r.List(ctx, id)
			if err != nil {
				return err
			}
			if in.ReplacesID != "" {
				found := false
				for _, old := range existing {
					if old.ID == in.ReplacesID && old.Active {
						found = true
					}
				}
				if !found {
					return masterDomain.VendorDocumentConflict
				}
			}
			for _, old := range existing {
				if old.Active && (old.ID == in.ReplacesID || (in.DocType != "other" && old.DocType == in.DocType)) {
					retired, err := r.Archive(ctx, id, old.ID, old.Version, p.ID)
					if err != nil {
						return err
					}
					if err = r.Changed(ctx, *retired, s.v.ID(), "archived", "Replaced by a new document"); err != nil {
						return err
					}
					if workflow, ok := r.(masterPorts.DocumentApprovalLifecycle); ok {
						if err = workflow.Retired(ctx, *retired, p.ID, "replaced"); err != nil {
							return err
						}
					}
				}
			}
		}
		documentID := s.v.ID()
		bucket, key, e := s.store(ctx, id, documentID, encrypted)
		if e != nil {
			return e
		}
		result, e = r.Insert(ctx, masterDomain.VendorDocumentDocument{ID: documentID, ObjectBucket: bucket, ObjectKey: key, SupplierID: id, Version: 1, DocType: in.DocType, Title: in.Title, Filename: in.Filename, ContentType: kind, ByteSize: int32(len(in.Content)), SHA256: hash, Active: true, Encrypted: true, CreatedBy: &p.ID, UpdatedBy: &p.ID, Ciphertext: nil})
		if e != nil {
			return e
		}
		if e = r.Changed(ctx, *result, s.v.ID(), "uploaded", reason); e != nil {
			return e
		}
		if workflow, ok := r.(masterPorts.DocumentApprovalLifecycle); ok {
			return workflow.Submitted(ctx, *result, p.ID)
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return result, nil
}
func (s *VendorDocumentService) Preview(ctx context.Context, t, supplier, id string) (*masterDomain.VendorDocumentDocument, []byte, error) {
	v, data, e := s.Download(ctx, t, supplier, id)
	if e != nil {
		return nil, nil, e
	}
	if s.i == nil {
		return nil, nil, masterDomain.VendorDocumentInvalid("Preview unavailable")
	}
	kind, e := s.inspect(data)
	if e != nil {
		return nil, nil, e
	}
	v.ContentType = kind
	return v, data, nil
}
func (s *VendorDocumentService) Archive(ctx context.Context, t, supplier, id string, version int32, reason string) (*masterDomain.VendorDocumentDocument, error) {
	p, e := s.actor(ctx, t, "settings.supplier_documents.write")
	if e != nil {
		return nil, e
	}
	if !masterDomain.VendorDocumentValidID(supplier) || !masterDomain.VendorDocumentValidID(id) {
		return nil, masterDomain.VendorDocumentMissing
	}
	if !masterDomain.VendorDocumentValidReason(reason) {
		return nil, masterDomain.VendorDocumentFieldError("reason", "Archive reason is required")
	}
	var result *masterDomain.VendorDocumentDocument
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorDocumentRepository, d masterPorts.VendorDocumentSuppliers) error {
		ok, e := d.Exists(ctx, supplier)
		if e != nil {
			return e
		}
		if !ok {
			return masterDomain.VendorDocumentMissing
		}
		result, e = r.Archive(ctx, supplier, id, version, p.ID)
		if e != nil {
			return e
		}
		if result == nil {
			return &masterDomain.Error{Kind: masterDomain.Conflict, Detail: "Document changed; refresh before archiving"}
		}
		if e = r.Changed(ctx, *result, s.v.ID(), "archived", reason); e != nil {
			return e
		}
		if workflow, ok := r.(masterPorts.DocumentApprovalLifecycle); ok {
			return workflow.Retired(ctx, *result, p.ID, "archived")
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return result, nil
}

// Translate reviewed validation messages from the inspector into Master field errors.
func (s *VendorDocumentService) inspect(data []byte) (string, error) {
	kind, err := s.i.Inspect(data)
	var validation interface {
		error
		ValidationMessage() string
	}
	if errors.As(err, &validation) {
		return "", masterDomain.VendorDocumentFieldError("file", validation.ValidationMessage())
	}
	return kind, err
}
