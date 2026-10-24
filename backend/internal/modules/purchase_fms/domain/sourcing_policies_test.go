package domain

import (
	"testing"
	"time"
)

const sourcingOperator = "11111111-1111-4111-8111-111111111111"
const sourcingApprover = "22222222-2222-4222-8222-222222222222"
const sourcingSupplier = "33333333-3333-4333-8333-333333333333"
const sourcingCaseID = "44444444-4444-4444-8444-444444444444"
const sourcingOfferID = "55555555-5555-4555-8555-555555555555"
const sourcingPackageID = "66666666-6666-4666-8666-666666666666"
const sourcingProjectID = "77777777-7777-4777-8777-777777777777"

func sourcingBoolean(v bool) *bool { return &v }
func sourcingSetup() (Principal, SourcingAccess, SourcingSource, []SourcingSupplier, []SourcingMember, time.Time) {
	return Principal{ID: sourcingOperator, Active: true, Superuser: true}, SourcingAccess{Read: true, Write: true, Approve: true, Configure: true, Work: map[int]bool{3: true, 4: true, 5: true}}, SourcingSource{PackageID: sourcingPackageID, DemandID: sourcingOfferID, DemandVersion: 1, Quantity: "2.0050", Currency: "INR", Available: true, Eligible: true, Blockers: []string{}}, []SourcingSupplier{{ID: sourcingSupplier, Version: 1, Eligible: true, Registration: "REG", Contact: "Contact", Location: "Site"}}, []SourcingMember{{ID: sourcingApprover}}, time.Date(2026, 10, 10, 19, 0, 0, 0, time.UTC)
}
func TestSourcingSourceSourcingLifecycleIndependentApprovalAndCurrentFinancialProof(t *testing.T) {
	a, caps, src, suppliers, members, now := sourcingSetup()
	var c *SourcingCase
	run := func(q SourcingCommand) {
		t.Helper()
		q.Reason = "Source lifecycle evidence"
		if c != nil {
			q.Version = c.Version
		}
		proof := &SourcingProof{Commercial: &SourcingCommercialPrice{Mode: "unit_price", Rate: "10", Base: "20.05", Discount: "0", Tax: "2", Charges: "1.01"}, Reference: "approved-line", Fingerprint: "current", VendorID: sourcingSupplier, Quantity: "2.0050", Total: "23.06", Currency: "INR", Effective: true, QuoteReference: "QUOTE", EvidenceReference: "Vendor evidence"}
		v, e := SourcingApply(c, a, caps, q, src, suppliers, members, proof, now, sourcingCaseID, sourcingOfferID, sourcingProjectID)
		if e != nil {
			t.Fatal(q.Action, e)
		}
		c = &v
	}
	run(SourcingCommand{Action: "configure", ApproverID: sourcingApprover})
	run(SourcingCommand{Action: "rfq", SupplierID: sourcingSupplier, DueDate: "2026-10-11", Terms: "Full specified quantity"})
	run(SourcingCommand{Action: "dispatch", OfferID: sourcingOfferID, SentDate: "2026-10-11", Channel: "manual", DispatchReference: "Recorded RFQ dispatch"})
	run(SourcingCommand{Action: "quote", OfferID: sourcingOfferID, QuoteReference: "QUOTE", EvidenceReference: "Vendor evidence", ValidUntil: "2026-10-20", UnitPrice: "10", Discount: "0", Tax: "2", Freight: "1", Other: "0.01", PaymentTerms: "Receipt", Warranty: "12 months", Compliant: sourcingBoolean(true), ComplianceNotes: "Matches specified item"})
	run(SourcingCommand{Action: "propose", OfferID: sourcingOfferID, LOITerms: "Controlled LOI terms"})
	if c.Decision == nil || c.Decision.Offer.UnitPrice != "10.0000" {
		t.Fatal("missing normalized immutable decision")
	}
	pending := SourcingCopy(*c)
	if _, e := SourcingApply(c, a, caps, SourcingCommand{Version: c.Version, Action: "decide", Reason: "Try self approval", Approve: sourcingBoolean(true)}, src, suppliers, members, nil, now, sourcingCaseID, sourcingOfferID, sourcingProjectID); e == nil {
		t.Fatal("superadmin self approval allowed")
	}
	a.ID = sourcingApprover
	run(SourcingCommand{Action: "decide", Approve: sourcingBoolean(true)})
	if c.State != "approved" || pending.State != "pending" {
		t.Fatal("approval mutated prior snapshot")
	}
	a.ID = sourcingOperator
	run(SourcingCommand{Action: "release", SentDate: "2026-10-11", Channel: "manual", DispatchReference: "Recorded LOI dispatch"})
	if c.State != "released" || c.LOIReference != "LOI-44444444444444448444444444444444" {
		t.Fatal("durable release missing")
	}
	if _, e := SourcingApply(c, a, caps, SourcingCommand{Version: c.Version, Action: "reopen", Reason: "Try change release"}, src, suppliers, members, nil, now, sourcingCaseID, sourcingOfferID, sourcingProjectID); e == nil {
		t.Fatal("released mutation allowed")
	}
}
func TestSourcingExactAbsoluteChargesAndTiedRanking(t *testing.T) {
	o := SourcingOffer{ID: sourcingOfferID, State: "quoted", Compliant: true, ValidUntil: "2026-10-11", UnitPrice: "10.00", Discount: "0.00", Tax: "2.00", Freight: "1.00", Other: "0.01"}
	total, e := SourcingTotal(o, "2.0050")
	if e != nil || total != "23.06" {
		t.Fatal(total, e)
	}
	x := o
	x.ID = sourcingCaseID
	rows, e := SourcingComparisons(SourcingCase{Source: SourcingSource{Quantity: "2.0050"}, Offers: []SourcingOffer{o, x}}, "2026-10-12")
	if e != nil || *rows[0].Rank != 1 || *rows[1].Rank != 1 || !rows[0].Expired {
		t.Fatal("rank/expiry", e)
	}
	for _, s := range []string{"NaN", "Inf", "-1", "1e2", "10.001", "100000000000000.00"} {
		if _, e := SourcingAmount("unit_price", s); e == nil {
			t.Fatal("unsafe amount", s)
		}
	}
	o.Discount = "21.00"
	if _, e := SourcingTotal(o, "2.0050"); e == nil {
		t.Fatal("excess discount")
	}
}
func TestSourcingProofCannotReuseDifferentVendorQuantityAmountOrCurrency(t *testing.T) {
	_, _, src, _, _, _ := sourcingSetup()
	o := SourcingOffer{SupplierID: sourcingSupplier, QuoteReference: "QUOTE", EvidenceReference: "Vendor evidence", UnitPrice: "10", Discount: "0", Tax: "0", Freight: "0", Other: "0"}
	c := SourcingCase{Source: src}
	good := SourcingProof{Commercial: &SourcingCommercialPrice{Mode: "unit_price", Rate: "10", Base: "20.05", Discount: "0", Tax: "0", Charges: "0"}, Reference: "approval", Fingerprint: "current", VendorID: sourcingSupplier, Quantity: "2.0050", Total: "20.05", Currency: "INR", Effective: true, QuoteReference: "QUOTE", EvidenceReference: "Vendor evidence"}
	if e := sourcingProofGuard(c, o, &good); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*SourcingProof){func(p *SourcingProof) { p.Quantity = "2" }, func(p *SourcingProof) { p.Total = "20.04" }, func(p *SourcingProof) { p.VendorID = sourcingOperator }, func(p *SourcingProof) { p.Currency = "USD" }, func(p *SourcingProof) { p.Effective = false }, func(p *SourcingProof) { p.QuoteReference = "OTHER" }, func(p *SourcingProof) { p.EvidenceReference = "Other revision" }} {
		bad := good
		mutate(&bad)
		if e := sourcingProofGuard(c, o, &bad); e == nil {
			t.Fatal("unrelated financial proof accepted")
		}
	}
}
func TestSourcingCurrentSourceAndPermissionsCannotBeBypassed(t *testing.T) {
	a, caps, src, suppliers, members, now := sourcingSetup()
	q := SourcingCommand{Action: "configure", ApproverID: sourcingApprover, Reason: "Configure"}
	for _, v := range []SourcingSource{func() SourcingSource { s := src; s.Available = false; return s }(), func() SourcingSource { s := src; s.Blockers = []string{"Held"}; return s }()} {
		if _, e := SourcingApply(nil, a, caps, q, v, suppliers, members, nil, now, sourcingCaseID, sourcingOfferID, sourcingProjectID); e == nil {
			t.Fatal("source bypass")
		}
	}
	caps.Configure = false
	if _, e := SourcingApply(nil, a, caps, q, src, suppliers, members, nil, now, sourcingCaseID, sourcingOfferID, sourcingProjectID); e == nil {
		t.Fatal("claimed superuser bypassed actual scope")
	}
	if _, e := (SourcingCommand{Action: "decide", Reason: "Missing boolean"}).Validate(); e == nil {
		t.Fatal("missing decision became rejection")
	}
}

