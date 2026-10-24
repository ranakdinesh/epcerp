package repository

import (
	"bytes"
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"os"
	"strings"
	"testing"
)

func TestLOIPortraitPaginationAndPricingModes(t *testing.T) {
	d := domain.LOIContent{LOICommand: domain.LOICommand{Reference: "LOI/26-27/VTPL/PREVIEW/01", Date: "2026-10-19", Format: "cable", Notes: "Design preview - sample data. Inspection requirements are subject to the approved project documents."}, Vendor: "SAMPLE VENDOR - FORMAT PREVIEW", Project: "Sample illumination project - format preview", ProjectNumber: "PREVIEW", Client: "Sample client", Address: "Sample address for layout verification", ContactPerson: "Sample contact", Currency: "INR", Quote: "SAMPLE-QUOTE", Group: "Sample cable group", Total: "145.69", Items: []domain.LOIItem{{Description: "Supply and underground laying of copper armoured cable from the main lighting panel to each local control panel, only after client acceptance. · Size: 25 sq. mm\nInclude termination accessories and testing as specified in BOQ item 08.", SKU: "SAMPLE-CABLE", Make: "Approved make", UOM: "m", Quantity: "10", Price: domain.QuotationApprovalsCommercialPrice{Mode: "line_total", Base: "123.46", Discount: "0.00", Tax: "22.22", Charges: "0.01"}, Total: "145.69", Payment: "As per approved quotation", Warranty: "18 months", DeliveryDays: 21, Technical: "Voltage: 1.1 kV\nConductor: Copper\nCores: 4"}}}
	data, e := (LOIPDFPrinter{}).Render(t.Context(), d, 1)
	if e != nil || !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatal("LOI PDF", e)
	}
	if path := os.Getenv("EPCERP_LOI_PREVIEW_OUTPUT"); path != "" {
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	d.Items[0].Description = strings.Repeat("Long vendor technical description with engineering specifications. ", 100)
	if _, e = (LOIPDFPrinter{}).Render(t.Context(), d, 2); e != nil {
		t.Fatal("long table row pagination", e)
	}
	c, cancel := context.WithCancel(t.Context())
	cancel()
	if _, e = (LOIPDFPrinter{}).Render(c, d, 1); e == nil {
		t.Fatal("cancelled render continued")
	}
	if loiWords(2397465) != "Twenty Three Lakh Ninety Seven Thousand Four Hundred Sixty Five" {
		t.Fatal("Indian amount words")
	}
}

func TestLOIItemScopeSurvivesStructuredSpecifications(t *testing.T) {
	item := domain.LOIItem{
		Description: "Supply, installation, testing and commissioning of an outdoor pillar box. · Enclosure: SS316, 2 mm\nInclude foundation, anchor bolts and cable terminations; BOQ item 5.",
		Technical:   "Enclosure: SS316, 2 mm\nIncomer: 400 A, 3P+N\nOutgoing: 250 A x 3; 125 A x 6",
		Make:        "Approved panel make",
	}
	want := []string{
		"Supply, installation, testing and commissioning of an outdoor pillar box.",
		"Enclosure: SS316, 2 mm",
		"Include foundation, anchor bolts and cable terminations; BOQ item 5.",
		"Incomer: 400 A, 3P+N",
		"Outgoing: 250 A x 3; 125 A x 6",
		"Make: Approved panel make",
	}
	got := loiItemDetails(item)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("scope/specifications lost or duplicated: %q", got)
	}
	item.Technical = ""
	if !strings.Contains(loiItemDescription(item), want[2]) {
		t.Fatal("free-text scope disappeared without structured specifications")
	}
}
