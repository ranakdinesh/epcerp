package domain

import (
	"testing"
)

func quotationsFixture() (QuotationsQuoteInput, QuotationsDocument) {
	sent := "2026-10-01"
	quantity := "3"
	months := int32(12)
	price := "0.3350"
	l := QuotationsLineInput{LineID: "11111111-1111-4111-8111-111111111111", UnitPrice: &price, PricingMode: "unit_price", CoverageType: "warranty", WarrantyMonths: &months, Discount: "50", Tax: "18", Compliant: true}
	d := QuotationsDocument{ID: "22222222-2222-4222-8222-222222222222", GroupID: "33333333-3333-4333-8333-333333333333", VendorAvailable: true, Data: QuotationsDocumentData{IsCurrent: true, SentDate: &sent, Lines: []QuotationsSourceLine{{ID: l.LineID, Quantity: &quantity}}}}
	q := QuotationsQuoteInput{DocumentID: d.ID, Reference: "Q1", ReceivedDate: sent, ValidUntil: "2026-10-31", Currency: "INR", Freight: "0", OtherCharges: "0", DeliveryTerms: "Ex works", PaymentTerms: "After delivery", Lines: []QuotationsLineInput{l}}
	return q, d
}
func TestQuotationsPythonHalfUpPricing(t *testing.T) {
	q, d := quotationsFixture()
	c, e := QuotationsCalculate(q, d, "2026-10-10")
	if e != nil {
		t.Fatal(e)
	}
	l := c.Lines[0]
	// Python's Decimal ROUND_HALF_UP at each stage: 3*.335 -> 1.01;
	// 50% reduction -> .51; net .50; GST .09.
	if *l.Base != "1.01" || *l.DiscountAmount != "0.51" || *l.Net != "0.50" || *l.TaxAmount != "0.09" || c.Total != "0.59" || !c.Complete || !c.Compliant {
		t.Fatalf("unexpected exact calculation: %+v", c)
	}
	q.Lines[0].PricingMode = "line_total"
	q.Lines[0].UnitPrice = nil
	q.Lines[0].LineTotal = quotationsStr("10.00")
	q.Lines[0].Discount = "0"
	q.Lines[0].Tax = "0"
	c, e = QuotationsCalculate(q, d, "2026-10-10")
	if e != nil {
		t.Fatal(e)
	}
	if c.Total != "10.00" || *c.Lines[0].UnitPrice != "3.33" {
		t.Fatalf("direct amount changed: %+v", c)
	}
}
func TestQuotationsPythonLargestRemainderAndZeroNet(t *testing.T) {
	for _, zero := range []bool{false, true} {
		q, d := quotationsFixture()
		q.Freight = "0.02"
		q.Lines = nil
		d.Data.Lines = nil
		ids := []string{"11111111-1111-4111-8111-111111111111", "44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555"}
		for _, id := range ids {
			base, _ := quotationsFixture()
			l := base.Lines[0]
			l.LineID = id
			l.Discount = "0"
			l.Tax = "0"
			l.UnitPrice = quotationsStr("1")
			if zero {
				l.Discount = "100"
			}
			q.Lines = append(q.Lines, l)
			qty := "1"
			d.Data.Lines = append(d.Data.Lines, QuotationsSourceLine{ID: id, Quantity: &qty})
		}
		// Browser order must not alter the source-order remainder tie-break.
		q.Lines[0], q.Lines[2] = q.Lines[2], q.Lines[0]
		c, e := QuotationsCalculate(q, d, "2026-10-10")
		if e != nil {
			t.Fatal(e)
		}
		for i, want := range []string{"0.01", "0.01", "0.00"} {
			if *c.Lines[i].AllocatedCharges != want {
				t.Fatalf("zero=%t line=%d allocation=%s", zero, i, *c.Lines[i].AllocatedCharges)
			}
		}
		want := "3.02"
		if zero {
			want = "0.02"
		}
		if c.Total != want {
			t.Fatalf("total %s want %s", c.Total, want)
		}
	}
}
func TestQuotationsQuotationValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*QuotationsQuoteInput, *QuotationsDocument)
	}{
		{"future receipt", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.ReceivedDate = "2026-10-11" }},
		{"before dispatch", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.ReceivedDate = "2026-09-30" }},
		{"expiry before receipt", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.ValidUntil = "2026-09-30" }},
		{"missing item", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines = nil }},
		{"duplicate item", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines = append(q.Lines, q.Lines[0]) }},
		{"unexplained partial", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines[0].UnitPrice = nil }},
		{"no warranty", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines[0].WarrantyMonths = nil }},
		{"negative price", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines[0].UnitPrice = quotationsStr("-1") }},
		{"extra decimals", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines[0].UnitPrice = quotationsStr("1.00001") }},
		{"double pricing", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines[0].LineTotal = quotationsStr("1") }},
		{"discount overflow", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines[0].Discount = "100.01" }},
		{"tax overflow", func(q *QuotationsQuoteInput, d *QuotationsDocument) { q.Lines[0].Tax = "100.01" }},
		{"zero quantity", func(q *QuotationsQuoteInput, d *QuotationsDocument) { d.Data.Lines[0].Quantity = quotationsStr("0") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, d := quotationsFixture()
			tc.change(&q, &d)
			if _, e := QuotationsCalculate(q, d, "2026-10-10"); e == nil {
				t.Fatal("invalid quotation accepted")
			}
		})
	}
}
func TestQuotationsPartialAndComparisonExclusions(t *testing.T) {
	q, d := quotationsFixture()
	line := q.Lines[0]
	line.LineID = "44444444-4444-4444-8444-444444444444"
	line.UnitPrice = nil
	line.Notes = "Vendor cannot supply this SKU"
	q.Lines = append(q.Lines, line)
	d.Data.Lines = append(d.Data.Lines, QuotationsSourceLine{ID: line.LineID, Quantity: quotationsStr("1")})
	c, e := QuotationsCalculate(q, d, "2026-10-10")
	if e != nil {
		t.Fatal(e)
	}
	if c.Complete || c.Lines[1].Landed != nil {
		t.Fatal("partial quote treated as complete")
	}
	d.Data.IsCurrent = false
	d.VendorAvailable = false
	q.ValidUntil = "2026-10-09"
	q.Currency = "USD"
	c.Compliant = false
	rows := QuotationsCompare([]QuotationsQuote{{QuotationsQuoteInput: q, Calculation: c}}, []QuotationsDocument{d}, "2026-10-10", "INR")
	if len(rows) != 1 || rows[0].Eligible || rows[0].Rank != nil || rows[0].Suggested || len(rows[0].Exclusions) != 6 {
		t.Fatalf("unsafe comparison: %+v", rows)
	}
}

