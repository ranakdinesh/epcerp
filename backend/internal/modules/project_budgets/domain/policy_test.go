package domain

import (
	"fmt"
	"testing"
	"time"
)

func id(n int) string          { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
func pointer(s string) *string { return &s }
func fixture() (Data, Scope) {
	owner := id(1)
	value := "1000.00"
	scope := Scope{Project: Project{ID: id(2), Currency: "INR", ContractValue: &value, Available: true}, Owners: []Owner{{ID: owner}}, Items: []Item{{BOID: id(6), LineID: id(5), BOVersion: 2, Quantity: "10"}}}
	data := Data{Total: value, Currency: "INR", OwnerID: owner, Heads: []Head{{ID: id(3), Name: "Purchase", Kind: "purchase", Amount: value, OwnerID: owner, Active: true}}, Allocations: []Allocation{{ID: id(7), BOID: id(6), LineID: id(5), HeadID: id(3), OwnerID: owner, Amount: "100.00", UnitPrice: pointer("10.00"), PricingQuantity: pointer("10"), Active: true}}}
	return data, scope
}
func TestBudgetMoneyAndHalfUp(t *testing.T) {
	for _, tc := range []struct{ qty, rate, want string }{{"0.005", "1.00", "0.01"}, {"3", "0.33", "0.99"}, {"1.2345", "10.00", "12.35"}, {"10", "0", "0.00"}} {
		v, e := LineBudget(tc.qty, tc.rate)
		if e != nil || v != tc.want {
			t.Fatalf("%s*%s=%s %v", tc.qty, tc.rate, v, e)
		}
	}
	for _, raw := range []string{"NaN", "Infinity", "-1", "1e2", "0.001", "100000000000000.00"} {
		if _, e := Money(raw); e == nil {
			t.Fatalf("accepted budget amount %s", raw)
		}
	}
	if _, e := LineBudget("99999999999999.9999", "99999999999999.99"); e == nil {
		t.Fatal("overflow accepted")
	}
}

func TestCommandRejectsMalformedPriceInput(t *testing.T) {
	valid := Price{BOID: id(6), LineID: id(5), BOVersion: 2, Quantity: "10", UnitPrice: pointer("10")}
	for _, change := range []func(*Price){
		func(p *Price) { p.BOID = "invalid" },
		func(p *Price) { p.LineID = "invalid" },
		func(p *Price) { p.BOVersion = 0 },
		func(p *Price) { p.Quantity = "0" },
		func(p *Price) { p.Quantity = "1e2" },
		func(p *Price) { p.UnitPrice = pointer("1.001") },
	} {
		p := valid
		change(&p)
		if _, e := (Command{Action: "price_items", Reason: "Update", Prices: []Price{p}}).Validate(); e == nil {
			t.Fatal("invalid price input accepted")
		}
	}
	if _, e := (Command{Action: "price_items", Reason: "Update", Prices: []Price{valid, valid}}).Validate(); e == nil {
		t.Fatal("duplicate price input accepted")
	}
}
func TestBudgetValidationAndReport(t *testing.T) {
	d, s := fixture()
	v, e := Validate(d, s, nil)
	if e != nil {
		t.Fatal(e)
	}
	if v.Allocations[0].Source == nil || v.Allocations[0].Source.BOVersion != 2 {
		t.Fatal("source snapshot missing")
	}
	r, e := BuildReport(v)
	if e != nil || r.Total != "1000.00" || r.PurchaseTotal != "1000.00" || r.ItemAllocations != "100.00" || r.Heads[0].Unallocated != "900.00" || r.Unallocated != "0.00" {
		t.Fatalf("rollup %+v %v", r, e)
	}
}
func TestBudgetValidationGuards(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Data, *Scope)
	}{
		{"invalid pricing mode", func(d *Data, s *Scope) { d.PricingMode = "caller_invented" }},
		{"wrong currency", func(d *Data, s *Scope) { d.Currency = "USD" }},
		{"inactive budget owner", func(d *Data, s *Scope) { d.OwnerID = id(9) }},
		{"inactive head owner", func(d *Data, s *Scope) { d.Heads[0].OwnerID = id(9) }},
		{"inactive allocation owner", func(d *Data, s *Scope) { d.Allocations[0].OwnerID = id(9) }},
		{"duplicate head identity", func(d *Data, s *Scope) { d.Heads = append(d.Heads, d.Heads[0]) }},
		{"duplicate sibling name", func(d *Data, s *Scope) {
			h := d.Heads[0]
			h.ID = id(8)
			h.Name = " PURCHASE "
			d.Heads = append(d.Heads, h)
		}},
		{"duplicate allocation", func(d *Data, s *Scope) { a := d.Allocations[0]; a.ID = id(8); d.Allocations = append(d.Allocations, a) }},
		{"foreign project BO line", func(d *Data, s *Scope) { d.Allocations[0].LineID = id(9) }},
		{"allocation into nonpurchase", func(d *Data, s *Scope) { d.Heads[0].Kind = "labour" }},
		{"inactive parent", func(d *Data, s *Scope) { d.Heads[0].ParentID = pointer(id(9)) }},
		{"cycle", func(d *Data, s *Scope) { d.Heads[0].ParentID = pointer(d.Heads[0].ID) }},
		{"stale quantity", func(d *Data, s *Scope) { s.Items[0].Quantity = "11" }},
		{"forged total", func(d *Data, s *Scope) { d.Allocations[0].Amount = "101.00" }},
		{"heads exceed total", func(d *Data, s *Scope) { d.Total = "999.99" }},
		{"allocations exceed head", func(d *Data, s *Scope) { d.Heads[0].Amount = "99.99" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, s := fixture()
			tc.change(&d, &s)
			if _, e := Validate(d, s, nil); e == nil {
				t.Fatal("invalid budget accepted")
			}
		})
	}
}

