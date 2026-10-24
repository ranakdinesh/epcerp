package domain

import (
	"testing"
)

func TestSourcingPartialAllocationsConserveQuantityAndApprovedValue(t *testing.T) {
	first, e := SourcingPlanAllocation("6", "10", "145.69", "0", "0", "87.41")
	if e != nil || first.RemainingQuantity != "4.0000" || first.RemainingAmount != "58.28" {
		t.Fatal(first, e)
	}
	second, e := SourcingPlanAllocation("4", "10", "145.69", first.Quantity, first.Amount, "58.27")
	if e != nil || second.RemainingQuantity != "0.0000" || second.RemainingAmount != "0.01" {
		t.Fatal(second, e)
	}
	// Rounding does not force an invented extra cent of expenditure.
	if _, e = SourcingPlanAllocation("0.0001", "10", "145.69", "10", "145.68", "0"); e == nil {
		t.Fatal("Quantity approval reused")
	}
	if _, e = SourcingPlanAllocation("4", "10", "145.69", "6", "87.41", "58.29"); e == nil {
		t.Fatal("Amount approval reused")
	}
}
func TestSourcingAllocationBoundsRemainExactBeyondFloatPrecision(t *testing.T) {
	a, e := SourcingPlanAllocation("0.0001", "999999999999999.9999", "99999999999999.99", "999999999999999.9998", "99999999999999.98", "0.01")
	if e != nil || a.RemainingQuantity != "0.0000" || a.RemainingAmount != "0.00" {
		t.Fatal(a, e)
	}
	for _, args := range [][6]string{
		{"1", "10", "100", "11", "0", "1"},
		{"1", "10", "100", "0", "101", "0"},
		{"0", "10", "100", "0", "0", "0"},
		{"1", "10", "100", "-1", "0", "0"},
		{"1e2", "1000", "100", "0", "0", "0"},
		{"1", "10", "100", "0", "0", "1.001"},
	} {
		if _, e = SourcingPlanAllocation(args[0], args[1], args[2], args[3], args[4], args[5]); e == nil {
			t.Fatal("Unsafe reservation", args)
		}
	}
}

func TestSourcingScopedApprovedPricingKeepsRatesPercentagesAndExactDirectBase(t *testing.T) {
	for _, mode := range []string{"unit_price", "line_total"} {
		p := SourcingCommercialPrice{Mode: mode, Rate: "12.3456", Base: "123.46", Discount: "0.00", Tax: "22.22", Charges: "0.01", DiscountPercent: "0", TaxPercent: "18"}
		firstBase := "74.07"
		if mode == "line_total" {
			firstBase = "74.08"
		}
		first, e := SourcingScopePrice(p, "6", "10")
		if e != nil || first.Base != firstBase || first.Tax != "13.33" || first.Charges != "0.01" {
			t.Fatal(mode, first, e)
		}
		second, e := SourcingScopePrice(p, "4", "10")
		if e != nil || second.Base != "49.38" || second.Tax != "8.89" || second.Charges != "0.00" {
			t.Fatal(mode, second, e)
		}
		if first.Rate != p.Rate || second.Rate != p.Rate {
			t.Fatal("rate precision changed")
		}
		if _, e = SourcingScopePrice(p, "11", "10"); e == nil {
			t.Fatal("over-approved quantity projected")
		}
		p.DiscountPercent = ""
		if _, e = SourcingScopePrice(p, "6", "10"); e == nil {
			t.Fatal("missing approved percentage invented")
		}
	}
}
