package domain

import "testing"

func TestShortlistDraftDoesNotRequireApprovalEvidence(t *testing.T) {
	g := "11111111-1111-4111-8111-111111111111"
	key := "22222222-2222-4222-8222-222222222222:1"
	d := QuotationShortlistDraft{GroupID: g, Proposals: []QuotationApprovalsNomination{{QuoteKey: key, BestPrice: true, Preferred: true, BestPriceComment: "Lowest complete cost", PreferredComment: "Meets delivery"}}}
	source := QuotationApprovalsSource{Groups: []QuotationApprovalsGroup{{ID: g}}, Quotes: []QuotationApprovalsQuote{{Key: key, GroupID: g, Eligible: true, Total: "10.00"}}}
	if _, e := QuotationShortlistDraftValidate(d, source, nil); e != nil {
		t.Fatal(e)
	}
	wrong := d
	wrong.GroupID = "33333333-3333-4333-8333-333333333333"
	if _, e := QuotationShortlistDraftValidate(wrong, source, nil); e == nil {
		t.Fatal("cross-group nomination accepted")
	}
	source.Quotes = append(source.Quotes, QuotationApprovalsQuote{Key: "44444444-4444-4444-8444-444444444444:1", GroupID: g, Eligible: true, Total: "9.00"})
	if _, e := QuotationShortlistDraftValidate(d, source, nil); e == nil {
		t.Fatal("false best-price designation accepted")
	}
}
