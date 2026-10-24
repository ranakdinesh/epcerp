package domain

import (
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var sourcingCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

var sourcingIdPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func SourcingID(s string) error {
	if !sourcingIdPattern.MatchString(s) {
		return SourcingInvalid("id", "Use a valid identifier")
	}
	return nil
}
func SourcingText(f, s string, n int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > n || (required && s == "") {
		return s, SourcingInvalid(f, "Provide a valid "+f)
	}
	return s, nil
}
func SourcingDate(f, s string) (string, error) {
	d, e := time.Parse("2006-01-02", s)
	if e != nil || d.Format("2006-01-02") != s {
		return s, SourcingInvalid(f, "Use a calendar date")
	}
	return s, nil
}
func SourcingToday(now time.Time) string {
	return now.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}

var sourcingAmountPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$`)
var sourcingQuantityPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,4})?$`)

func sourcingScaled(f, s string, scale, max int, positive bool) (*big.Int, error) {
	pat := sourcingAmountPattern
	if scale == 4 {
		pat = sourcingQuantityPattern
	}
	if !pat.MatchString(s) {
		return nil, SourcingInvalid(f, "Use a nonnegative plain decimal with the permitted precision")
	}
	parts := strings.Split(s, ".")
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	digits := parts[0] + fraction + strings.Repeat("0", scale-len(fraction))
	v, ok := new(big.Int).SetString(digits, 10)
	if !ok || len(v.String()) > max || (positive && v.Sign() == 0) {
		return nil, SourcingInvalid(f, "Amount or quantity exceeds the permitted range")
	}
	return v, nil
}
func SourcingAmount(f, s string) (string, error) {
	v, e := sourcingScaled(f, s, 2, 16, false)
	if e != nil {
		return "", e
	}
	return sourcingFormat(v), nil
}

// Rate retains Go quotation precision; monetary charges and totals remain two decimals.
func SourcingRate(s string) (string, error) {
	v, e := sourcingScaled("unit_price", s, 4, 18, false)
	if e != nil {
		return "", e
	}
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(v, big.NewInt(10000), r)
	return fmt.Sprintf("%s.%04d", q.String(), r.Int64()), nil
}
func sourcingFormat(v *big.Int) string {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(v, big.NewInt(100), r)
	return fmt.Sprintf("%s.%02d", q.String(), r.Int64())
}
func SourcingTotal(o SourcingOffer, quantity string) (string, error) {
	q, e := sourcingScaled("quantity", quantity, 4, 19, true)
	if e != nil {
		return "", e
	}
	mode, e := sourcingPricingMode(o.PricingMode)
	if e != nil {
		return "", e
	}
	var base *big.Int
	if mode == "line_total" {
		if o.UnitPrice != "" {
			return "", SourcingInvalid("unit_price", "Use only the direct line total")
		}
		base, e = sourcingScaled("line_total", o.LineTotal, 2, 16, false)
	} else {
		if o.LineTotal != "" {
			return "", SourcingInvalid("line_total", "Use only the unit rate")
		}
		var rate *big.Int
		rate, e = sourcingScaled("unit_price", o.UnitPrice, 4, 18, false)
		if e == nil {
			base = new(big.Int).Mul(q, rate)
			base.Add(base, big.NewInt(500000))
			base.Quo(base, big.NewInt(1000000))
		}
	}
	if e != nil {
		return "", e
	}
	a := make([]*big.Int, 5)
	for i, v := range []struct{ field, value string }{{"discount", o.Discount}, {"tax", o.Tax}, {"freight", o.Freight}, {"other", o.Other}} {
		a[i+1], e = sourcingScaled(v.field, v.value, 2, 16, false)
		if e != nil {
			return "", e
		}
	}
	if a[1].Cmp(base) > 0 {
		return "", SourcingInvalid("discount", "Discount exceeds the line value")
	}
	base.Sub(base, a[1])
	for _, v := range a[2:] {
		base.Add(base, v)
	}
	if len(base.String()) > 16 {
		return "", SourcingInvalid("total", "Quotation total exceeds the permitted range")
	}
	return sourcingFormat(base), nil
}
func SourcingComparisons(c SourcingCase, today string) ([]SourcingComparison, error) {
	out := []SourcingComparison{}
	ranks := []string{}
	for _, o := range c.Offers {
		if o.State != "quoted" {
			continue
		}
		v, e := SourcingTotal(o, c.Source.Quantity)
		if e != nil {
			return nil, e
		}
		if o.Compliant {
			ranks = append(ranks, v)
		}
		out = append(out, SourcingComparison{OfferID: o.ID, Total: v, Expired: o.ValidUntil < today})
	}
	slices.SortFunc(ranks, func(a, b string) int {
		x, _ := sourcingScaled("total", a, 2, 16, false)
		y, _ := sourcingScaled("total", b, 2, 16, false)
		return x.Cmp(y)
	})
	ranks = slices.Compact(ranks)
	for i := range out {
		for _, o := range c.Offers {
			if o.ID == out[i].OfferID && o.Compliant {
				v := slices.Index(ranks, out[i].Total) + 1
				out[i].Rank = &v
			}
		}
	}
	return out, nil
}

func sourcingNewAmountSum(a, b *big.Int) string { return sourcingFormat(new(big.Int).Add(a, b)) }

func sourcingPricingMode(mode string) (string, error) {
	if mode == "" {
		return "unit_price", nil
	}
	if mode != "unit_price" && mode != "line_total" {
		return "", SourcingInvalid("pricing_mode", "Choose unit rate or direct line total")
	}
	return mode, nil
}
