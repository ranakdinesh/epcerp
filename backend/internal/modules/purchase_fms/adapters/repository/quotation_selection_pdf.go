package repository

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure/pdfreport"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func quotationSelectionFill(n domain.QuotationApprovalsNomination) pdfreport.Fill {
	colors := []pdfreport.Color{}
	if n.BestPrice {
		colors = append(colors, pdfreport.Color{R: 237, G: 248, B: 239})
	}
	if n.Preferred {
		colors = append(colors, pdfreport.Color{R: 255, G: 250, B: 224})
	}
	return pdfreport.Fill{Colors: colors}
}
func quotationSelectionLegend() []pdfreport.LegendEntry {
	return []pdfreport.LegendEntry{
		{Label: "Lowest price", Fill: quotationSelectionFill(domain.QuotationApprovalsNomination{BestPrice: true})},
		{Label: "Preferred", Fill: quotationSelectionFill(domain.QuotationApprovalsNomination{Preferred: true})},
		{Label: "Both selections", Fill: quotationSelectionFill(domain.QuotationApprovalsNomination{BestPrice: true, Preferred: true})},
	}
}
