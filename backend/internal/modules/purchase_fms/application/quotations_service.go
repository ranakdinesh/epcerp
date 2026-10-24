package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type QuotationsService struct {
	u         ports.QuotationsUnitOfWork
	a         ports.QuotationsAuthorizer
	v         ports.QuotationsValues
	cipher    ports.QuotationsCipher
	inspector ports.QuotationsInspector
}

func NewQuotations(u ports.QuotationsUnitOfWork, a ports.QuotationsAuthorizer, v ports.QuotationsValues, c ports.QuotationsCipher, i ports.QuotationsInspector) *QuotationsService {
	return &QuotationsService{u, a, v, c, i}
}
func (s *QuotationsService) actor(ctx context.Context, t string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, t)
	if e != nil {
		return a, e
	}
	if !a.Active || a.ID == "" {
		return a, domain.QuotationsUnauthorized
	}
	return a, nil
}
func (s *QuotationsService) workspace(ctx context.Context, r ports.QuotationsRepository, c ports.QuotationsContext, a domain.Principal, p, b string, write bool) (domain.QuotationsWorkspace, error) {
	source, e := c.Source(ctx, p, b)
	if e != nil {
		return domain.QuotationsWorkspace{}, e
	}
	record, e := r.Get(ctx, p, b)
	if e != nil {
		return domain.QuotationsWorkspace{}, e
	}
	out := domain.QuotationsWorkspace{TaxRate: domain.QuotationTaxRate, Documents: source.Documents, Currency: source.Currency, CanWrite: write && source.Available && a.Can("projects.group_quotations.read") && a.Can("projects.group_quotations.write"), History: []domain.QuotationsEvent{}, Comparisons: []domain.QuotationsComparison{}, ComparisonRevisions: []domain.QuotationsComparison{}, Revisions: []domain.QuotationsQuote{}, Attachments: []domain.QuotationsAttachment{}}
	quotes := []domain.QuotationsQuote{}
	if record != nil {
		quotes = record.Quotes
	}
	revisions, e := r.Revisions(ctx, p, b)
	if e != nil {
		return out, e
	}
	if a.Can("projects.group_quotations.read") {
		out.Record = record
		out.History, e = r.History(ctx, p, b)
		if e != nil {
			return out, e
		}
		out.Revisions = revisions
		for _, d := range source.Documents {
			files, e := r.Attachments(ctx, d.ID)
			if e != nil {
				return out, e
			}
			out.Attachments = append(out.Attachments, files...)
		}
	}
	if a.Can("projects.group_comparisons.read") {
		out.Comparisons = domain.QuotationsCompare(quotes, source.Documents, domain.QuotationsToday(s.v.Now()), source.Currency)
		out.ComparisonRevisions = domain.QuotationsCompare(revisions, source.Documents, domain.QuotationsToday(s.v.Now()), source.Currency)
	}
	return out, nil
}
func (s *QuotationsService) Workspace(ctx context.Context, t, p, b, section string) (out domain.QuotationsWorkspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	permission := "projects.group_quotations.read"
	view := "projects.group_quotations.view"
	if section == "comparison" {
		permission = "projects.group_comparisons.read"
		view = "projects.group_comparisons.view"
	} else if section != "receipt" {
		return out, domain.QuotationsInvalid("section", "Choose receipt or comparison; awards require the approved finance workflow")
	}
	if !a.Can(permission) || !a.Can(view) {
		return out, domain.QuotationsForbidden
	}
	if p, e = domain.QuotationsID(p); e != nil {
		return
	}
	if b, e = domain.QuotationsID(b); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationsRepository, c ports.QuotationsContext) error {
		write, e := c.Scope(ctx, a, p, permission, false)
		if e != nil {
			return e
		}
		out, e = s.workspace(ctx, r, c, a, p, b, write)
		return e
	})
	return
}
func (s *QuotationsService) Execute(ctx context.Context, t, p, b string, input domain.QuotationsCommand) (out domain.QuotationsWorkspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	if !a.Can("projects.group_quotations.read") || !a.Can("projects.group_quotations.write") {
		return out, domain.QuotationsForbidden
	}
	if p, e = domain.QuotationsID(p); e != nil {
		return
	}
	if b, e = domain.QuotationsID(b); e != nil {
		return
	}
	input, e = input.Validate()
	if e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationsRepository, c ports.QuotationsContext) error {
		write, e := c.Scope(ctx, a, p, "projects.group_quotations.write", true)
		if e != nil {
			return e
		}
		if !write {
			return domain.QuotationsForbidden
		}
		source, e := c.Source(ctx, p, b)
		if e != nil {
			return e
		}
		if !source.Available {
			return domain.QuotationsInvalid("", "Project purchasing is inactive, held or closed")
		}
		record, e := r.Get(ctx, p, b)
		if e != nil {
			return e
		}
		var before *domain.QuotationsRecord
		if record == nil {
			record = &domain.QuotationsRecord{ProjectID: p, BOID: b, Quotes: []domain.QuotationsQuote{}, Active: true}
		} else {
			copy := domain.QuotationsCopy(*record)
			before = &copy
		}
		if record.Version != input.Version {
			return domain.QuotationsStale
		}
		var doc *domain.QuotationsDocument
		for i := range source.Documents {
			if source.Documents[i].ID == input.Quote.DocumentID {
				doc = &source.Documents[i]
				break
			}
		}
		if doc == nil || doc.Data.SentDate == nil {
			return domain.QuotationsInvalid("document_id", "Record dispatch of this vendor indent before entering its quotation")
		}
		if !doc.Data.IsCurrent {
			return domain.QuotationsInvalid("document_id", "This indent scope is superseded; record the revised indent quotation")
		}
		if !doc.VendorAvailable {
			return domain.QuotationsInvalid("document_id", "Vendor is inactive or unavailable")
		}
		if guard, ok := r.(ports.QuotationsFreezeGuard); ok {
			pending, e := guard.PendingDocument(ctx, p, b, doc.ID)
			if e != nil {
				return e
			}
			if pending {
				return domain.QuotationsInvalid("document_id", "Waiting for Finance approval; group quotation information is frozen")
			}
		}
		q := domain.QuotationsQuote{QuotationsQuoteInput: input.Quote}
		if q.Currency != source.Currency {
			return domain.QuotationsInvalid("currency", "Quotes must use the project currency")
		}
		if q.DocumentNotProvided {
			if q.AttachmentID != nil {
				return domain.QuotationsInvalid("attachment_id", "Choose either a quotation file or document not provided")
			}
			if q.NoDocumentReason == "" {
				return domain.QuotationsInvalid("no_document_reason", "Record how prices were received without a quotation document")
			}
			q.EvidenceReference = ""
		} else {
			if q.AttachmentID == nil {
				return domain.QuotationsInvalid("attachment_id", "Upload and select a quotation file for this exact vendor indent")
			}
			file, e := r.Attachment(ctx, doc.ID, *q.AttachmentID)
			if e != nil {
				return e
			}
			if file == nil || !file.Active {
				return domain.QuotationsInvalid("attachment_id", "Select evidence belonging to this exact vendor indent")
			}
			file.Ciphertext = nil
			q.Attachment = file
			q.EvidenceReference = file.Filename
			q.NoDocumentReason = ""
		}
		revisions, e := r.Revisions(ctx, p, b)
		if e != nil {
			return e
		}
		q.Revision = 1
		for _, old := range revisions {
			if q.AttachmentID != nil && old.AttachmentID != nil && *q.AttachmentID == *old.AttachmentID {
				return domain.QuotationsInvalid("attachment_id", "Upload revised evidence before saving a new quotation revision")
			}
			if old.DocumentID == q.DocumentID && old.Revision >= q.Revision {
				q.Revision = old.Revision + 1
			}
		}
		q.QuotationsQuoteInput, e = domain.QuotationsNormalizeMoney(q.QuotationsQuoteInput)
		if e != nil {
			return e
		}
		q.Calculation, e = domain.QuotationsCalculate(q.QuotationsQuoteInput, *doc, domain.QuotationsToday(s.v.Now()))
		if e != nil {
			return e
		}
		now := s.v.Now()
		q.CreatedAt = now
		q.CreatedBy = a.ID
		q.UpdatedAt = now
		q.UpdatedBy = a.ID
		q.RecordedAt = now
		q.RecordedBy = a.ID
		q.Reason = input.Reason
		retained := []domain.QuotationsQuote{}
		for _, old := range record.Quotes {
			if old.DocumentID == q.DocumentID {
				q.CreatedAt = old.CreatedAt
				q.CreatedBy = old.CreatedBy
			} else {
				retained = append(retained, old)
			}
		}
		record.Quotes = append(retained, q)
		record.Version++
		saved, e := r.Save(ctx, *record, a.ID)
		if e != nil {
			return e
		}
		if e = r.Event(ctx, domain.QuotationsEvent{ID: s.v.ID(), BOID: b, Action: "quote_saved", Reason: input.Reason, Before: before, After: saved, ActorID: a.ID, CreatedAt: saved.UpdatedAt}); e != nil {
			return e
		}
		out, e = s.workspace(ctx, r, c, a, p, b, write)
		return e
	})
	return
}
func (s *QuotationsService) Upload(ctx context.Context, t, p, b, document, filename, mime string, data []byte) (out domain.QuotationsAttachment, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	if !a.Can("projects.group_quotations.read") || !a.Can("projects.group_quotations.write") {
		return out, domain.QuotationsForbidden
	}
	for _, id := range []*string{&p, &b, &document} {
		if *id, e = domain.QuotationsID(*id); e != nil {
			return
		}
	}
	if len(data) == 0 || len(data) > 10<<20 {
		return out, domain.QuotationsInvalid("file", "Upload nonempty quotation evidence of at most 10 MiB")
	}
	if e = s.inspector.Inspect(filename, mime, data); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationsRepository, c ports.QuotationsContext) error {
		write, e := c.Scope(ctx, a, p, "projects.group_quotations.write", true)
		if e != nil {
			return e
		}
		if !write {
			return domain.QuotationsForbidden
		}
		source, e := c.Source(ctx, p, b)
		if e != nil {
			return e
		}
		found := false
		for _, d := range source.Documents {
			if d.ID == document && d.Data.SentDate != nil && d.Data.IsCurrent && d.VendorAvailable {
				found = true
			}
		}
		if !source.Available || !found {
			return domain.QuotationsInvalid("document_id", "Upload only for a current dispatched indent while purchasing is available")
		}
		encrypted, hash, e := s.cipher.Seal(data)
		if e != nil {
			return e
		}
		v := domain.QuotationsAttachment{ID: s.v.ID(), DocumentID: document, Filename: filename, ContentType: mime, ByteSize: int64(len(data)), SHA256: hash, Active: true, CreatedBy: a.ID, Ciphertext: encrypted}
		out, e = r.InsertAttachment(ctx, v)
		if e != nil {
			return e
		}
		return r.AttachmentEvent(ctx, out, s.v.ID(), "uploaded", a.ID)
	})
	return
}
func (s *QuotationsService) Download(ctx context.Context, t, p, b, document, file string) (out domain.QuotationsAttachment, data []byte, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	if !a.Can("projects.group_quotations.read") {
		return out, nil, domain.QuotationsForbidden
	}
	for _, id := range []*string{&p, &b, &document, &file} {
		if *id, e = domain.QuotationsID(*id); e != nil {
			return
		}
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationsRepository, c ports.QuotationsContext) error {
		if _, e = c.Scope(ctx, a, p, "projects.group_quotations.read", false); e != nil {
			return e
		}
		source, e := c.Source(ctx, p, b)
		if e != nil {
			return e
		}
		found := false
		for _, d := range source.Documents {
			if d.ID == document {
				found = true
			}
		}
		if !found {
			return domain.QuotationsMissing
		}
		v, e := r.Attachment(ctx, document, file)
		if e != nil {
			return e
		}
		if v == nil {
			return domain.QuotationsMissing
		}
		data, e = s.cipher.Open(v.Ciphertext, v.SHA256)
		if e != nil {
			return e
		}
		v.Ciphertext = nil
		out = *v
		return r.AttachmentEvent(ctx, out, s.v.ID(), "downloaded", a.ID)
	})
	if e != nil {
		out = domain.QuotationsAttachment{}
		data = nil
	}
	return
}

