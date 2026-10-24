package application

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

// DocumentApprovalService owns document decisions and their review/correction tasks.
// It is bound by composition to the uploading/reviewing transaction.
type DocumentApprovalService struct {
	r ports.DocumentApprovalRepository
	u ports.VendorVerificationUsers
}

func NewDocumentApprovalService(r ports.DocumentApprovalRepository, u ports.VendorVerificationUsers) *DocumentApprovalService {
	return &DocumentApprovalService{r, u}
}
func (s *DocumentApprovalService) Submitted(ctx context.Context, d domain.VendorDocumentDocument, actor string) error {
	return s.submit(ctx, d, actor, actor, "uploaded")
}
func (s *DocumentApprovalService) Reconciled(ctx context.Context, d domain.VendorDocumentDocument, actor string) error {
	p, e := s.u.Get(ctx, actor)
	if e != nil {
		return e
	}
	if p == nil || !p.Allows(domain.DocumentAutoApprovePermission) || !p.Allows("settings.supplier_documents.import") {
		return domain.VendorVerificationForbidden
	}
	existing, e := s.r.Get(ctx, d.ID)
	if e != nil {
		return e
	}
	if existing != nil {
		return nil
	}
	uploader := ""
	if d.CreatedBy != nil {
		uploader = *d.CreatedBy
	}
	return s.submit(ctx, d, uploader, actor, "reconciled")
}
func (s *DocumentApprovalService) submit(ctx context.Context, d domain.VendorDocumentDocument, uploader, actor, action string) error {
	p, e := s.u.Get(ctx, uploader)
	if e != nil {
		return e
	}
	if p == nil || !p.Active || !p.Allows("settings.supplier_documents.write") {
		return domain.VendorDocumentForbidden
	}
	state, comment := "pending", "Awaiting document review"
	if p.Allows(domain.DocumentAutoApprovePermission) {
		state = "accept"
		comment = "Automatically approved by upload permission"
	}
	if e = s.r.Save(ctx, domain.DocumentApproval{DocumentID: d.ID, SupplierID: d.SupplierID, DocumentVersion: d.Version, SHA256: d.SHA256, State: state, Comment: comment}, actor, action); e != nil {
		return e
	}
	if state == "accept" {
		return nil
	}
	reviewer, e := s.r.Reviewer(ctx, d.SupplierID)
	if e != nil {
		return e
	}
	if reviewer == "" {
		if candidates, ok := s.u.(ports.VendorReviewAssignmentUsers); ok {
			options, err := candidates.Candidates(ctx)
			if err != nil {
				return err
			}
			for _, option := range options {
				if option.Eligible {
					candidate, err := s.u.Get(ctx, option.ID)
					if err != nil {
						return err
					}
					if candidate != nil && !candidate.Superuser && candidate.Allows(domain.DocumentAutoApprovePermission) {
						if reviewer != "" {
							return domain.VendorVerificationFieldError("file", "Choose the supplier's Finance reviewer before uploading")
						}
						reviewer = option.ID
					}
				}
			}
		}
	}
	if reviewer == "" {
		return domain.VendorVerificationFieldError("file", "Assign an active Finance reviewer before uploading for approval")
	}
	p, e = s.u.Get(ctx, reviewer)
	if e != nil {
		return e
	}
	if p == nil || !p.Active || !p.Allows("accounts.vendor_verification.read") || !p.Allows("accounts.vendor_verification.write") {
		return domain.VendorVerificationFieldError("file", "Assign an active Finance reviewer before uploading for approval")
	}
	return s.r.Task(ctx, d.ID, d.SupplierID, reviewer, "review", "Review uploaded document", actor)
}
func (s *DocumentApprovalService) Decided(ctx context.Context, snapshot domain.VendorVerificationSnapshot, decisions []domain.VendorVerificationDecision, actor string) error {
	for _, v := range decisions {
		d, e := s.r.Document(ctx, snapshot.ID, v.DocumentID)
		if e != nil {
			return e
		}
		if d == nil || !d.Active {
			return domain.VendorDocumentMissing
		}
		previous, e := s.r.Get(ctx, d.ID)
		if e != nil {
			return e
		}
		if previous != nil && previous.State == v.Decision && previous.DocumentVersion == d.Version && previous.SHA256 == d.SHA256 {
			continue
		}
		if e = s.r.Save(ctx, domain.DocumentApproval{DocumentID: d.ID, SupplierID: d.SupplierID, DocumentVersion: d.Version, SHA256: d.SHA256, State: v.Decision, Comment: v.Comment}, actor, "reviewed"); e != nil {
			return e
		}
		outcome := "approved"
		if v.Decision == "reject" {
			outcome = "rejected"
		}
		if e = s.r.Complete(ctx, d.ID, "review", outcome, v.Comment, actor); e != nil {
			return e
		}
		if v.Decision == "reject" && d.CreatedBy != nil {
			if e = s.r.Task(ctx, d.ID, d.SupplierID, *d.CreatedBy, "correction", v.Comment, actor); e != nil {
				return e
			}
		} else if e = s.r.Complete(ctx, d.ID, "correction", "approved", v.Comment, actor); e != nil {
			return e
		}
	}
	return nil
}
func (s *DocumentApprovalService) Retired(ctx context.Context, d domain.VendorDocumentDocument, actor, outcome string) error {
	p, e := s.u.Get(ctx, actor)
	if e != nil {
		return e
	}
	if p == nil || !p.Allows("settings.supplier_documents.write") {
		return domain.VendorDocumentForbidden
	}
	for _, kind := range []string{"review", "correction"} {
		result := outcome
		if kind == "correction" && outcome == "replaced" {
			result = "resubmitted"
		}
		if e := s.r.Complete(ctx, d.ID, kind, result, "Document "+outcome, actor); e != nil {
			return e
		}
	}
	return s.r.Save(ctx, domain.DocumentApproval{DocumentID: d.ID, SupplierID: d.SupplierID, DocumentVersion: d.Version, SHA256: d.SHA256, State: "pending", Comment: "Document " + outcome}, actor, outcome)
}
