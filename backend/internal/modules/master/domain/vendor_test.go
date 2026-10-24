package domain

import (
	"strings"
	"testing"
)

func TestVendorPaymentTermsAndUnmappedReferences(t *testing.T) {
	if VendorValidateTerms(map[string]any{"advance_pct": 33.33, "qc_pct": 33.33, "delivery_pct": 33.34, "credit_days": 30}) != nil {
		t.Fatal("valid exact terms rejected")
	}
	for _, values := range []map[string]any{{"advance_pct": 10}, {"advance_pct": -1}, {"advance_pct": "100"}, {"credit_days": 1.5}, {"advance_pct": 101}, {"extra": 0}} {
		if VendorValidateTerms(values) == nil {
			t.Fatal("invalid terms accepted")
		}
	}
}

func TestVendorIndentTextFieldsAreBoundedAndOptional(t *testing.T) {
	if e := VendorValidateMaster("V1", "Vendor", map[string]any{}); e != nil {
		t.Fatal(e)
	}
	for key, maximum := range map[string]int{"indent_notes": 4000, "indent_terms": 12000, "delivery_terms": 2000} {
		if e := VendorValidateMaster("V1", "Vendor", map[string]any{key: strings.Repeat("अ", maximum)}); e != nil {
			t.Fatal(key, e)
		}
		for _, value := range []any{true, []string{"clause"}, strings.Repeat("अ", maximum+1)} {
			e := VendorValidateMaster("V1", "Vendor", map[string]any{key: value})
			if e == nil || e.(*Error).Field != key {
				t.Fatal("invalid terms not associated with field", key, e)
			}
		}
	}
}
