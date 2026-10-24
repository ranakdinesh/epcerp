package domain

import "testing"

func TestLOIExactComponentsAndDirectTotal(t *testing.T) {
	d := LOIContent{Total: "147.10", Items: []LOIItem{{Total: "145.69", Price: QuotationApprovalsCommercialPrice{Mode: "line_total", Base: "123.46", Discount: "0.00", Tax: "22.22", Charges: "0.01"}}, {Total: "1.41", Price: QuotationApprovalsCommercialPrice{Mode: "unit_price", Base: "1.20", Discount: "0.01", Tax: "0.21", Charges: "0.01"}}}}
	s, e := LOITotals(d)
	if e != nil || s["total"] != "147.10" || s["tax"] != "22.43" || s["charges"] != "0.02" {
		t.Fatal(s, e)
	}
	d.Items[0].Price.Tax = "22.23"
	if _, e = LOITotals(d); e == nil {
		t.Fatal("inconsistent approved components accepted")
	}
	d.Items[0].Price.Tax = "22.22"
	d.Total = "147.11"
	if _, e = LOITotals(d); e == nil {
		t.Fatal("inconsistent group total accepted")
	}
}
func TestLOIDocumentInputs(t *testing.T) {
	good := LOICommand{Reference: "LOI/26-27/VTPL/01", Date: "2026-10-19", Format: "cable"}
	if _, e := good.Validate("2026-10-19"); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*LOICommand){func(q *LOICommand) { q.Date = "2026-10-20" }, func(q *LOICommand) { q.Reference = " " }, func(q *LOICommand) { q.Date = "2026-02-30" }, func(q *LOICommand) { q.Format = "unknown" }} {
		q := good
		change(&q)
		if _, e := q.Validate("2026-10-19"); e == nil {
			t.Fatal("invalid LOI input accepted", q)
		}
	}
}

func TestLOIReviewRequiresEveryConfirmation(t *testing.T) {
	for _, q := range []LOIReviewCommand{{}, {DetailsCorrect: true, PricesCorrect: true}, {DetailsCorrect: true, FormatCorrect: true}, {PricesCorrect: true, FormatCorrect: true}} {
		if q.Validate() == nil {
			t.Fatal("incomplete review accepted", q)
		}
	}
	if (LOIReviewCommand{DetailsCorrect: true, PricesCorrect: true, FormatCorrect: true}).Validate() != nil {
		t.Fatal("complete review rejected")
	}
}