func TestQuotationsAcceptsOneOverallIndentTotalWithoutItemPrices(t *testing.T) {
	q, d := quotationsFixture()
	q.AmountMode = "total_only"
	q.TotalAmount = quotationsStr("125000.50")
	q.Freight = "1200"
	q.OtherCharges = "300"
	q.Lines[0] = QuotationsLineInput{LineID: q.Lines[0].LineID}

	command, e := (QuotationsCommand{Version: 0, Reason: "Vendor supplied one overall total", Quote: q}).Validate()
	if e != nil {
		t.Fatal(e)
	}
	if command.Quote.Freight != "0" || command.Quote.OtherCharges != "0" {
		t.Fatal("total-only quotation retained separate charges")
	}
	c, e := QuotationsCalculate(command.Quote, d, "2026-10-10")
	if e != nil {
		t.Fatal(e)
	}
	if c.Total != "125000.50" || !c.Complete || c.Compliant || len(c.Lines) != 1 || c.Lines[0].Landed != nil {
		t.Fatalf("unexpected total-only calculation: %+v", c)
	}
	rows := QuotationsCompare([]QuotationsQuote{{QuotationsQuoteInput: command.Quote, Calculation: c}}, []QuotationsDocument{d}, "2026-10-10", "INR")
	if len(rows) != 1 || rows[0].Eligible || len(rows[0].Exclusions) != 2 {
		t.Fatalf("total-only quotation bypassed item approval controls: %+v", rows)
	}
}

func TestQuotationsRejectsInvalidOverallTotalMode(t *testing.T) {
	for _, total := range []*string{nil, quotationsStr("0"), quotationsStr("-1"), quotationsStr("1.001")} {
		q, _ := quotationsFixture()
		q.AmountMode = "total_only"
		q.TotalAmount = total
		if _, e := (QuotationsCommand{Version: 0, Reason: "Receive total", Quote: q}).Validate(); e == nil {
			t.Fatalf("accepted invalid overall total %v", total)
		}
	}
}
func TestQuotationsExactDecimalBounds(t *testing.T) {
	for _, s := range []string{"NaN", "Infinity", "1e2", "-0.01", "+1", " 1", "1.", "9999999999999", "0.00001"} {
		if _, e := quotationsAmount(s, 12, 4); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	if _, e := quotationsAmount("999999999999.9999", 12, 4); e != nil {
		t.Fatal(e)
	}
	q, d := quotationsFixture()
	bad := int32(13)
	q.Lines[0].WarrantyMonths = &bad
	q.Lines[0].UnitPrice = nil
	q.Lines[0].Notes = "Unavailable"
	if _, e := (QuotationsCommand{Version: 0, Reason: "Receive quotation", Quote: q}).Validate(); e == nil {
		t.Fatal("unsupported warranty accepted on partial item")
	}
	_ = d
}
