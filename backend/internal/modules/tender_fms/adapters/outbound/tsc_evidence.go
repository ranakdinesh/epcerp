package outbound

import (
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	"strings"
)

func tscDocumentType(filename string, blocks []reading.EvidenceBlock) string {
	text := strings.ToLower(filename)
	for i := 0; i < len(blocks) && i < 3; i++ {
		text += " " + strings.ToLower(blocks[i].SearchText)
	}
	for _, item := range []struct {
		kind  string
		terms []string
	}{
		{"corrigendum", []string{"corrigendum", "addendum", "amendment"}},
		{"notice", []string{"notice inviting", "invitation for bid", "nit.", "bid data sheet"}},
		{"special_conditions", []string{"special conditions", "scc."}},
		{"general_conditions", []string{"general conditions", "gcc."}},
		{"boq", []string{"bill of quantities", "boq."}},
		{"technical", []string{"technical specification"}},
	} {
		for _, term := range item.terms {
			if strings.Contains(text, term) {
				return item.kind
			}
		}
	}
	return "unclassified"
}
