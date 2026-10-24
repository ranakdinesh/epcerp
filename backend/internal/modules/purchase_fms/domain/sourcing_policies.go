package domain

import (
	"strings"
	"time"
)

type SourcingCommercialPrice struct {
	DiscountPercent string `json:"discount_percent"`
	TaxPercent      string `json:"tax_percent"`
	Mode            string `json:"pricing_mode"`
	Rate            string `json:"unit_price"`
	Base            string `json:"base"`
	Discount        string `json:"discount"`
	Tax             string `json:"tax"`
	Charges         string `json:"allocated_charges"`
}
type SourcingProof struct {
	Commercial                                                                                     *SourcingCommercialPrice
	Reference, Fingerprint, VendorID, Quantity, Total, Currency, QuoteReference, EvidenceReference string
	Effective                                                                                      bool
}

func (s SourcingSource) Guard() error {
	for _, v := range []string{s.PackageID, s.DemandID} {
		if e := SourcingID(v); e != nil {
			return e
		}
	}
	if s.DemandVersion < 1 || !sourcingCurrencyPattern.MatchString(s.Currency) {
		return SourcingInvalid("source", "Use a current source revision and currency")
	}

	if !s.Available || !s.Eligible || len(s.Blockers) > 0 {
		return SourcingInvalid("source", "Current source controls block sourcing: "+strings.Join(s.Blockers, "; "))
	}
	if _, e := sourcingScaled("quantity", s.Quantity, 4, 19, true); e != nil {
		return e
	}
	return nil
}
func sourcingMember(members []SourcingMember, id string) bool {
	for _, m := range members {
		if m.ID == id {
			return true
		}
	}
	return false
}
func SourcingSupplierGuard(s *SourcingSupplier, onboarding bool) error {
	if s == nil || !s.Eligible {
		return SourcingInvalid("supplier_id", "Supplier must be active, qualified and linked to this item")
	}
	if onboarding && (strings.TrimSpace(s.Registration) == "" || strings.TrimSpace(s.Contact) == "" || strings.TrimSpace(s.Location) == "") {
		return SourcingInvalid("supplier_id", "Complete supplier registration, contact and location before LOI release")
	}
	return nil
}
func sourcingFindSupplier(rows []SourcingSupplier, id string) *SourcingSupplier {
	for _, s := range rows {
		if s.ID == id {
			return &s
		}
	}
	return nil
}
func SourcingSelected(c SourcingCase, today string) (*SourcingOffer, error) {
	for _, o := range c.Offers {
		if o.ID == c.SelectedOfferID {
			if o.State != "quoted" || !o.Compliant {
				return nil, SourcingInvalid("offer_id", "Select a received technically compliant quotation")
			}
			if o.ValidUntil == "" || o.ValidUntil < today {
				return nil, SourcingInvalid("valid_until", "Selected quotation expired; return to draft and revise it")
			}
			return &o, nil
		}
	}
	return nil, SourcingMissing
}
func SourcingAccessGuard(a Principal, caps SourcingAccess, c *SourcingCase, action string) error {
	if !a.Active || a.ID == "" {
		return SourcingUnauthorized
	}
	if !caps.Read {
		return SourcingForbidden
	}
	if action == "decide" {
		if !caps.Approve || c == nil || a.ID != c.ApproverID || a.ID == c.ProposedBy || c.State != "pending" {
			return SourcingForbidden
		}
		return nil
	}
	if !caps.Write {
		return SourcingForbidden
	}
	if action == "reopen" && caps.Cancel {
		return nil
	}
	if action == "configure" {
		if !caps.Configure {
			return SourcingForbidden
		}
		return nil
	}
	n := 4
	if action == "rfq" || action == "dispatch" || action == "no_bid" {
		n = 3
	} else if action == "release" {
		n = 5
	}
	if !caps.Work[n] {
		return SourcingForbidden
	}
	return nil
}
func sourcingSameSource(a, b SourcingSource) bool {
	return a.PackageID == b.PackageID && a.DemandID == b.DemandID && a.DemandVersion == b.DemandVersion && a.Quantity == b.Quantity && a.Currency == b.Currency
}
func sourcingProofGuard(c SourcingCase, o SourcingOffer, p *SourcingProof) error {
	if p == nil || !p.Effective || p.Reference == "" || p.Fingerprint == "" || p.VendorID != o.SupplierID || p.Currency != c.Source.Currency || p.QuoteReference == "" || p.EvidenceReference == "" || p.QuoteReference != o.QuoteReference || p.EvidenceReference != o.EvidenceReference {
		return SourcingInvalid("financial_approval", "A current matching Finance-approved quotation is required")
	}
	if p.Commercial == nil {
		return SourcingInvalid("financial_approval", "Current approved commercial price evidence is required")
	}
	full := p.Commercial
	fullTotal, err := SourcingTotal(SourcingOffer{PricingMode: full.Mode, LineTotal: sourcingDirectBase(*full), UnitPrice: sourcingUnitRate(*full), Discount: full.Discount, Tax: full.Tax, Freight: full.Charges, Other: "0"}, p.Quantity)
	approvedTotal, errTotal := SourcingAmount("financial_approval", p.Total)
	if err != nil || errTotal != nil || fullTotal != approvedTotal {
		return SourcingInvalid("financial_approval", "Approved quantity, price and total evidence must agree")
	}
	scopedPrice, e := SourcingScopePrice(*p.Commercial, c.Source.Quantity, p.Quantity)
	if e != nil {
		return e
	}
	price := &scopedPrice
	mode, e := sourcingPricingMode(o.PricingMode)
	if e != nil {
		return e
	}
	if mode != price.Mode {
		return SourcingInvalid("pricing_mode", "Pricing mode differs from the approved quotation")
	}
	if mode == "line_total" {
		base, e := SourcingAmount("line_total", o.LineTotal)
		approvedBase, err := SourcingAmount("line_total", price.Base)
		if e != nil || err != nil || base != approvedBase {
			return SourcingInvalid("line_total", "Line total differs from the approved quotation")
		}
	} else {
		rate, e := SourcingRate(o.UnitPrice)
		if e != nil {
			return e
		}
		approvedRate, e := SourcingRate(price.Rate)
		if e != nil || rate != approvedRate {
			return SourcingInvalid("unit_price", "Unit rate differs from the approved quotation")
		}
	}
	for _, v := range []struct{ field, actual, expected string }{{"discount", o.Discount, price.Discount}, {"tax", o.Tax, price.Tax}} {
		actual, e := SourcingAmount(v.field, v.actual)
		if e != nil {
			return e
		}
		expected, e := SourcingAmount(v.field, v.expected)
		if e != nil || actual != expected {
			return SourcingInvalid(v.field, "Amount differs from the approved quotation")
		}
	}
	freight, e := sourcingScaled("freight", o.Freight, 2, 16, false)
	if e != nil {
		return e
	}
	other, e := sourcingScaled("other", o.Other, 2, 16, false)
	if e != nil {
		return e
	}
	charges, e := sourcingScaled("charges", price.Charges, 2, 16, false)
	if e != nil || sourcingNewAmountSum(freight, other) != sourcingFormat(charges) {
		return SourcingInvalid("freight", "Allocated charges differ from the approved quotation")
	}
	total, e := SourcingTotal(o, c.Source.Quantity)
	if e != nil {
		return e
	}
	expected, e := SourcingTotal(SourcingOffer{PricingMode: price.Mode, LineTotal: sourcingDirectBase(*price), UnitPrice: sourcingUnitRate(*price), Discount: price.Discount, Tax: price.Tax, Freight: price.Charges, Other: "0"}, c.Source.Quantity)
	if e != nil || total != expected {
		return SourcingInvalid("financial_approval", "Finance approval must match the scoped selected quotation amount")
	}
	return nil
}