func TestSourcingDecisionCopyPreservesHistoricalPointers(t *testing.T) {
	rank := 1
	at := sourcingSetupTime()
	original := SourcingCase{ApprovedAt: &at, ReleasedAt: &at, Decision: &SourcingDecision{Comparison: []SourcingComparison{{Rank: &rank}}}}
	copied := SourcingCopy(original)
	*copied.Decision.Comparison[0].Rank = 9
	*copied.ApprovedAt = at.AddDate(0, 0, 1)
	*copied.ReleasedAt = at.AddDate(0, 0, 2)
	if *original.Decision.Comparison[0].Rank != 1 || !original.ApprovedAt.Equal(at) || !original.ReleasedAt.Equal(at) {
		t.Fatal("copy rewrote immutable decision evidence")
	}
}
func sourcingSetupTime() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) }

func TestSourcingGoQuotationRatePrecisionRetainsExactMoneyRounding(t *testing.T) {
	o := SourcingOffer{UnitPrice: "12.3456", Discount: "0", Tax: "22.23", Freight: "0", Other: "0"}
	v, e := SourcingTotal(o, "10")
	if e != nil || v != "145.69" {
		t.Fatal("Go quotation rate interoperability", v, e)
	}
	if _, e := SourcingRate("1.23456"); e == nil {
		t.Fatal("excess rate precision")
	}
}

