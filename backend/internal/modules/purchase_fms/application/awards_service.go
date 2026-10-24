package application

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

// AwardsService owns the approved-group handoff, not quotation or Finance approval.
type AwardsService struct {
	u ports.AwardsUnitOfWork
	a ports.WorkflowAuthorizer
	v ports.WorkflowValues
}

func NewAwards(u ports.AwardsUnitOfWork, a ports.WorkflowAuthorizer, v ports.WorkflowValues) *AwardsService {
	return &AwardsService{u, a, v}
}
func (s *AwardsService) actor(ctx context.Context, token, p string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.WorkflowUnauthorized
	}
	if e = domain.WorkflowID(p); e != nil {
		return a, e
	}
	return a, nil
}
func (s *AwardsService) Workspace(ctx context.Context, token, p string) (out domain.AwardWorkspace, e error) {
	a, e := s.actor(ctx, token, p)
	if e != nil {
		return out, e
	}
	e = s.u.WithinAwards(ctx, func(ctx context.Context, x ports.AwardsSession) error {
		if err := x.Context.Authorize(ctx, a, p, false); err != nil {
			return err
		}
		rows, err := x.Context.Candidates(ctx, p)
		if err != nil {
			return err
		}
		out.Rows = []domain.AwardRow{}
		out.Members = []domain.WorkflowMember{}
		out.CanWrite = x.Context.Authorize(ctx, a, p, true) == nil
		if out.CanWrite {
			out.Members, err = x.WorkContext.Members(ctx, p)
			if err != nil {
				return err
			}
		}
		for _, c := range rows {
			row := domain.AwardRow{ID: c.Attempt.ID, BOID: c.BOID, Group: c.Attempt.GroupName, Vendor: c.Attempt.Vendor, Reference: c.Attempt.Reference, Currency: c.Attempt.Currency, Total: c.Attempt.Total, Items: len(c.Lines), State: "ready", Packages: []domain.AwardPackage{}}
			h, err := x.Awards.Get(ctx, p, c.Attempt.ID)
			if err != nil {
				return err
			}
			if h != nil {
				docs, err := x.Documents.List(ctx, p, c.Attempt.ID)
				if err != nil {
					return err
				}
				row.LOIReviewComplete = len(docs) > 0 && docs[0].Review != nil
				row.State = "loi_pending"
				row.Packages = append([]domain.AwardPackage{}, h.Packages...)
				all := len(row.Packages) > 0
				for i, pkg := range row.Packages {
					record, err := x.Sourcing.Get(ctx, p, pkg.ID)
					if err != nil {
						return err
					}
					if record != nil {
						row.Packages[i].State = record.State
						row.Packages[i].LOIReference = record.LOIReference
					}
					all = all && record != nil && record.State == "released"
				}
				if all {
					row.State = "loi_released"
				}
			}
			out.Rows = append(out.Rows, row)
		}
		return nil
	})
	if e != nil {
		out = domain.AwardWorkspace{}
	}
	return
}
func (s *AwardsService) Execute(ctx context.Context, token, p string, q domain.AwardCommand) (out domain.AwardHandoff, e error) {
	a, e := s.actor(ctx, token, p)
	if e != nil {
		return out, e
	}
	for _, id := range []string{q.AttemptID, q.DoerID, q.ResolverID} {
		if e = domain.WorkflowID(id); e != nil {
			return out, e
		}
	}
	q.Reason, e = domain.WorkflowText("reason", q.Reason, 2000, true)
	if e != nil {
		return out, e
	}
	if !domain.WorkflowDate(q.PlannedDate) {
		return out, domain.WorkflowInvalid("planned_date", "Choose a valid planned date")
	}
	e = s.u.WithinAwards(ctx, func(ctx context.Context, x ports.AwardsSession) error {
		if err := x.Context.Authorize(ctx, a, p, true); err != nil {
			return err
		}
		if err := x.Awards.Lock(ctx, p); err != nil {
			return err
		}
		candidates, err := x.Context.Candidates(ctx, p)
		if err != nil {
			return err
		}
		var candidate *domain.AwardCandidate
		for i := range candidates {
			if candidates[i].Attempt.ID == q.AttemptID {
				candidate = &candidates[i]
				break
			}
		}
		if candidate == nil {
			return domain.WorkflowInvalid("attempt_id", "Use a current Finance-approved group; refresh changed evidence")
		}
		existing, err := x.Awards.Get(ctx, p, q.AttemptID)
		if err != nil {
			return err
		}
		if existing != nil {
			out = *existing
			return nil
		}
		approved := candidate.Attempt
		if approved.ReviewedBy == nil || approved.ReviewedAt == nil || *approved.ReviewedBy == approved.SubmittedBy || len(candidate.Lines) == 0 {
			return domain.WorkflowInvalid("approval", "Independent Finance approval evidence is incomplete")
		}
		members, err := x.WorkContext.Members(ctx, p)
		if err != nil {
			return err
		}
		// Assignment policy validates live doer/resolver permissions and distinct identities.
		plan := []domain.WorkflowAssignment{}
		for n := 3; n <= 20; n++ {
			plan = append(plan, domain.WorkflowAssignment{Number: n, Team: "Procurement / project team", DoerID: q.DoerID, ResolverID: q.ResolverID, PlannedDate: q.PlannedDate})
		}
		now := s.v.Now()
		_, err = domain.WorkflowApplyAssignments(domain.WorkflowPackage{Steps: domain.WorkflowDefinitions()}, plan, members, a.ID, now)
		if err != nil {
			return err
		}
		all, err := x.Indents.List(ctx, p)
		if err != nil {
			return err
		}
		indentID := s.v.ID()
		input := domain.IndentsInput{Currency: approved.Currency, DoerID: q.DoerID, ResolverID: q.ResolverID, Documents: "Approved quotation " + approved.Reference + " / " + approved.QuoteKey, Lines: []domain.IndentsLine{}}
		released := []domain.IndentsRelease{}
		versions := map[string]int32{}
		for _, line := range candidate.Lines {
			source, err := x.IndentContext.Line(ctx, p, line.DemandID)
			if err != nil {
				return err
			}
			if source.Item.ID != line.Approval.Item.ID {
				return domain.WorkflowStale
			}
			budget := "0"
			if line.Approval.Budget != nil {
				budget = *line.Approval.Budget
			}
			il := domain.IndentsLine{DemandID: line.DemandID, Quantity: line.Approval.Quantity, Budget: budget, FullKit: "Vendor drawings, datasheets, QAP and applicable test certificates for " + line.Approval.Item.Description}
			committed := int64(0)
			for _, other := range all {
				for _, r := range other.Data.Released {
					if r.DemandID == line.DemandID {
						v, err := domain.IndentsQuantity(r.Quantity)
						if err != nil {
							return err
						}
						if committed > 9223372036854775807-v {
							return domain.WorkflowInvalid("quantity", "Released quantity exceeds supported range")
						}
						committed += v
					}
				}
			}
			if err = domain.IndentsReleaseGuard(il, source, committed); err != nil {
				return err
			}
			input.Lines = append(input.Lines, il)
			versions[line.DemandID] = source.DemandVersion
			released = append(released, domain.IndentsRelease{IndentsLine: il, At: now, ActorID: a.ID, Evidence: "Finance-approved award " + approved.ID, Source: source})
		}
		if _, err = (domain.IndentsSave{Data: input, Version: 1, Reason: q.Reason}).Validate(); err != nil {
			return err
		}
		if err = x.IndentContext.Validate(ctx, p, input); err != nil {
			return err
		}
		indent, err := x.Indents.Save(ctx, domain.IndentsRecord{ID: indentID, ProjectID: p, Version: 1, Active: true, Data: domain.IndentsData{IndentsInput: input, Number: domain.IndentsNumber(indentID), State: "released", Released: released, Review: &domain.IndentsReview{ActorID: *approved.ReviewedBy, At: *approved.ReviewedAt, Evidence: "Existing Finance approval " + approved.ID, DemandVersions: versions}}}, a.ID)
		if err != nil {
			return err
		}
		if err = x.Indents.Event(ctx, domain.IndentsEvent{ID: s.v.ID(), SubjectID: indent.ID, Action: "approved_award_handoff", Reason: q.Reason, After: indent, ActorID: a.ID, CreatedAt: indent.UpdatedAt}); err != nil {
			return err
		}
		out = domain.AwardHandoff{ID: s.v.ID(), ProjectID: p, BOID: candidate.BOID, AttemptID: approved.ID, CreatedBy: a.ID, CreatedAt: now, Packages: []domain.AwardPackage{}}
		for i, line := range candidate.Lines {
			source, err := x.WorkContext.Source(ctx, p, indent.ID, line.DemandID)
			if err != nil {
				return err
			}
			if err = source.Guard(); err != nil {
				return err
			}
			pkgID := s.v.ID()
			pkg := domain.WorkflowPackage{ID: pkgID, ProjectID: p, IndentID: indent.ID, DemandID: line.DemandID, Code: "PF-" + pkgID, Description: source.Description, Source: source, Version: 1, Active: true, CreatedBy: &a.ID, UpdatedBy: &a.ID, CreatedAt: now, UpdatedAt: now, Steps: domain.WorkflowDefinitions()}
			pkg, err = domain.WorkflowApplyAssignments(pkg, plan, members, a.ID, now)
			if err != nil {
				return err
			}
			for n := range pkg.Steps {
				if n < 2 {
					pkg.Steps[n].State = "source_linked"
				}
			}
			pkg, err = x.Workflow.Save(ctx, pkg, a.ID)
			if err != nil {
				return err
			}
			if err = x.Workflow.Event(ctx, domain.WorkflowEvent{ID: s.v.ID(), Action: "create", Reason: q.Reason, After: pkg, At: pkg.UpdatedAt, ActorID: a.ID}); err != nil {
				return err
			}
			src, err := x.SourceContext.Source(ctx, p, pkgID)
			if err != nil {
				return err
			}
			if err = src.Guard(); err != nil {
				return err
			}
			suppliers, err := x.SourceContext.Suppliers(ctx, p, pkgID)
			if err != nil {
				return err
			}
			var supplier *domain.SourcingSupplier
			for n := range suppliers {
				if suppliers[n].ID == approved.VendorID {
					supplier = &suppliers[n]
				}
			}
			if err = domain.SourcingSupplierGuard(supplier, true); err != nil {
				return err
			}
			price := line.Price
			offer := domain.SourcingOffer{ID: s.v.ID(), SupplierID: approved.VendorID, Supplier: *supplier, State: "quoted", QuoteReference: approved.Reference, EvidenceReference: approved.QuoteKey, ValidUntil: approved.ValidUntil, PricingMode: price.Mode, Discount: price.Discount, Tax: price.Tax, Freight: price.Charges, Other: "0", DeliveryDays: line.DeliveryDays, PaymentTerms: line.PaymentTerms, Warranty: line.Warranty, Compliant: true, ComplianceNotes: "Existing approved quotation selection", Active: true, CreatedAt: now, UpdatedAt: now, CreatedBy: a.ID, UpdatedBy: a.ID}
			if price.Mode == "line_total" {
				offer.LineTotal = price.Base
			} else {
				offer.UnitPrice = price.Rate
			}
			proof, err := x.SourceContext.VerifySelection(ctx, p, pkgID, offer)
			if err != nil {
				return err
			}
			if approved.ValidUntil < domain.SourcingToday(now) {
				return domain.SourcingInvalid("valid_until", "Approved quotation expired; revise it before award")
			}
			record := domain.SourcingCase{AwardReference: approved.ID, ID: s.v.ID(), ProjectID: p, PackageID: pkgID, Source: src, ApproverID: *approved.ReviewedBy, Version: 1, State: "draft", Offers: []domain.SourcingOffer{offer}, Active: true, CreatedAt: now, UpdatedAt: now, CreatedBy: a.ID, UpdatedBy: a.ID}
			record, err = x.Sourcing.Save(ctx, record, a.ID)
			if err != nil {
				return err
			}
			if err = x.Sourcing.Event(ctx, domain.SourcingEvent{ID: s.v.ID(), CaseID: record.ID, Action: "configure", Reason: "Prepare existing Finance-approved award " + approved.ID, After: record, At: record.UpdatedAt, ActorID: a.ID}); err != nil {
				return err
			}
			before := domain.SourcingCopy(record)
			record.State = "approved"
			record.Version++
			record.SelectedOfferID = offer.ID
			record.SelectionReason = approved.Reason
			record.LOITerms = approved.Terms
			record.ProposedBy = approved.SubmittedBy
			record.ApprovedBy = *approved.ReviewedBy
			record.ApprovedAt = approved.ReviewedAt
			record.Decision = &domain.SourcingDecision{Source: src, Offer: offer, Terms: record.LOITerms, Reason: record.SelectionReason, FinancialReference: proof.Reference + ":" + proof.Fingerprint, Comparison: []domain.SourcingComparison{{OfferID: offer.ID, Total: line.Approval.Quoted}}}
			balances, err := x.Sourcing.LockAllocation(ctx, p, line.DemandID, record.ID, record.Decision.FinancialReference)
			if err != nil {
				return err
			}
			allocation, err := domain.SourcingPlanAllocation(src.Quantity, proof.Quantity, proof.Total, balances.Quantity, balances.Amount, line.Approval.Quoted)
			if err != nil {
				return err
			}
			record, err = x.Sourcing.Save(ctx, record, a.ID)
			if err != nil {
				return err
			}
			if err = x.Sourcing.StoreAllocation(ctx, record, proof, allocation); err != nil {
				return err
			}
			if err = x.Sourcing.Event(ctx, domain.SourcingEvent{ID: s.v.ID(), CaseID: record.ID, Action: "award", Reason: q.Reason, Before: &before, After: record, At: record.UpdatedAt, ActorID: a.ID}); err != nil {
				return err
			}
			out.Packages = append(out.Packages, domain.AwardPackage{ID: pkgID, Description: released[i].Source.Item.Description, Quantity: src.Quantity, UOM: released[i].Source.Item.UOM, State: "approved"})
		}
		return x.Awards.Save(ctx, out, q.Reason)
	})
	if e != nil {
		out = domain.AwardHandoff{}
	}
	return
}
