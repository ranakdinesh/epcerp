package domain

import (
	"strings"
	"unicode/utf8"
)

type IndentDocumentsTerms struct {
	Source        string   `json:"source"`
	Notes         []string `json:"notes"`
	Conditions    []string `json:"conditions"`
	DeliveryBasis string   `json:"delivery_basis"`
}

// Current vendor defaults become immutable document evidence only at generation.
func IndentDocumentsVendorTerms(v IndentDocumentsVendor) (*IndentDocumentsTerms, error) {
	values := map[string]string{}
	for key, maximum := range map[string]int{"indent_notes": 4000, "indent_terms": 12000, "delivery_terms": 2000} {
		raw, exists := v.Data[key]
		if !exists {
			continue
		}
		value, ok := raw.(string)
		if !ok || utf8.RuneCountInString(value) > maximum {
			return nil, IndentDocumentsInvalid("vendor_id", "Vendor indent terms are invalid; correct them in Vendors / Suppliers")
		}
		values[key] = strings.TrimSpace(value)
	}
	if values["indent_terms"] == "" || values["delivery_terms"] == "" {
		return nil, IndentDocumentsInvalid("vendor_id", "Configure this vendor's purchase indent conditions and delivery basis in Vendors / Suppliers before generating an indent")
	}
	return &IndentDocumentsTerms{Source: "vendor_master", Notes: indentTermLines(values["indent_notes"]), Conditions: indentTermLines(values["indent_terms"]), DeliveryBasis: values["delivery_terms"]}, nil
}
func indentTermLines(value string) []string {
	result := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}