func TestSourcingApprovedPricingRejectsEqualGrossSubstitution(t *testing.T) {
	_, _, src, _, _, _ := sourcingSetup()
	src.Quantity = "10"
	price := &SourcingCommercialPrice{Mode: "unit_price", Rate: "12.3456", Base: "123.46", Discount: "0", Tax: "22.22", Charges: "0.01"}
	p := &SourcingProof{Commercial: price, Reference: "approval", Fingerprint: "current", VendorID: sourcingSupplier, Quantity: "10", Total: "145.69", Currency: "INR", Effective: true, QuoteReference: "QUOTE", EvidenceReference: "Doc:2"}
	o := SourcingOffer{SupplierID: sourcingSupplier, QuoteReference: "QUOTE", EvidenceReference: "Doc:2", UnitPrice: "12.3456", Discount: "0", Tax: "22.22", Freight: "0.01", Other: "0"}
	c := SourcingCase{Source: src}
	if e := sourcingProofGuard(c, o, p); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*SourcingOffer){func(o *SourcingOffer) { o.UnitPrice = "12.3455" }, func(o *SourcingOffer) { o.Tax = "22.23"; o.Freight = "0" }, func(o *SourcingOffer) { o.Discount = "0.01"; o.Other = "0.01" }} {
		v := o
		change(&v)
		total, e := SourcingTotal(v, "10")
		if e != nil || total != "145.69" {
			t.Fatal("Adversarial gross changed", total, e)
		}
		if e = sourcingProofGuard(c, v, p); e == nil {
			t.Fatal("Changed approved commercial breakdown accepted")
		}
	}
	missing := *p
	missing.Commercial = nil
	if e := sourcingProofGuard(c, o, &missing); e == nil {
		t.Fatal("Missing approved pricing accepted")
	}
}

func TestSourcingDirectLineTotalPreservesExactApprovedBase(t *testing.T) {
	offer := SourcingOffer{SupplierID: sourcingSupplier, PricingMode: "line_total", LineTotal: "1.00", Discount: "0", Tax: "0.18", Freight: "0.01", Other: "0", QuoteReference: "Q", EvidenceReference: "document:1"}
	source := SourcingSource{Quantity: "30000.0000", Currency: "INR"}
	proof := &SourcingProof{Effective: true, Reference: "approved", Fingerprint: "current", VendorID: sourcingSupplier, Currency: "INR", Quantity: source.Quantity, Total: "1.19", QuoteReference: "Q", EvidenceReference: "document:1", Commercial: &SourcingCommercialPrice{Mode: "line_total", Rate: "0.0000", Base: "1.00", Discount: "0.00", Tax: "0.18", Charges: "0.01"}}
	if total, err := SourcingTotal(offer, source.Quantity); err != nil || total != "1.19" {
		t.Fatal(total, err)
	}
	if err := sourcingProofGuard(SourcingCase{Source: source}, offer, proof); err != nil {
		t.Fatal(err)
	}
	altered := offer
	altered.LineTotal = "0.99"
	altered.Other = "0.01"
	if err := sourcingProofGuard(SourcingCase{Source: source}, altered, proof); err == nil {
		t.Fatal("same gross with changed direct base accepted")
	}
	altered = offer
	altered.PricingMode = "unit_price"
	altered.LineTotal = ""
	altered.UnitPrice = "0.0001"
	if err := sourcingProofGuard(SourcingCase{Source: source}, altered, proof); err == nil {
		t.Fatal("approved direct total replaced by unit rate")
	}
	altered = offer
	altered.UnitPrice = "0.0001"
	if _, err := SourcingTotal(altered, source.Quantity); err == nil {
		t.Fatal("ambiguous price inputs accepted")
	}
}

func TestSourcingReceivedDirectPriceValidation(t *testing.T) {
	q := SourcingCommand{Action: "quote", Reason: "Receive exact direct amount", OfferID: sourcingOfferID, QuoteReference: "Q", EvidenceReference: "document:1", ValidUntil: "2026-10-20", PricingMode: "line_total", LineTotal: "1", Discount: "0", Tax: "0.18", Freight: "0.01", Other: "0", PaymentTerms: "Receipt", Warranty: "12 months", Compliant: sourcingBoolean(true), ComplianceNotes: "Meets specification"}
	got, err := q.Validate()
	if err != nil || got.LineTotal != "1.00" || got.UnitPrice != "" {
		t.Fatal(got, err)
	}
	for _, mode := range []string{"line_total", "unit_price", "invalid"} {
		bad := q
		bad.PricingMode = mode
		bad.UnitPrice = "1"
		if _, err := bad.Validate(); err == nil {
			t.Fatal("ambiguous or unknown pricing accepted", mode)
		}
	}
	bad := q
	bad.LineTotal = "1.001"
	if _, err := bad.Validate(); err == nil {
		t.Fatal("over-precise direct total accepted")
	}
}
