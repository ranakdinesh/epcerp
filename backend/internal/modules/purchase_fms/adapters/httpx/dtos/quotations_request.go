package dtos

import (
	"encoding/json"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationsLineRequest struct {
	LineID         string             `json:"line_id"`
	UnitPrice      *QuotationsDecimal `json:"unit_price"`
	PricingMode    string             `json:"pricing_mode"`
	LineTotal      *QuotationsDecimal `json:"line_total"`
	CoverageType   string             `json:"coverage_type"`
	WarrantyMonths *int32             `json:"warranty_months"`
	Discount       QuotationsDecimal  `json:"discount_pct"`
	Tax            QuotationsDecimal  `json:"tax_pct"`
	Compliant      bool               `json:"compliant"`
	Notes          string             `json:"notes"`
}
type QuotationsQuoteRequest struct {
	DocumentID          string                  `json:"document_id"`
	AmountMode          string                  `json:"amount_mode"`
	TotalAmount         *QuotationsDecimal      `json:"total_amount"`
	Reference           string                  `json:"reference"`
	AttachmentID        *string                 `json:"attachment_id"`
	EvidenceReference   string                  `json:"evidence_reference"`
	DocumentNotProvided bool                    `json:"document_not_provided"`
	NoDocumentReason    string                  `json:"no_document_reason"`
	ReceivedDate        string                  `json:"received_date"`
	ValidUntil          string                  `json:"valid_until"`
	Currency            string                  `json:"currency"`
	Freight             QuotationsDecimal       `json:"freight"`
	OtherCharges        QuotationsDecimal       `json:"other_charges"`
	DeliveryDays        *int32                  `json:"delivery_days"`
	DeliveryTerms       string                  `json:"delivery_terms"`
	PaymentTerms        string                  `json:"payment_terms"`
	Lines               []QuotationsLineRequest `json:"lines"`
}
type QuotationsCommandRequest struct {
	Version *int32                 `json:"version"`
	Reason  string                 `json:"reason"`
	Quote   QuotationsQuoteRequest `json:"quote"`
}

func QuotationsDecimalText(v QuotationsDecimal) string {
	if v == "" {
		return "0"
	}
	return string(v)
}
func QuotationsDecimalPointer(v *QuotationsDecimal) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}
func (r QuotationsCommandRequest) Command() (domain.QuotationsCommand, error) {
	if r.Version == nil || r.Quote.DeliveryDays == nil {
		return domain.QuotationsCommand{}, domain.QuotationsInvalid("", "Version and delivery days are required")
	}
	q := r.Quote
	out := domain.QuotationsCommand{Version: *r.Version, Reason: r.Reason, Quote: domain.QuotationsQuoteInput{DocumentID: q.DocumentID, AmountMode: q.AmountMode, TotalAmount: QuotationsDecimalPointer(q.TotalAmount), Reference: q.Reference, AttachmentID: q.AttachmentID, EvidenceReference: q.EvidenceReference, DocumentNotProvided: q.DocumentNotProvided, NoDocumentReason: q.NoDocumentReason, ReceivedDate: q.ReceivedDate, ValidUntil: q.ValidUntil, Currency: q.Currency, Freight: QuotationsDecimalText(q.Freight), OtherCharges: QuotationsDecimalText(q.OtherCharges), DeliveryDays: *q.DeliveryDays, DeliveryTerms: q.DeliveryTerms, PaymentTerms: q.PaymentTerms, Lines: []domain.QuotationsLineInput{}}}
	for _, l := range q.Lines {
		out.Quote.Lines = append(out.Quote.Lines, domain.QuotationsLineInput{LineID: l.LineID, UnitPrice: QuotationsDecimalPointer(l.UnitPrice), PricingMode: l.PricingMode, LineTotal: QuotationsDecimalPointer(l.LineTotal), CoverageType: l.CoverageType, WarrantyMonths: l.WarrantyMonths, Discount: QuotationsDecimalText(l.Discount), Tax: QuotationsDecimalText(l.Tax), Compliant: l.Compliant, Notes: l.Notes})
	}
	return out, nil
}

// Null is allowed only for explicitly nullable fields; omitted defaults remain
// distinct from an explicit null on percentages, boolean flags and arrays.
func QuotationsNonNull(raw []byte) error {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return domain.QuotationsInvalid("", "Expected a JSON object")
	}
	for k, v := range obj {
		if string(v) == "null" && k != "attachment_id" && k != "total_amount" && k != "unit_price" && k != "line_total" && k != "warranty_months" {
			return domain.QuotationsInvalid(k, "This field must not be null")
		}
		if k == "quote" {
			if e := QuotationsNonNull(v); e != nil {
				return e
			}
		}
		if k == "lines" {
			var rows []json.RawMessage
			if json.Unmarshal(v, &rows) != nil {
				return domain.QuotationsInvalid(k, "Expected quotation item rows")
			}
			for _, row := range rows {
				if e := QuotationsNonNull(row); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
