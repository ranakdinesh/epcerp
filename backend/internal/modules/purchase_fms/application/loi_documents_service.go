package application

import (
	"context"
	"errors"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

// LOIDocumentsService prepares immutable vendor documents; it never dispatches,
// changes a quotation or records a client/manufacturing clearance.
type LOIDocumentsService struct {
	u           ports.AwardsUnitOfWork
	a           ports.WorkflowAuthorizer
	v           ports.WorkflowValues
	printer     ports.LOIPrinter
	fingerprint ports.LOIFingerprinter
	cipher      ports.TechnicalCipher
	objects     ports.OrdersObjects
	bucket      string
}

func NewLOIDocuments(u ports.AwardsUnitOfWork, a ports.WorkflowAuthorizer, v ports.WorkflowValues, p ports.LOIPrinter, f ports.LOIFingerprinter, c ports.TechnicalCipher, o ports.OrdersObjects, b string) *LOIDocumentsService {
	return &LOIDocumentsService{u, a, v, p, f, c, o, b}
}
func (s *LOIDocumentsService) actor(ctx context.Context, token, p, attempt string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil {
		return a, e
	}
	if !a.Active {
		return a, domain.WorkflowUnauthorized
	}
	for _, id := range []string{p, attempt} {
		if e = domain.WorkflowID(id); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *LOIDocumentsService) Workspace(ctx context.Context, token, p, attempt string) (out domain.LOIWorkspace, e error) {
	a, e := s.actor(ctx, token, p, attempt)
	if e != nil {
		return out, e
	}
	e = s.u.WithinAwards(ctx, func(ctx context.Context, x ports.AwardsSession) error {
		if e := x.Context.Authorize(ctx, a, p, false); e != nil {
			return e
		}
		h, e := x.Awards.Get(ctx, p, attempt)
		if e != nil {
			return e
		}
		if h == nil {
			return domain.WorkflowInvalid("award", "Prepare the approved award before generating its LOI")
		}
		out.Documents, e = x.Documents.List(ctx, p, attempt)
		if e != nil {
			return e
		}
		out.DefaultReference = "LOI/VTPL/" + h.ID[:8]
		out.Today = domain.SourcingToday(s.v.Now())
		out.CanGenerate = x.Context.Authorize(ctx, a, p, true) == nil
		return nil
	})
	return
}
func (s *LOIDocumentsService) Generate(ctx context.Context, token, p, attempt string, q domain.LOICommand) (out domain.LOIDocument, e error) {
	a, e := s.actor(ctx, token, p, attempt)
	if e != nil {
		return out, e
	}
	today := domain.SourcingToday(s.v.Now())
	q, e = q.Validate(today)
	if e != nil {
		return out, e
	}
	e = s.u.WithinAwards(ctx, func(ctx context.Context, x ports.AwardsSession) error {
		if e := x.Context.Authorize(ctx, a, p, true); e != nil {
			return e
		}
		if e := x.Awards.Lock(ctx, p); e != nil {
			return e
		}
		h, e := x.Awards.Get(ctx, p, attempt)
		if e != nil {
			return e
		}
		if h == nil {
			return domain.WorkflowInvalid("award", "Prepare the approved award before generating its LOI")
		}
		candidates, e := x.Context.Candidates(ctx, p)
		if e != nil {
			return e
		}
		var candidate *domain.AwardCandidate
		for i := range candidates {
			if candidates[i].Attempt.ID == attempt {
				candidate = &candidates[i]
			}
		}
		if candidate == nil {
			return domain.WorkflowInvalid("approval", "Refresh the current Finance approval and supplier documents before generating this LOI")
		}
		approved := candidate.Attempt
		if approved.ReviewedAt == nil || q.Date < domain.SourcingToday(*approved.ReviewedAt) {
			return domain.WorkflowInvalid("date", "LOI date must be on or after Finance approval")
		}
		content, e := x.DocumentContext.Metadata(ctx, p, approved.VendorID)
		if e != nil {
			return e
		}
		content.LOICommand = q
		content.TemplateVersion = domain.LOITemplateVersion
		content.Group = approved.GroupName
		content.Quote = approved.Reference
		content.Currency = approved.Currency
		content.Total = approved.Total
		content.ApprovedTerms = approved.Terms
		content.Items = []domain.LOIItem{}
		content.SourceHash, e = s.fingerprint.Approval(approved)
		if e != nil {
			return e
		}
		for _, line := range candidate.Lines {
			var record *domain.SourcingCase
			for _, pkg := range h.Packages {
				r, err := x.Sourcing.Get(ctx, p, pkg.ID)
				if err != nil {
					return err
				}
				if r != nil && r.Source.DemandID == line.DemandID {
					record = r
				}
			}
			if record == nil || record.AwardReference != attempt || (record.State != "approved" && record.State != "released") {
				return domain.WorkflowInvalid("award", "Current approved item handoff is unavailable")
			}
			source, err := x.SourceContext.Source(ctx, p, record.PackageID)
			if err != nil {
				return err
			}
			if err = source.Guard(); err != nil {
				return err
			}
			if source.DemandVersion != record.Source.DemandVersion || source.Quantity != record.Source.Quantity {
				return domain.WorkflowStale
			}
			offer, err := domain.SourcingSelected(*record, today)
			if err != nil {
				return err
			}
			suppliers, err := x.SourceContext.Suppliers(ctx, p, record.PackageID)
			if err != nil {
				return err
			}
			var supplier *domain.SourcingSupplier
			for i := range suppliers {
				if suppliers[i].ID == approved.VendorID {
					supplier = &suppliers[i]
				}
			}
			if err = domain.SourcingSupplierGuard(supplier, true); err != nil {
				return err
			}
			if offer.SupplierID != approved.VendorID || supplier.Version != offer.Supplier.Version {
				return domain.WorkflowStale
			}
			proof, err := x.SourceContext.VerifySelection(ctx, p, record.PackageID, *offer)
			if err != nil {
				return err
			}
			if proof == nil || !proof.Effective || record.Decision == nil || record.Decision.FinancialReference != proof.Reference+":"+proof.Fingerprint {
				return domain.WorkflowStale
			}
			item := line.Approval.Item
			specs, err := x.DocumentContext.Specifications(ctx, item.ID)
			if err != nil {
				return err
			}
			content.Items = append(content.Items, domain.LOIItem{Description: item.Description, SKU: item.SKU, Make: item.Manufacturer, UOM: item.UOM, Quantity: line.Approval.Quantity, Price: line.Price, Total: line.Approval.Quoted, Payment: line.PaymentTerms, Warranty: line.Warranty, DeliveryDays: line.DeliveryDays, Technical: specs})
		}
		if _, e = domain.LOITotals(content); e != nil {
			return e
		}
		requestHash, e := s.fingerprint.Content(content)
		if e != nil {
			return e
		}
		previous, e := x.Documents.List(ctx, p, attempt)
		if e != nil {
			return e
		}
		version := int32(1)
		for _, old := range previous {
			if old.RequestHash == requestHash {
				out = old
				return nil
			}
			if old.Version >= version {
				version = old.Version + 1
			}
		}
		data, e := s.printer.Render(ctx, content, version)
		if e != nil {
			return e
		}
		encrypted, digest, e := s.cipher.Seal(data)
		if e != nil {
			return e
		}
		id := s.v.ID()
		objectKey := "vendor-lois/" + p + "/" + h.ID + "/" + id
		if e = s.objects.Put(ctx, s.bucket, objectKey, encrypted); e != nil {
			return e
		}
		out = domain.LOIDocument{ID: id, ProjectID: p, HandoffID: h.ID, AttemptID: attempt, Version: version, Reference: q.Reference, Date: q.Date, Format: q.Format, Notes: q.Notes, RequestHash: requestHash, Content: content, SHA256: digest, Bucket: s.bucket, ObjectKey: objectKey, Size: int64(len(data)), CreatedBy: a.ID, CreatedAt: s.v.Now()}
		if e = x.Documents.Save(ctx, out); e != nil {
			return e
		}
		return x.Documents.Event(ctx, out, "generated", a.ID)
	})
	if e != nil {
		out = domain.LOIDocument{}
	}
	return
}
func (s *LOIDocumentsService) Download(ctx context.Context, token, p, attempt, id string) (data []byte, name string, e error) {
	a, e := s.actor(ctx, token, p, attempt)
	if e != nil {
		return nil, "", e
	}
	if e = domain.WorkflowID(id); e != nil {
		return nil, "", e
	}
	e = s.u.WithinAwards(ctx, func(ctx context.Context, x ports.AwardsSession) error {
		if e := x.Context.Authorize(ctx, a, p, false); e != nil {
			return e
		}
		d, e := x.Documents.Get(ctx, p, attempt, id)
		if e != nil {
			return e
		}
		if d == nil {
			return domain.WorkflowMissing
		}
		encrypted, e := s.objects.Get(ctx, d.Bucket, d.ObjectKey)
		if e != nil {
			return e
		}
		data, e = s.cipher.Open(encrypted, d.SHA256)
		if e != nil {
			return e
		}
		if int64(len(data)) != d.Size {
			return errors.New("LOI file integrity check failed")
		}
		name = d.Filename()
		return x.Documents.Event(ctx, *d, "downloaded", a.ID)
	})
	if e != nil {
		return nil, "", e
	}
	return
}
