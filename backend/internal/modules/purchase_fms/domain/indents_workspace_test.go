package domain

import (
	"testing"
)

func TestIndentsRemainingIncludesWithdrawnCommitmentsAndRetainsExactDecimals(t *testing.T) {
	rows := []IndentsRecord{{Active: false, Data: IndentsData{Released: []IndentsRelease{{IndentsLine: IndentsLine{DemandID: "a", Quantity: "1.111"}}, {IndentsLine: IndentsLine{DemandID: "other", Quantity: "9"}}}}}, {Active: true, Data: IndentsData{Released: []IndentsRelease{{IndentsLine: IndentsLine{DemandID: "a", Quantity: "2.222"}}}}}}
	used, left, e := IndentsRemaining("10", rows, "a")
	if e != nil || used != "3.333" || left != "6.667" {
		t.Fatal(used, left, e)
	}
	used, left, e = IndentsRemaining("1", rows, "a")
	if e != nil || used != "3.333" || left != "0.000" {
		t.Fatal("Existing commitments disappeared after a source reduction", used, left, e)
	}
	rows[0].Data.Released[0].Quantity = "NaN"
	if _, _, e = IndentsRemaining("10", rows, "a"); e == nil {
		t.Fatal("Invalid stored quantity accepted")
	}
}
