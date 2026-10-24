package dtos

import (
	"strings"
	"testing"
)

func TestSiteReceiptsSiteReceiptStrictCommand(t *testing.T) {
	for _, v := range []string{`{"version":0,"action":"record","reason":"Arrival","receipt":{"status":"delivered_at_site","arrival_date":"2026-10-11","receiving_officer":"Receiver","packages_received":0,"packages_condition":"quarantine"}}`, `{"version":2,"action":"revise","reason":"Correction"}`} {
		if _, e := SiteReceiptsDecode(strings.NewReader(v)); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{
		`{"version":0,"action":"record","reason":"r","receipt":{"status":"pending","packages_received":1.5}}`,
		`{"version":0,"action":"record","reason":"r","receipt":{"status":"pending","packages_received":2147483648}}`,
		`{"version":0,"action":"record","reason":"r","receipt":{"status":"pending","arrival_date":null}}`,
		`{"version":0,"action":"record","reason":"r","receipt":{"status":"pending","status":"delivered_at_site"}}`,
		`{"version":0,"action":"record","reason":"r","receipt":{"status":"pending","actor_id":"forged"}}`,
		`{"version":0,"action":"record","reason":"r","receipt":{"status":"pending"},"source_snapshot":{}}`,
		`{"version":1,"action":"revise","reason":"r","receipt":{}}`,
	} {
		if _, e := SiteReceiptsDecode(strings.NewReader(v)); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
