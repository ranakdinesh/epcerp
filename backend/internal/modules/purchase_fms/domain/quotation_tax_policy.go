package domain

// Central business policy for new purchase quotation revisions. Historical
// revision tax snapshots are retained and never recalculated under a later policy.
const QuotationTaxRate = "18.00"

func QuotationsNormalizeMoney(q QuotationsQuoteInput) (QuotationsQuoteInput, error) {
	fixed := func(raw string, digits int) (string, error) {
		v, e := quotationsAmount(raw, digits, 2)
		if e != nil {
			return "", e
		}
		return quotationsFixed(quotationsRounded(v, 2), 2), nil
	}
	q.Lines = append([]QuotationsLineInput{}, q.Lines...)
	for i := range q.Lines {
		l := &q.Lines[i]
		l.Tax = QuotationTaxRate
		if l.UnitPrice != nil {
			v, e := fixed(*l.UnitPrice, 12)
			if e != nil {
				return q, e
			}
			l.UnitPrice = quotationsStr(v)
		}
		if l.LineTotal != nil {
			v, e := fixed(*l.LineTotal, 18)
			if e != nil {
				return q, e
			}
			l.LineTotal = quotationsStr(v)
		}
		v, e := fixed(l.Discount, 3)
		if e != nil {
			return q, e
		}
		l.Discount = v
	}
	for _, p := range []*string{&q.Freight, &q.OtherCharges} {
		v, e := fixed(*p, 18)
		if e != nil {
			return q, e
		}
		*p = v
	}
	return q, nil
}
