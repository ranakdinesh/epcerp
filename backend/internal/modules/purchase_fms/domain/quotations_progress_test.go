package domain

import (
	"testing"
)

func TestQuotationsQuotationProgressContainsPricingCoverageAndDetachedSource(t *testing.T) {
	q, doc := quotationsFixture()
	calc, e := QuotationsCalculate(q, doc, "2026-10-10")
	if e != nil {
		t.Fatal(e)
	}
	r := QuotationsRecord{ProjectID: "project", BOID: "bo", Active: true, Quotes: []QuotationsQuote{{QuotationsQuoteInput: q, Calculation: calc, Revision: 1}}}
	p, e := QuotationsProgressFacts(&r, "project", "bo")
	if e != nil || len(p.Quotes) != 1 || !p.Quotes[0].Lines[0].Priced {
		t.Fatal(p, e)
	}
	*p.Quotes[0].Lines[0].Source.Quantity = "999"
	if *r.Quotes[0].Calculation.Lines[0].Source.Quantity != "3" {
		t.Fatal("aliased quotation source")
	}
	r.Quotes[0].Calculation.Lines[0].Landed = nil
	p, e = QuotationsProgressFacts(&r, "project", "bo")
	if e != nil || p.Quotes[0].Lines[0].Priced {
		t.Fatal("missing line price counted", p, e)
	}
	if _, e = QuotationsProgressFacts(&r, "foreign", "bo"); e == nil {
		t.Fatal("foreign quotation counted")
	}
	r.Quotes = append(r.Quotes, r.Quotes[0])
	if _, e = QuotationsProgressFacts(&r, "project", "bo"); e == nil {
		t.Fatal("duplicate quotation counted")
	}
}
