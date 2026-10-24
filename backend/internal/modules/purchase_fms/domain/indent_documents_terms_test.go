package domain

import (
	"strings"
	"testing"
)

func TestIndentTermsAreVendorSpecificAndIndependent(t *testing.T) {
	data := map[string]any{"indent_notes": " First note\r\n\r\nSecond note ", "indent_terms": "Vendor A conditions", "delivery_terms": "Site A"}
	a, e := IndentDocumentsVendorTerms(IndentDocumentsVendor{Data: data})
	if e != nil || strings.Join(a.Notes, "|") != "First note|Second note" {
		t.Fatal(a, e)
	}
	data["indent_terms"] = "Changed master"
	b, e := IndentDocumentsVendorTerms(IndentDocumentsVendor{Data: map[string]any{"indent_terms": "Vendor B conditions", "delivery_terms": "Site B"}})
	if e != nil || a.Conditions[0] != "Vendor A conditions" || b.Conditions[0] != "Vendor B conditions" || a.DeliveryBasis == b.DeliveryBasis {
		t.Fatal("vendor terms mixed or aliased", e)
	}
	doc := IndentDocumentsDocument{Data: IndentDocumentsData{Terms: a}}
	copy := IndentDocumentsCopy(doc)
	copy.Data.Terms.Conditions[0] = "Changed copy"
	if a.Conditions[0] != "Vendor A conditions" {
		t.Fatal("document copy aliased frozen terms")
	}
}
func TestIndentTermsRequireConfiguredConditionsAndDelivery(t *testing.T) {
	for _, data := range []map[string]any{nil, {}, {"indent_terms": " ", "delivery_terms": "Site"}, {"indent_terms": "Terms"}, {"indent_terms": true, "delivery_terms": "Site"}, {"indent_terms": strings.Repeat("x", 12001), "delivery_terms": "Site"}} {
		if _, e := IndentDocumentsVendorTerms(IndentDocumentsVendor{Data: data}); e == nil {
			t.Fatal("unconfigured/invalid vendor terms accepted")
		}
	}
}
