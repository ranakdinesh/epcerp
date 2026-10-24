package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ProgressGroupedProviders struct {
	Projects  projects.ContextReader
	BOs       contracts.BOReader
	Groups    contracts.IndentGroupsReader
	Documents contracts.IndentDocumentsReader
	Quotes    contracts.QuotationsProgressReader
	Approvals contracts.QuotationApprovalsProgressReader
}
type ProgressGroupedReader struct {
	c database.Connection
	p ProgressGroupedProviders
}

func NewProgressGroupedReader(c database.Connection, p ProgressGroupedProviders) *ProgressGroupedReader {
	return &ProgressGroupedReader{c, p}
}
func progressValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Canonicalize the complete business snapshot published by both BO and document
// owners. BO-only demand version is verified separately by the work-package source.
func progressItemSnapshot(id, sku, description, key, uom, manufacturer string) string {
	v := struct{ ID, SKU, Description, EngineeringKey, UOM, Manufacturer string }{id, sku, description, key, uom, manufacturer}
	data, _ := json.Marshal(v)
	return string(data)
}
func (r *ProgressGroupedReader) Read(ctx context.Context, p, b string) ([]domain.ProgressEvidence, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Grouped progress requires the consumer transaction")
	}
	for _, id := range []string{p, b} {
		if e := domain.ProgressID(id); e != nil {
			return nil, e
		}
	}
	if r.p.Projects == nil || r.p.BOs == nil || r.p.Groups == nil || r.p.Documents == nil || r.p.Quotes == nil || r.p.Approvals == nil {
		return nil, errors.New("Grouped progress providers are not configured")
	}
	project, e := r.p.Projects.Get(ctx, p, true)
	if e != nil {
		return nil, e
	}
	if project == nil {
		return nil, &domain.ProgressError{Status: 404, Field: "project", Message: "Project not found"}
	}
	record, e := r.p.BOs.Get(ctx, p, b)
	if e != nil {
		return nil, e
	}
	if record == nil || !record.Active {
		return nil, &domain.ProgressError{Status: 404, Field: "bo", Message: "BO list not found"}
	}
	if record.ProjectID != p || record.ID != b {
		return nil, domain.ProgressInvalid("source", "BO reporting source scope changed")
	}
	group, e := r.p.Groups.Get(ctx, p, b)
	if e != nil {
		return nil, e
	}
	if group == nil || !group.Active {
		return []domain.ProgressEvidence{}, nil
	}
	if group.ProjectID != p || group.BOID != b {
		return nil, domain.ProgressInvalid("source", "Group reporting source scope changed")
	}
	source := domain.ProgressGroupedSource{BOState: record.State, GroupState: group.State, Groups: []domain.ProgressGroupedGroup{}}
	for _, line := range record.Lines {
		i := line.Snapshot
		source.Lines = append(source.Lines, domain.ProgressGroupedLine{ID: line.ID, Quantity: progressValue(line.Quantity), RequiredDate: progressValue(line.RequiredDate), Snapshot: progressItemSnapshot(i.ID, i.SKU, i.Description, i.EngineeringKey, i.UOM, i.Manufacturer)})
	}
	for _, g := range group.Groups {
		source.Groups = append(source.Groups, domain.ProgressGroupedGroup{ID: g.ID, Revision: g.ScopeRevision, LineIDs: append([]string{}, g.LineIDs...), VendorIDs: append([]string{}, g.VendorIDs...)})
	}
	docs, e := r.p.Documents.List(ctx, p, b)
	if e != nil {
		return nil, e
	}
	currentDocs := map[string]domain.ProgressGroupedDocument{}
	for _, doc := range docs {
		if doc.ProjectID != p || doc.BOID != b {
			return nil, domain.ProgressInvalid("source", "Document reporting source scope changed")
		}
		out := domain.ProgressGroupedDocument{ID: doc.ID, GroupID: doc.GroupID, VendorID: doc.VendorID, Version: doc.Version, Revision: doc.Data.ScopeRevision, Active: doc.Active, SentDate: progressValue(doc.Data.SentDate)}
		for _, line := range doc.Data.Lines {
			i := line.Snapshot
			out.Lines = append(out.Lines, domain.ProgressGroupedLine{ID: line.ID, Quantity: progressValue(line.Quantity), RequiredDate: progressValue(line.RequiredDate), Snapshot: progressItemSnapshot(i.ID, i.SKU, i.Description, i.EngineeringKey, i.UOM, i.Manufacturer)})
		}
		source.Documents = append(source.Documents, out)
		if doc.Active {
			currentDocs[doc.ID] = out
		}
	}
	quoted, e := r.p.Quotes.Progress(ctx, p, b)
	if e != nil {
		return nil, e
	}
	if quoted.ProjectID != p || quoted.BOID != b {
		return nil, domain.ProgressInvalid("source", "Quotation reporting source scope changed")
	}
	for _, quote := range quoted.Quotes {
		doc, exists := currentDocs[quote.DocumentID]
		if !exists {
			continue
		}
		_, e := time.Parse("2006-01-02", quote.ReceivedDate)
		if e != nil {
			continue
		}
		row := domain.ProgressGroupedQuote{DocumentID: quote.DocumentID, DocumentVersion: doc.Version, Active: true}
		byID := map[string]domain.ProgressGroupedLine{}
		for _, line := range doc.Lines {
			byID[line.ID] = line
		}
		for _, line := range quote.Lines {
			i := line.Source.Snapshot
			out := domain.ProgressGroupedLine{ID: line.Source.ID, Quantity: progressValue(line.Source.Quantity), RequiredDate: progressValue(line.Source.RequiredDate), Snapshot: progressItemSnapshot(i.ID, i.SKU, i.Description, i.EngineeringKey, i.UOM, i.Manufacturer)}
			if line.Priced && domain.ProgressGroupedLineMatches(out, byID[out.ID]) {
				row.PricedLineIDs = append(row.PricedLineIDs, out.ID)
			}
		}
		source.Quotes = append(source.Quotes, row)
	}
	finance, e := r.p.Approvals.Progress(ctx, p, b)
	if e != nil {
		return nil, e
	}
	if finance.ProjectID != p || finance.BOID != b {
		return nil, domain.ProgressInvalid("source", "Finance reporting source scope changed")
	}
	source.ApprovedGroupIDs = append([]string{}, finance.ApprovedGroupIDs...)
	source.ApprovalStarted = finance.Started
	return domain.ProgressGroupedFacts(source)
}
