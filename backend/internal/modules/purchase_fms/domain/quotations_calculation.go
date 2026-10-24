package domain

import (
	"math/big"
	"regexp"
	"sort"
	"strings"
)

var quotationsDecimal = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
var quotationsHundred = big.NewRat(100, 1)

func quotationsAmount(raw string, integers, places int) (*big.Rat, error) {
	if !quotationsDecimal.MatchString(raw) || len(raw) > integers+places+2 {
		return nil, QuotationsInvalid("amount", "Prices and charges must be finite nonnegative decimal amounts")
	}
	parts := strings.Split(raw, ".")
	if len(strings.TrimLeft(parts[0], "0")) > integers || (len(parts) > 1 && len(parts[1]) > places) {
		return nil, QuotationsInvalid("amount", "Amount exceeds the supported digits or decimal places")
	}
	v, ok := new(big.Rat).SetString(raw)
	if !ok {
		return nil, QuotationsInvalid("amount", "Invalid decimal amount")
	}
	return v, nil
}
func quotationsRounded(v *big.Rat, places int) *big.Int {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	n := new(big.Int).Mul(v.Num(), scale)
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, v.Denom(), r)
	if new(big.Int).Mul(r, big.NewInt(2)).Cmp(v.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}
func quotationsFixed(v *big.Int, places int) string {
	s := v.String()
	for len(s) <= places {
		s = "0" + s
	}
	if places == 0 {
		return s
	}
	return s[:len(s)-places] + "." + s[len(s)-places:]
}
func quotationsCents(v *big.Int) *big.Rat { return new(big.Rat).SetFrac(v, big.NewInt(100)) }
func quotationsStr(v string) *string      { return &v }
func quotationsWarranty(v *int32) bool {
	if v == nil {
		return false
	}
	switch *v {
	case 6, 12, 18, 24, 36, 48, 60, 84:
		return true
	}
	return false
}
func QuotationsCalculate(q QuotationsQuoteInput, d QuotationsDocument, today string) (QuotationsCalculation, error) {
	out := QuotationsCalculation{Lines: []QuotationsCalcLine{}, Compliant: true}
	if d.Data.SentDate == nil || !QuotationsDate(*d.Data.SentDate) || q.ReceivedDate < *d.Data.SentDate || q.ReceivedDate > today {
		return out, QuotationsInvalid("received_date", "Received date must be between indent dispatch and today")
	}
	if q.ValidUntil < q.ReceivedDate {
		return out, QuotationsInvalid("valid_until", "Quote validity cannot precede receipt")
	}
	mapped := map[string]QuotationsLineInput{}
	for _, l := range q.Lines {
		if _, ok := mapped[l.LineID]; ok {
			return out, QuotationsInvalid("lines", "Record every vendor-indent item exactly once")
		}
		mapped[l.LineID] = l
	}
	if len(mapped) != len(d.Data.Lines) {
		return out, QuotationsInvalid("lines", "Record every vendor-indent item exactly once")
	}
	if q.AmountMode == "total_only" {
		if q.TotalAmount == nil {
			return out, QuotationsInvalid("total_amount", "Enter the vendor's overall quotation total")
		}
		amount, e := quotationsAmount(*q.TotalAmount, 20, 2)
		if e != nil || amount.Sign() <= 0 {
			return out, QuotationsInvalid("total_amount", "Overall quotation total must be a positive amount")
		}
		for _, source := range d.Data.Lines {
			line, ok := mapped[source.ID]
			if !ok {
				return out, QuotationsInvalid("lines", "Record every vendor-indent item exactly once")
			}
			if source.Quantity == nil {
				return out, QuotationsInvalid("quantity", "Indent quantity is missing")
			}
			quantity, quantityError := quotationsAmount(*source.Quantity, 13, 3)
			if quantityError != nil || quantity.Sign() <= 0 {
				return out, QuotationsInvalid("quantity", "Indent quantities must be positive")
			}
			out.Lines = append(out.Lines, QuotationsCalcLine{QuotationsLineInput: line, Quantity: *source.Quantity, Source: source})
		}
		out.Total = quotationsFixed(quotationsRounded(amount, 2), 2)
		out.Complete = true
		out.Compliant = false
		return out, nil
	}
	priced := []int{}
	weights := []*big.Int{}
	sumWeights := new(big.Int)
	total := new(big.Int)
	for _, source := range d.Data.Lines {
		l, ok := mapped[source.ID]
		if !ok {
			return out, QuotationsInvalid("lines", "Record every vendor-indent item exactly once")
		}
		if source.Quantity == nil {
			return out, QuotationsInvalid("quantity", "Indent quantity is missing")
		}
		quantity, e := quotationsAmount(*source.Quantity, 13, 3)
		if e != nil || quantity.Sign() <= 0 {
			return out, QuotationsInvalid("quantity", "Indent quantities must be positive")
		}
		row := QuotationsCalcLine{QuotationsLineInput: l, UnitPrice: l.UnitPrice, Quantity: *source.Quantity, Source: source}
		out.Compliant = out.Compliant && l.Compliant
		mode := l.PricingMode
		if mode == "" {
			mode = "unit_price"
		}
		if mode != "unit_price" && mode != "line_total" {
			return out, QuotationsInvalid("pricing_mode", "Choose unit price or line total")
		}
		price := l.UnitPrice
		if mode == "line_total" {
			if l.UnitPrice != nil {
				return out, QuotationsInvalid("unit_price", "Enter only line total when using direct amount")
			}
			price = l.LineTotal
		} else if l.LineTotal != nil {
			return out, QuotationsInvalid("line_total", "Enter only unit price when using unit rate")
		}
		if price == nil {
			if strings.TrimSpace(l.Notes) == "" {
				return out, QuotationsInvalid("notes", "Record why an item is not quoted")
			}
			out.Lines = append(out.Lines, row)
			continue
		}
		if !quotationsWarranty(l.WarrantyMonths) {
			return out, QuotationsInvalid("warranty_months", "Select warranty/guarantee months for every quoted SKU")
		}
		if l.CoverageType != "warranty" && l.CoverageType != "guarantee" && l.CoverageType != "both" {
			return out, QuotationsInvalid("coverage_type", "Choose warranty, guarantee or both")
		}
		integer, scale := 12, 4
		if mode == "line_total" {
			integer, scale = 18, 2
		}
		rate, e := quotationsAmount(*price, integer, scale)
		if e != nil {
			return out, e
		}
		base := quotationsRounded(new(big.Rat).Mul(quantity, rate), 2)
		if mode == "line_total" {
			base = quotationsRounded(rate, 2)
			row.UnitPrice = quotationsStr(quotationsFixed(quotationsRounded(new(big.Rat).Quo(quotationsCents(base), quantity), 2), 2))
		}
		discount, e := quotationsAmount(l.Discount, 3, 2)
		if e != nil || discount.Cmp(quotationsHundred) > 0 {
			return out, QuotationsInvalid("discount_pct", "Discount must be from zero to 100")
		}
		tax, e := quotationsAmount(l.Tax, 3, 2)
		if e != nil || tax.Cmp(quotationsHundred) > 0 {
			return out, QuotationsInvalid("tax_pct", "Tax must be from zero to 100")
		}
		reduction := quotationsRounded(new(big.Rat).Quo(new(big.Rat).Mul(quotationsCents(base), discount), quotationsHundred), 2)
		net := new(big.Int).Sub(base, reduction)
		taxAmount := quotationsRounded(new(big.Rat).Quo(new(big.Rat).Mul(quotationsCents(net), tax), quotationsHundred), 2)
		landed := new(big.Int).Add(net, taxAmount)
		row.Base = quotationsStr(quotationsFixed(base, 2))
		row.DiscountAmount = quotationsStr(quotationsFixed(reduction, 2))
		row.Net = quotationsStr(quotationsFixed(net, 2))
		row.TaxAmount = quotationsStr(quotationsFixed(taxAmount, 2))
		row.Landed = quotationsStr(quotationsFixed(landed, 2))
		priced = append(priced, len(out.Lines))
		weights = append(weights, net)
		sumWeights.Add(sumWeights, net)
		total.Add(total, landed)
		out.Lines = append(out.Lines, row)
	}
	if len(priced) == 0 {
		return out, QuotationsInvalid("lines", "Record a price for at least one item")
	}
	freight, e := quotationsAmount(q.Freight, 14, 2)
	if e != nil {
		return out, e
	}
	other, e := quotationsAmount(q.OtherCharges, 14, 2)
	if e != nil {
		return out, e
	}
	extras := quotationsRounded(new(big.Rat).Add(freight, other), 2)
	if sumWeights.Sign() == 0 {
		sumWeights.SetInt64(int64(len(weights)))
		for i := range weights {
			weights[i] = big.NewInt(1)
		}
	}
	shares := []*big.Int{}
	remainders := []*big.Int{}
	allocated := new(big.Int)
	order := []int{}
	for i, w := range weights {
		n := new(big.Int).Mul(extras, w)
		share, remainder := new(big.Int), new(big.Int)
		share.QuoRem(n, sumWeights, remainder)
		shares = append(shares, share)
		remainders = append(remainders, remainder)
		allocated.Add(allocated, share)
		order = append(order, i)
	}
	sort.SliceStable(order, func(i, j int) bool { return remainders[order[i]].Cmp(remainders[order[j]]) > 0 })
	left := new(big.Int).Sub(extras, allocated).Int64()
	for i := int64(0); i < left; i++ {
		shares[order[i]].Add(shares[order[i]], big.NewInt(1))
	}
	for i, index := range priced {
		row := &out.Lines[index]
		row.AllocatedCharges = quotationsStr(quotationsFixed(shares[i], 2))
		landed, _ := new(big.Rat).SetString(*row.Landed)
		row.Landed = quotationsStr(quotationsFixed(new(big.Int).Add(quotationsRounded(landed, 2), shares[i]), 2))
	}
	total.Add(total, extras)
	maximum, _ := new(big.Int).SetString("9999999999999999999999", 10)
	if total.Cmp(maximum) > 0 {
		return out, QuotationsInvalid("total", "Quotation total exceeds supported limit")
	}
	out.Total = quotationsFixed(total, 2)
	out.Complete = len(priced) == len(out.Lines)
	return out, nil
}
func QuotationsCompare(quotes []QuotationsQuote, documents []QuotationsDocument, today, currency string) []QuotationsComparison {
	out := []QuotationsComparison{}
	for _, q := range quotes {
		var d *QuotationsDocument
		for i := range documents {
			if documents[i].ID == q.DocumentID {
				d = &documents[i]
				break
			}
		}
		if d == nil {
			continue
		}
		reasons := []string{}
		expired := q.ValidUntil < today
		if !d.Data.IsCurrent {
			reasons = append(reasons, "Superseded indent scope — revised quotation required")
		}
		if q.Currency != currency {
			reasons = append(reasons, "Quotation currency differs from the project")
		}
		if q.AmountMode == "total_only" {
			reasons = append(reasons, "Overall quotation total has no item allocation — record item prices before Accounts approval")
		}
		if !q.Calculation.Complete {
			reasons = append(reasons, "Partial quotation")
		}
		if !q.Calculation.Compliant {
			reasons = append(reasons, "Technical compliance not confirmed for every item")
		}
		if !d.VendorAvailable {
			reasons = append(reasons, "Vendor inactive or unavailable")
		}
		if expired {
			reasons = append(reasons, "Quotation expired")
		}
		out = append(out, QuotationsComparison{DocumentID: q.DocumentID, AmountMode: q.AmountMode, TotalAmount: q.TotalAmount, GroupID: d.GroupID, GroupName: d.Data.GroupName, ScopeRevision: d.Data.ScopeRevision, VendorID: d.VendorID, Vendor: d.Data.Vendor.Name, Reference: q.Reference, Total: q.Calculation.Total, Eligible: len(reasons) == 0, Exclusions: reasons, Expired: expired, DeliveryDays: q.DeliveryDays, PaymentTerms: q.PaymentTerms, DeliveryTerms: q.DeliveryTerms, Lines: append([]QuotationsCalcLine{}, q.Calculation.Lines...), Revision: q.Revision, ReceivedDate: q.ReceivedDate, ValidUntil: q.ValidUntil, RecordedAt: q.RecordedAt, Reason: q.Reason})
	}
	for i := range out {
		r := &out[i]
		if !r.Eligible {
			continue
		}
		lower := map[string]bool{}
		own, _ := new(big.Rat).SetString(r.Total)
		for _, other := range out {
			if !other.Eligible || other.GroupID != r.GroupID {
				continue
			}
			v, _ := new(big.Rat).SetString(other.Total)
			if v.Cmp(own) < 0 {
				lower[other.Total] = true
			}
		}
		rank := int32(len(lower) + 1)
		r.Rank = &rank
		r.Suggested = rank == 1
		for j := range r.Lines {
			line := &r.Lines[j]
			if line.Landed == nil {
				continue
			}
			price, _ := new(big.Rat).SetString(*line.Landed)
			lowest := true
			for _, other := range out {
				if !other.Eligible || other.GroupID != r.GroupID {
					continue
				}
				for _, candidate := range other.Lines {
					if candidate.LineID == line.LineID && candidate.Landed != nil {
						v, _ := new(big.Rat).SetString(*candidate.Landed)
						if v.Cmp(price) < 0 {
							lowest = false
						}
					}
				}
			}
			line.ItemLowest = lowest
		}
	}
	return out
}

// New quotation choices; historic revisions retain their previously valid durations.
func quotationsWarrantyInput(v *int32) bool {
	if v == nil {
		return false
	}
	switch *v {
	case 6, 12, 24, 48:
		return true
	}
	return false
}