// Apply is source-state policy only. Current scope and evidence are supplied by the owning application ports.
func SourcingApply(base *SourcingCase, a Principal, caps SourcingAccess, q SourcingCommand, src SourcingSource, suppliers []SourcingSupplier, members []SourcingMember, proof *SourcingProof, now time.Time, newCaseID, newOfferID, project string) (SourcingCase, error) {
	var empty SourcingCase
	var e error
	q, e = q.Validate()
	if e != nil {
		return empty, e
	}
	if e = SourcingAccessGuard(a, caps, base, q.Action); e != nil {
		return empty, e
	}
	if e = src.Guard(); e != nil && q.Action != "reopen" {
		return empty, e
	}
	version := int32(0)
	if base != nil {
		version = base.Version
	}
	if q.Version != version {
		return empty, SourcingStale
	}
	today := SourcingToday(now)
	c := SourcingCase{}
	if base != nil {
		c = SourcingCopy(*base)
		if !c.Active {
			return empty, SourcingMissing
		}
		if !sourcingSameSource(c.Source, src) {
			return empty, SourcingInvalid("source", "Released source changed; review current material evidence")
		}
		if c.State == "released" {
			return empty, SourcingInvalid("state", "Released LOI is immutable")
		}
	} else {
		if q.Action != "configure" {
			return empty, SourcingInvalid("approver_id", "Configure an independent sourcing approver first")
		}
		if e = SourcingID(newCaseID); e != nil {
			return empty, e
		}
		if e = SourcingID(project); e != nil {
			return empty, e
		}
		c = SourcingCase{ID: newCaseID, ProjectID: project, PackageID: src.PackageID, Source: src, State: "draft", Offers: []SourcingOffer{}, Active: true, CreatedAt: now, CreatedBy: a.ID}
	}
	switch q.Action {
	case "configure":
		if c.State != "draft" {
			return empty, SourcingInvalid("state", "Return to draft before changing the approver")
		}
		if !sourcingMember(members, q.ApproverID) {
			return empty, SourcingInvalid("approver_id", "Select an active scoped sourcing approver")
		}
		c.ApproverID = q.ApproverID
	case "rfq", "dispatch", "no_bid", "quote":
		if c.State != "draft" {
			return empty, SourcingInvalid("state", "Return to draft before changing RFQs or quotations")
		}
		if e = sourcingOfferCommand(&c, q, suppliers, a.ID, now, newOfferID); e != nil {
			return empty, e
		}
	case "propose":
		if c.State != "draft" {
			return empty, SourcingInvalid("state", "Only draft sourcing may be submitted")
		}
		for _, o := range c.Offers {
			if o.State != "quoted" && o.State != "no_bid" {
				return empty, SourcingInvalid("offers", "Record a quotation or reasoned no-bid for every RFQ")
			}
		}
		if a.ID == c.ApproverID {
			return empty, SourcingInvalid("approver_id", "Proposer and approver must differ")
		}
		if !sourcingMember(members, c.ApproverID) {
			return empty, SourcingInvalid("approver_id", "Approver no longer has active project approval access")
		}
		c.SelectedOfferID = q.OfferID
		o, err := SourcingSelected(c, today)
		if err != nil {
			return empty, err
		}
		s := sourcingFindSupplier(suppliers, o.SupplierID)
		if e = SourcingSupplierGuard(s, false); e != nil {
			return empty, e
		}
		if e = sourcingProofGuard(c, *o, proof); e != nil {
			return empty, e
		}
		for i := range c.Offers {
			if c.Offers[i].ID == o.ID {
				c.Offers[i].Supplier = *s
				c.Offers[i].UpdatedAt = now
				c.Offers[i].UpdatedBy = a.ID
				o = &c.Offers[i]
			}
		}
		comparison, err := SourcingComparisons(c, today)
		if err != nil {
			return empty, err
		}
		c.SelectionReason = q.Reason
		c.LOITerms = q.LOITerms
		c.ProposedBy = a.ID
		c.State = "pending"
		c.Decision = &SourcingDecision{Source: c.Source, Offer: *o, Comparison: comparison, Terms: q.LOITerms, Reason: q.Reason, FinancialReference: proof.Reference + ":" + proof.Fingerprint}
	case "decide":
		if !sourcingMember(members, a.ID) {
			return empty, SourcingForbidden
		}
		o, err := SourcingSelected(c, today)
		if err != nil {
			return empty, err
		}
		s := sourcingFindSupplier(suppliers, o.SupplierID)
		if e = SourcingSupplierGuard(s, false); e != nil {
			return empty, e
		}
		if s.Version != o.Supplier.Version {
			return empty, SourcingInvalid("supplier_id", "Supplier changed after submission; return to draft")
		}
		if e = sourcingProofGuard(c, *o, proof); e != nil {
			return empty, e
		}
		if c.Decision == nil || c.Decision.FinancialReference != proof.Reference+":"+proof.Fingerprint {
			return empty, SourcingInvalid("financial_approval", "Financial evidence changed after proposal; return to draft")
		}
		if *q.Approve {
			c.State = "approved"
			c.ApprovedBy = a.ID
			c.ApprovedAt = &now
		} else {
			c.State = "draft"
			c.ApprovedBy = ""
			c.ApprovedAt = nil
			c.Decision = nil
		}
	case "reopen":
		if c.State != "pending" && c.State != "approved" {
			return empty, SourcingInvalid("state", "Only pending or approved sourcing may return to draft")
		}
		c.State = "draft"
		c.ApprovedBy = ""
		c.ApprovedAt = nil
		c.ProposedBy = ""
		c.Decision = nil
	case "release":
		if c.State != "approved" || c.ApprovedAt == nil || c.ApprovedBy == "" || c.ApprovedBy == c.ProposedBy {
			return empty, SourcingInvalid("state", "Independent approval is required before LOI release")
		}
		o, err := SourcingSelected(c, today)
		if err != nil {
			return empty, err
		}
		s := sourcingFindSupplier(suppliers, o.SupplierID)
		if e = SourcingSupplierGuard(s, true); e != nil {
			return empty, e
		}
		if s.Version != o.Supplier.Version {
			return empty, SourcingInvalid("supplier_id", "Supplier changed after selection; return for new approval")
		}
		if !sourcingMember(members, c.ApproverID) {
			return empty, SourcingInvalid("approver_id", "Approver access changed; return for new approval")
		}
		if e = sourcingProofGuard(c, *o, proof); e != nil {
			return empty, e
		}
		if c.Decision == nil || c.Decision.FinancialReference != proof.Reference+":"+proof.Fingerprint {
			return empty, SourcingInvalid("financial_approval", "Financial evidence changed; return for new approval")
		}
		if q.SentDate < SourcingToday(*c.ApprovedAt) || q.SentDate > today {
			return empty, SourcingInvalid("sent_date", "LOI dispatch must be on or after approval and not future")
		}
		c.LOIReference = "LOI-" + strings.ToUpper(strings.ReplaceAll(c.ID, "-", ""))
		c.LOISentDate = q.SentDate
		c.LOIChannel = q.Channel
		c.LOIDispatchReference = q.DispatchReference
		c.ReleasedAt = &now
		c.State = "released"
	}
	c.Version = version + 1
	c.UpdatedAt = now
	c.UpdatedBy = a.ID
	return c, nil
}
func sourcingOfferCommand(c *SourcingCase, q SourcingCommand, suppliers []SourcingSupplier, actor string, now time.Time, newID string) error {
	today := SourcingToday(now)
	i := -1
	for n, o := range c.Offers {
		if o.ID == q.OfferID {
			i = n
		}
	}
	if q.Action == "rfq" {
		s := sourcingFindSupplier(suppliers, q.SupplierID)
		if e := SourcingSupplierGuard(s, false); e != nil {
			return e
		}
		if q.DueDate < today {
			return SourcingInvalid("due_date", "RFQ due date must be today or later")
		}
		if q.OfferID != "" {
			if i < 0 || c.Offers[i].State != "draft" || c.Offers[i].SupplierID != q.SupplierID {
				return SourcingInvalid("offer_id", "Only an unsent RFQ for the same supplier may be edited")
			}
		} else {
			for _, o := range c.Offers {
				if o.SupplierID == q.SupplierID {
					return SourcingInvalid("supplier_id", "Supplier already has an RFQ in this package")
				}
			}
			if len(c.Offers) >= 50 {
				return SourcingInvalid("offers", "A sourcing case supports at most 50 suppliers")
			}
			if e := SourcingID(newID); e != nil {
				return e
			}
			c.Offers = append(c.Offers, SourcingOffer{ID: newID, SupplierID: q.SupplierID, State: "draft", Active: true, CreatedAt: now, CreatedBy: actor})
			i = len(c.Offers) - 1
		}
		c.Offers[i].DueDate = q.DueDate
		c.Offers[i].Terms = q.Terms
		c.Offers[i].Supplier = *s
	} else {
		if i < 0 {
			return SourcingMissing
		}
		o := &c.Offers[i]
		if q.Action != "no_bid" {
			if e := SourcingSupplierGuard(sourcingFindSupplier(suppliers, o.SupplierID), false); e != nil {
				return e
			}
		}
		switch q.Action {
		case "dispatch":
			if o.State != "draft" {
				return SourcingInvalid("state", "RFQ already sent")
			}
			if q.SentDate > today || q.SentDate < SourcingToday(o.CreatedAt) || q.SentDate > o.DueDate {
				return SourcingInvalid("sent_date", "RFQ dispatch must be within creation/due date and not future")
			}
			o.State = "sent"
			o.SentDate = q.SentDate
			o.Channel = q.Channel
			o.DispatchReference = q.DispatchReference
		case "no_bid":
			o.State = "no_bid"
			o.NoBidReason = q.Reason
		case "quote":
			if (o.State != "sent" && o.State != "quoted" && o.State != "no_bid") || o.SentDate == "" {
				return SourcingInvalid("state", "Record RFQ dispatch before entering a quotation")
			}
			if q.ValidUntil < today {
				return SourcingInvalid("valid_until", "Quotation validity cannot be past")
			}
			o.QuoteReference = q.QuoteReference
			o.EvidenceReference = q.EvidenceReference
			o.ValidUntil = q.ValidUntil
			o.PricingMode = q.PricingMode
			o.LineTotal = q.LineTotal
			o.UnitPrice = q.UnitPrice
			o.Discount = q.Discount
			o.Tax = q.Tax
			o.Freight = q.Freight
			o.Other = q.Other
			o.DeliveryDays = q.DeliveryDays
			o.PaymentTerms = q.PaymentTerms
			o.Warranty = q.Warranty
			o.Compliant = *q.Compliant
			o.ComplianceNotes = q.ComplianceNotes
			if _, e := SourcingTotal(*o, c.Source.Quantity); e != nil {
				return e
			}
			o.State = "quoted"
			o.NoBidReason = ""
		}
	}
	c.Offers[i].UpdatedAt = now
	c.Offers[i].UpdatedBy = actor
	return nil
}
