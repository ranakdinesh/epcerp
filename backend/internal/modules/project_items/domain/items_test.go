package domain

import (
	"testing"
)

func TestExactQuantityAndLocationAllocations(t *testing.T) {
	for _, input := range []string{"0", "-1", "NaN", "1e3", "1.0001", "10000000000000"} {
		if _, e := Quantity(input); e == nil {
			t.Fatal("invalid quantity", input)
		}
	}
	if v, e := Quantity("9999999999999.999"); e != nil || v != 9999999999999999 {
		t.Fatal("exact maximum", v, e)
	}
	s := Save{ItemID: "11111111-1111-1111-1111-111111111111", Quantity: "10.125", RequiredDate: "2026-10-12", Version: 1, Active: true, Reason: "Capture requirement", Allocations: []Allocation{{Location: "Site A", Quantity: "6"}, {Location: "Site B", Quantity: "4.125"}}}
	if _, e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	if e := s.ValidateUnit("M"); e != nil {
		t.Fatal(e)
	}
	if e := s.ValidateUnit("NOS"); e == nil {
		t.Fatal("fractional counted material")
	}
	s.Allocations[1].Location = " site a "
	if _, e := s.Validate(); e == nil {
		t.Fatal("duplicate location")
	}
	s.Allocations[1].Location = "Site B"
	s.Allocations[1].Quantity = "4"
	if _, e := s.Validate(); e == nil {
		t.Fatal("allocation sum")
	}
}
func TestReleasedQuantityProtections(t *testing.T) {
	if e := CommitmentsGuard("10", true, "10.000"); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		quantity string
		active   bool
	}{{"9.999", true}, {"10", false}} {
		if e := CommitmentsGuard(tc.quantity, tc.active, "10"); e == nil {
			t.Fatal("released commitment changed")
		}
	}
	if e := CommitmentsGuard("1", false, "0.000"); e != nil {
		t.Fatal("zero commitment", e)
	}
}
