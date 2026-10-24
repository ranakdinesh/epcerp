package inbound

import (
	"strings"
	"testing"
)

func TestAccountsStrictCommand(t *testing.T) {
	for _, v := range []string{`{"version":0,"action":"record","reason":"Actual","acceptance":{"status":"accepted","acceptance_date":"2026-10-11","officer_name":"Actual officer","voucher_ref":"Actual","booked_amount":"0"}}`, `{"version":2,"action":"revise","reason":"Correction"}`} {
		if _, e := Decode(strings.NewReader(v)); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{
		`{"version":0,"action":"record","reason":"r","acceptance":{"status":"pending","accepted_qty":"forged"}}`,
		`{"version":0,"action":"record","reason":"r","acceptance":{"status":"pending","actor_id":"forged"}}`,
		`{"version":0,"action":"record","reason":"r","acceptance":{"status":"pending","officer_name":null}}`,
		`{"version":0,"action":"record","reason":"r","acceptance":{"status":"pending","status":"accepted"}}`,
		`{"version":0,"action":"record","reason":"r","acceptance":{"status":"pending"},"source_snapshot":{}}`,
		`{"version":1,"action":"revise","reason":"r","acceptance":{}}`,
	} {
		if _, e := Decode(strings.NewReader(v)); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