func TestAllocationIdentityAndHistoricalSourceSurviveCorrection(t *testing.T) {
	d, s := fixture()
	before, e := Validate(d, s, nil)
	if e != nil {
		t.Fatal(e)
	}
	d = CopyData(before)
	d.Allocations[0].Active = false
	d.Allocations[0].Source = nil
	saved, e := Validate(d, s, &before)
	if e != nil || saved.Allocations[0].Source == nil || saved.Allocations[0].Source.BOVersion != 2 {
		t.Fatal("historical source lost", e)
	}
	d = CopyData(before)
	d.Allocations[0].LineID = id(10)
	item := s.Items[0]
	item.LineID = id(10)
	s.Items = append(s.Items, item)
	if _, e = Validate(d, s, &before); e == nil {
		t.Fatal("allocation retargeted to another valid source")
	}
}
func TestHierarchyDepthAndCategory(t *testing.T) {
	d, s := fixture()
	for n := 4; n <= 8; n++ {
		parent := id(n - 1)
		d.Heads = append(d.Heads, Head{ID: id(n), ParentID: &parent, Name: fmt.Sprint("Subhead ", n), Kind: "purchase", Amount: "100.00", OwnerID: id(1), Active: true})
	}
	if _, e := Validate(d, s, nil); e != nil {
		t.Fatal("six levels should be accepted", e)
	}
	parent := id(8)
	d.Heads = append(d.Heads, Head{ID: id(9), ParentID: &parent, Name: "Seventh level", Kind: "purchase", Amount: "100", OwnerID: id(1), Active: true})
	if _, e := Validate(d, s, nil); e == nil {
		t.Fatal("seventh level accepted")
	}
	d.Heads = d.Heads[:6]
	d.Heads[1].Kind = "labour"
	if _, e := Validate(d, s, nil); e == nil {
		t.Fatal("mixed categories in hierarchy")
	}
}
func TestDeactivateAndAuditIdentity(t *testing.T) {
	d, s := fixture()
	before := CopyData(d)
	d.Heads = nil
	if _, e := Validate(d, s, &before); e == nil {
		t.Fatal("head deleted")
	}
	d = CopyData(before)
	d.Allocations = nil
	if _, e := Validate(d, s, &before); e == nil {
		t.Fatal("allocation deleted")
	}
	d = CopyData(before)
	d.Heads[0].Active = false
	d.Allocations[0].Active = false
	if _, e := Validate(d, s, &before); e != nil {
		t.Fatal("deactivation", e)
	}
	first := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	later := first.Add(time.Hour)
	before = Attribute(before, nil, id(1), first)
	d = Attribute(d, &before, id(9), later)
	if d.Heads[0].CreatedAt != first || *d.Heads[0].CreatedBy != id(1) || *d.Heads[0].UpdatedBy != id(9) || d.Heads[0].UpdatedAt != later {
		t.Fatal("audit identity overwritten")
	}
}
func TestPricePurchaseItemsReconcileAndLegacy(t *testing.T) {
	d, s := fixture()
	prices := []Price{{BOID: id(6), LineID: id(5), BOVersion: 2, Quantity: "10.0000", UnitPrice: pointer("12.34"), Changed: true}}
	out, e := PriceItems(&d, prices, s.Items, id(1), "1000", "INR", id(8), map[string]string{})
	if e != nil {
		t.Fatal(e)
	}
	if out.Allocations[0].Amount != "123.40" || out.Heads[0].Amount != "123.40" || out.Total != "1000.00" || out.Allocations[0].ID != id(7) {
		t.Fatalf("reconciled budget %+v", out)
	}
	prices[0].BOVersion = 1
	if _, e = PriceItems(&d, prices, s.Items, id(1), "1000", "INR", id(8), nil); e != Stale {
		t.Fatal("stale BO version", e)
	}
	prices[0].BOVersion = 2
	prices[0].Quantity = "11"
	if _, e = PriceItems(&d, prices, s.Items, id(1), "1000", "INR", id(8), nil); e != Stale {
		t.Fatal("stale quantity", e)
	}
	prices[0].Quantity = "10"
	prices[0].UnitPrice = nil
	out, e = PriceItems(&d, prices, s.Items, id(1), "1000", "INR", id(8), nil)
	if e != nil || out.Allocations[0].Active || out.Heads[0].Amount != "0.00" {
		t.Fatal("clear unit budget", e)
	}
	d.Allocations[0].UnitPrice = nil
	d.Allocations[0].PricingQuantity = nil
	prices[0].Changed = false
	out, e = PriceItems(&d, prices, s.Items, id(1), "1000", "INR", id(8), nil)
	if e != nil || !out.Allocations[0].Active || out.Allocations[0].Amount != "100.00" {
		t.Fatal("untouched legacy total lost", e)
	}
	if _, e = PriceItems(&d, nil, s.Items, id(1), "1000", "INR", id(8), nil); e == nil {
		t.Fatal("incomplete item set accepted")
	}
}
func TestPricePurchaseItemsPreservesNonPurchaseAndRemovedRecords(t *testing.T) {
	d, s := fixture()
	d.Heads = append(d.Heads, Head{ID: id(10), Name: "Labour", Kind: "labour", Amount: "50", OwnerID: id(1), Active: true})
	d.Allocations = append(d.Allocations, Allocation{ID: id(11), BOID: id(12), LineID: id(13), HeadID: id(3), OwnerID: id(1), Amount: "1", Active: true})
	p := []Price{{BOID: id(6), LineID: id(5), BOVersion: 2, Quantity: "10", Changed: false}}
	out, e := PriceItems(&d, p, s.Items, id(1), "1000", "INR", id(8), nil)
	if e != nil || len(out.Allocations) != 2 || out.Allocations[1].Active || out.Heads[1].Amount != "50" {
		t.Fatal("nonpurchase or historical record changed", e)
	}
}
