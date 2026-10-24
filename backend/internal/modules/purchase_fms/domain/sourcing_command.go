package domain

import (
	"slices"
)

func (q SourcingCommand) Validate() (SourcingCommand, error) {
	if q.Version < 0 {
		return q, SourcingInvalid("version", "Use the current version")
	}
	var e error
	q.Reason, e = SourcingText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	if !slices.Contains([]string{"configure", "rfq", "dispatch", "no_bid", "quote", "propose", "decide", "reopen", "release"}, q.Action) {
		return q, SourcingInvalid("action", "Choose a sourcing action")
	}
	checkID := func(s string) error { return SourcingID(s) }
	text := func(f string, p *string, n int) error {
		var err error
		*p, err = SourcingText(f, *p, n, true)
		return err
	}
	switch q.Action {
	case "configure":
		e = checkID(q.ApproverID)
	case "rfq":
		if e = checkID(q.SupplierID); e == nil && q.OfferID != "" {
			e = checkID(q.OfferID)
		}
		if e == nil {
			_, e = SourcingDate("due_date", q.DueDate)
		}
		if e == nil {
			e = text("rfq_terms", &q.Terms, 4000)
		}
	case "no_bid":
		e = checkID(q.OfferID)
	case "dispatch", "release":
		if q.Action == "dispatch" {
			e = checkID(q.OfferID)
		}
		if e == nil {
			_, e = SourcingDate("sent_date", q.SentDate)
		}
		if e == nil && !slices.Contains([]string{"email", "whatsapp", "manual"}, q.Channel) {
			e = SourcingInvalid("channel", "Choose email, whatsapp or manual")
		}
		if e == nil {
			e = text("dispatch_reference", &q.DispatchReference, 1000)
		}
	case "quote":
		if e = checkID(q.OfferID); e != nil {
			break
		}
		if _, e = SourcingDate("valid_until", q.ValidUntil); e != nil {
			break
		}
		for _, r := range []struct {
			f string
			p *string
			n int
		}{{"quote_reference", &q.QuoteReference, 200}, {"evidence_reference", &q.EvidenceReference, 1000}, {"payment_terms", &q.PaymentTerms, 2000}, {"warranty", &q.Warranty, 1000}, {"compliance_notes", &q.ComplianceNotes, 2000}} {
			if e = text(r.f, r.p, r.n); e != nil {
				break
			}
		}
		if e != nil {
			break
		}
		if q.DeliveryDays < 0 || q.DeliveryDays > 3650 {
			e = SourcingInvalid("delivery_days", "Use 0–3650 delivery days")
			break
		}
		if q.Compliant == nil {
			e = SourcingInvalid("compliant", "Record an explicit technical compliance decision")
			break
		}
		q.PricingMode, e = sourcingPricingMode(q.PricingMode)
		if e != nil {
			break
		}
		if q.PricingMode == "line_total" {
			if q.UnitPrice != "" {
				e = SourcingInvalid("unit_price", "Use only the direct line total")
				break
			}
			q.LineTotal, e = SourcingAmount("line_total", q.LineTotal)
		} else {
			if q.LineTotal != "" {
				e = SourcingInvalid("line_total", "Use only the unit rate")
				break
			}
			q.UnitPrice, e = SourcingRate(q.UnitPrice)
		}
		if e != nil {
			break
		}
		for _, v := range []struct {
			f string
			p *string
		}{{"discount", &q.Discount}, {"tax", &q.Tax}, {"freight", &q.Freight}, {"other", &q.Other}} {
			if v.f == "unit_price" {
				*v.p, e = SourcingRate(*v.p)
			} else {
				*v.p, e = SourcingAmount(v.f, *v.p)
			}
			if e != nil {
				break
			}
		}
	case "propose":
		e = checkID(q.OfferID)
		if e == nil {
			e = text("loi_terms", &q.LOITerms, 4000)
		}
	case "decide":
		if q.Approve == nil {
			e = SourcingInvalid("approve", "Record an explicit approval or rejection")
		}
	}
	return q, e
}
