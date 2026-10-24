package application

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

// ReconcileApprovals is an explicit operator action for pre-workflow documents.
// It preserves existing decisions and imports no source application data.
func (s *VendorDocumentService) ReconcileApprovals(ctx context.Context, token, supplier string) (domain.DocumentApprovalReconciliation, error) {
	p, e := s.actor(ctx, token, "settings.supplier_documents.write")
	if e != nil {
		return domain.DocumentApprovalReconciliation{}, e
	}
	if !p.Allows(domain.DocumentAutoApprovePermission) || !p.Allows("settings.supplier_documents.import") {
		return domain.DocumentApprovalReconciliation{}, domain.VendorDocumentForbidden
	}
	result := domain.DocumentApprovalReconciliation{}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.VendorDocumentRepository, d ports.VendorDocumentSuppliers) error {
		exists, e := d.Exists(ctx, supplier)
		if e != nil {
			return e
		}
		if !exists {
			return domain.VendorDocumentMissing
		}
		workflow, ok := r.(ports.DocumentApprovalLifecycle)
		if !ok {
			return domain.VendorDocumentInvalid("Document workflow unavailable")
		}
		documents, e := r.List(ctx, supplier)
		if e != nil {
			return e
		}
		latest := map[string]string{}
		for _, v := range documents {
			if v.Active && v.DocType != "other" {
				latest[v.DocType] = v.ID
			}
		}
		for _, v := range documents {
			if !v.Active {
				continue
			}
			if v.DocType != "other" && latest[v.DocType] != v.ID {
				retired, e := r.Archive(ctx, supplier, v.ID, v.Version, p.ID)
				if e != nil {
					return e
				}
				if e = r.Changed(ctx, *retired, s.v.ID(), "archived", "Replaced by the latest uploaded document"); e != nil {
					return e
				}
				if e = workflow.Retired(ctx, *retired, p.ID, "replaced"); e != nil {
					return e
				}
				result.Replaced++
				continue
			}
			stored, e := r.Get(ctx, supplier, v.ID)
			if e != nil {
				return e
			}
			if stored == nil {
				return domain.VendorDocumentMissing
			}
			encrypted := stored.Ciphertext
			if stored.ObjectKey != "" {
				if s.objects == nil {
					return domain.VendorDocumentIntegrity
				}
				encrypted, e = s.objects.Get(ctx, stored.ObjectBucket, stored.ObjectKey)
				if e != nil {
					return e
				}
			}
			plain, e := s.c.Open(encrypted, stored.SHA256)
			if e != nil {
				return e
			}
			if _, e = s.inspect(plain); e != nil {
				result.Unreadable++
				continue
			}
			if e = workflow.Reconciled(ctx, *stored, p.ID); e != nil {
				return e
			}
			result.Processed++
		}
		return nil
	})
	if e != nil {
		return domain.DocumentApprovalReconciliation{}, e
	}
	return result, nil
}
