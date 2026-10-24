package domain

import "testing"

func TestQuotationMoneyPolicyPreservesLumpSumAndCanonicalSnapshots(t *testing.T) {
	q, d := quotationsFixture()
	q.Lines[0].UnitPrice = nil
	q.Lines[0].PricingMode = "line_total"
	q.Lines[0].LineTotal = quotationsStr("100")
	q.Lines[0].Discount = "0"
	q.Lines[0].Tax = "0"
	n, e := QuotationsNormalizeMoney(q)
	if e != nil {
		t.Fatal(e)
	}
	c, e := QuotationsCalculate(n, d, "2026-10-10")
	if e != nil {
		t.Fatal(e)
	}
	if *n.Lines[0].LineTotal != "100.00" || n.Lines[0].Tax != "18.00" || *c.Lines[0].UnitPrice != "33.33" || *c.Lines[0].Base != "100.00" || c.Total != "118.00" {
		t.Fatal(n, c)
	}
	if q.Lines[0].Tax != "0" || *q.Lines[0].LineTotal != "100" {
		t.Fatal("mutated submitted input")
	}
	q.Lines[0].LineTotal = quotationsStr("1.234")
	if _, e := QuotationsNormalizeMoney(q); e == nil {
		t.Fatal("accepted excessive precision")
	}
}

func TestQuotationNewCoverageChoicesAndHistoricalCalculation(t *testing.T) {
	q, d := quotationsFixture()
	q.Lines[0].UnitPrice = quotationsStr("1.00")
	q.DocumentNotProvided = true
	q.NoDocumentReason = "Vendor telephone quotation"
	for _, months := range []int32{6, 12, 24, 48} {
		q.Lines[0].WarrantyMonths = &months
		c := QuotationsCommand{Reason: "Receive quote", Quote: q}
		if _, e := c.Validate(); e != nil {
			t.Fatal(months, e)
		}
		if _, e := QuotationsCalculate(q, d, "2026-10-10"); e != nil {
			t.Fatal(months, e)
		}
	}
	for _, months := range []int32{18, 36, 60, 84} {
		q.Lines[0].WarrantyMonths = &months
		if _, e := (QuotationsCommand{Reason: "Receive quote", Quote: q}).Validate(); e == nil {
			t.Fatal("old duration accepted for new quote", months)
		}
		if _, e := QuotationsCalculate(q, d, "2026-10-10"); e != nil {
			t.Fatal("historic calculation rejected", months, e)
		}
	}
}