// AuthorizeUpload is called by the HTTP adapter before reading multipart bytes.
// Upload repeats every live check inside its committing transaction.
func (s *QuotationsService) AuthorizeUpload(ctx context.Context, t, p, b, document string) (e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return e
	}
	if !a.Can("projects.group_quotations.read") || !a.Can("projects.group_quotations.write") {
		return domain.QuotationsForbidden
	}
	for _, id := range []*string{&p, &b, &document} {
		if *id, e = domain.QuotationsID(*id); e != nil {
			return e
		}
	}
	return s.u.Within(ctx, func(ctx context.Context, _ ports.QuotationsRepository, c ports.QuotationsContext) error {
		write, e := c.Scope(ctx, a, p, "projects.group_quotations.write", true)
		if e != nil {
			return e
		}
		if !write {
			return domain.QuotationsForbidden
		}
		source, e := c.Source(ctx, p, b)
		if e != nil {
			return e
		}
		if source.Available {
			for _, d := range source.Documents {
				if d.ID == document && d.Data.SentDate != nil && d.Data.IsCurrent && d.VendorAvailable {
					return nil
				}
			}
		}
		return domain.QuotationsInvalid("document_id", "Upload only for a current dispatched indent while purchasing is available")
	})
}

// BOChoices exposes only the navigation identifiers for this capability; it does
// not grant BO editing or expose its item list through quotation permission.
func (s *QuotationsService) BOChoices(ctx context.Context, t, p, section string) (out []domain.QuotationsBOChoice, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return nil, e
	}
	permission, view := "projects.group_quotations.read", "projects.group_quotations.view"
	if section == "comparison" {
		permission, view = "projects.group_comparisons.read", "projects.group_comparisons.view"
	} else if section != "receipt" {
		return nil, domain.QuotationsInvalid("section", "Choose receipt or comparison")
	}
	if !a.Can(permission) || !a.Can(view) {
		return nil, domain.QuotationsForbidden
	}
	if p, e = domain.QuotationsID(p); e != nil {
		return nil, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, _ ports.QuotationsRepository, c ports.QuotationsContext) error {
		if _, e := c.Scope(ctx, a, p, permission, false); e != nil {
			return e
		}
		out, e = c.BOChoices(ctx, p)
		return e
	})
	return
}
