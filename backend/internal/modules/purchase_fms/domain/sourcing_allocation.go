package domain

import (
	"fmt"
	"math/big"
)

// Allocation is a proposed debit of immutable released quantity and approved
// commercial value. Used balances must come from the owning, locked ledger,
// never from an HTTP command. Unspent approval is retained, not fabricated cost.
type SourcingAllocation struct {
	Quantity          string
	Amount            string
	RemainingQuantity string
	RemainingAmount   string
}

func sourcingQuantityString(v *big.Int) string {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(v, big.NewInt(10000), r)
	return fmt.Sprintf("%s.%04d", q.String(), r.Int64())
}
func SourcingPlanAllocation(quantity, approvedQuantity, approvedAmount, usedQuantity, usedAmount, amount string) (SourcingAllocation, error) {
	empty := SourcingAllocation{}
	q, e := sourcingScaled("quantity", quantity, 4, 19, true)
	if e != nil {
		return empty, e
	}
	approved, e := sourcingScaled("approved_quantity", approvedQuantity, 4, 19, true)
	if e != nil {
		return empty, e
	}
	used, e := sourcingScaled("used_quantity", usedQuantity, 4, 19, false)
	if e != nil {
		return empty, e
	}
	cap, e := sourcingScaled("approved_amount", approvedAmount, 2, 16, false)
	if e != nil {
		return empty, e
	}
	spent, e := sourcingScaled("used_amount", usedAmount, 2, 16, false)
	if e != nil {
		return empty, e
	}
	value, e := sourcingScaled("amount", amount, 2, 16, false)
	if e != nil {
		return empty, e
	}
	remaining := new(big.Int).Sub(approved, used)
	available := new(big.Int).Sub(cap, spent)
	if remaining.Sign() < 0 || available.Sign() < 0 {
		return empty, SourcingInvalid("allocation", "Current reservations exceed the approved source; review the owning ledger")
	}
	if q.Cmp(remaining) > 0 {
		return empty, SourcingInvalid("quantity", "Other purchase packages already reserved this approved quantity")
	}
	if value.Cmp(available) > 0 {
		return empty, SourcingInvalid("financial_approval", "Other purchase packages already reserved this approved amount")
	}
	return SourcingAllocation{Quantity: sourcingQuantityString(q), Amount: sourcingFormat(value), RemainingQuantity: sourcingQuantityString(new(big.Int).Sub(remaining, q)), RemainingAmount: sourcingFormat(new(big.Int).Sub(available, value))}, nil
}

// ScopePrice projects actual approved commercial evidence onto a released part.
// Rates and percentages remain exact; charges and direct bases are proportional.
func SourcingScopePrice(price SourcingCommercialPrice, quantity, approvedQuantity string) (SourcingCommercialPrice, error) {
	q, e := sourcingScaled("quantity", quantity, 4, 19, true)
	if e != nil {
		return SourcingCommercialPrice{}, e
	}
	full, e := sourcingScaled("approved_quantity", approvedQuantity, 4, 19, true)
	if e != nil || q.Cmp(full) > 0 {
		return SourcingCommercialPrice{}, SourcingInvalid("quantity", "Released quantity exceeds approved quotation quantity")
	}
	if q.Cmp(full) == 0 {
		return price, nil
	}
	mode, e := sourcingPricingMode(price.Mode)
	if e != nil {
		return SourcingCommercialPrice{}, e
	}
	proportion := func(field, amount string) (*big.Int, error) {
		v, e := sourcingScaled(field, amount, 2, 16, false)
		if e != nil {
			return nil, e
		}
		return sourcingHalfUp(new(big.Int).Mul(v, q), full), nil
	}
	var base *big.Int
	if mode == "line_total" {
		base, e = proportion("base", price.Base)
	} else {
		var rate *big.Int
		rate, e = sourcingScaled("unit_price", price.Rate, 4, 18, false)
		if e == nil {
			base = sourcingHalfUp(new(big.Int).Mul(q, rate), big.NewInt(1000000))
		}
	}
	if e != nil {
		return SourcingCommercialPrice{}, e
	}
	percent := func(field, s string) (*big.Int, error) {
		p, e := sourcingScaled(field, s, 2, 5, false)
		if e != nil || p.Cmp(big.NewInt(10000)) > 0 {
			return nil, SourcingInvalid(field, "Current approved percentage evidence is required")
		}
		return p, nil
	}
	discount, e := percent("discount_percent", price.DiscountPercent)
	if e != nil {
		return SourcingCommercialPrice{}, e
	}
	tax, e := percent("tax_percent", price.TaxPercent)
	if e != nil {
		return SourcingCommercialPrice{}, e
	}
	reduction := sourcingHalfUp(new(big.Int).Mul(base, discount), big.NewInt(10000))
	taxAmount := sourcingHalfUp(new(big.Int).Mul(new(big.Int).Sub(base, reduction), tax), big.NewInt(10000))
	charges, e := proportion("charges", price.Charges)
	if e != nil {
		return SourcingCommercialPrice{}, e
	}
	price.Base = sourcingFormat(base)
	price.Discount = sourcingFormat(reduction)
	price.Tax = sourcingFormat(taxAmount)
	price.Charges = sourcingFormat(charges)
	return price, nil
}
func sourcingHalfUp(n, d *big.Int) *big.Int {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, d, r)
	if new(big.Int).Mul(r, big.NewInt(2)).Cmp(d) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}
func sourcingDirectBase(p SourcingCommercialPrice) string {
	if p.Mode == "line_total" {
		return p.Base
	}
	return ""
}
func sourcingUnitRate(p SourcingCommercialPrice) string {
	if p.Mode != "line_total" {
		return p.Rate
	}
	return ""
}

type SourcingBalances struct{ Quantity, Amount string }
