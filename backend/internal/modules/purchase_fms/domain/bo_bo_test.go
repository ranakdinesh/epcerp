package domain

import (
	"errors"
	"testing"
)

func TestBOBOIncompleteDraftAndCountedCompletion(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	quantity := "1.125"
	date := "2026-10-10"
	draft := BOSave{Version: 0, Reason: "Prepare draft", Lines: []BOInputLine{{ID: id}}}
	if _, e := draft.Validate(); e != nil {
		t.Fatal(e)
	}
	line := BOLine{ID: id}
	if e := BOValidateLine(line, false); e != nil {
		t.Fatal(e)
	}
	if e := BOValidateLine(line, true); e == nil {
		t.Fatal("incomplete completion")
	}
	line.ItemID = &id
	line.Quantity = &quantity
	line.RequiredDate = &date
	line.Snapshot.UOM = "NOS"
	if e := BOValidateLine(line, true); e == nil {
		t.Fatal("fractional count")
	}
	line.Snapshot.UOM = "M"
	if e := BOValidateLine(line, true); e != nil {
		t.Fatal(e)
	}
	draft.Lines = append(draft.Lines, draft.Lines[0])
	if _, e := draft.Validate(); e == nil {
		t.Fatal("duplicate row")
	}
}
func TestBOBOOwnershipVersionAndCompletedImmutability(t *testing.T) {
	r := BORecord{Version: 2, Active: true, State: "draft", PreparerID: "owner"}
	owner := Principal{ID: "owner", Active: true}
	other := Principal{ID: "other", Active: true}
	if e := BOEditable(r, other, 2); !errors.Is(e, BOForbidden) {
		t.Fatal(e)
	}
	if e := BOEditable(r, owner, 1); !errors.Is(e, BOStale) {
		t.Fatal(e)
	}
	r.State = "completed"
	if e := BOEditable(r, owner, 2); e == nil {
		t.Fatal("completed edit")
	}
	r.State = "draft"
	other.Superuser = true
	if e := BOEditable(r, other, 2); e != nil {
		t.Fatal(e)
	}
	other.Active = false
	if other.Can("projects.purchase_bo.write") {
		t.Fatal("inactive admin")
	}
}
func TestBOBOQuantityPrecisionAndCalendar(t *testing.T) {
	for _, v := range []string{"0", "-1", "NaN", "1e3", "1.0001", "10000000000000"} {
		if _, e := BOQuantity(v); e == nil {
			t.Fatal(v)
		}
	}
	if v, e := BOQuantity("9999999999999.999"); e != nil || v != 9999999999999999 {
		t.Fatal(v, e)
	}
	for _, v := range []string{"2026-02-29", "0000-01-01", "2026-1-1"} {
		if BODate(v) {
			t.Fatal(v)
		}
	}
}
