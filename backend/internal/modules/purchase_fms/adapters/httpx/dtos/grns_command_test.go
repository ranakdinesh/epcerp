package dtos

import (
	"strings"
	"testing"
)

func TestGRNsGRNStrictDecimalCommand(t *testing.T) {
	for _, v := range []string{`{"version":0,"action":"record","reason":"Actual","grn":{"status":"issued","grn_number":"GRN-1","grn_date":"2026-10-11","received_qty":"1.000001","accepted_qty":"1","rejected_qty":"0.000001"}}`, `{"version":2,"action":"revise","reason":"Correction"}`} {
		if _, e := GRNsDecode(strings.NewReader(v)); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{
		`{"version":0,"action":"record","reason":"r","grn":{"status":"pending","received_qty":1.000001}}`,
		`{"version":0,"action":"record","reason":"r","grn":{"status":"pending","uom":"forged"}}`,
		`{"version":0,"action":"record","reason":"r","grn":{"status":"pending","actor_id":"forged"}}`,
		`{"version":0,"action":"record","reason":"r","grn":{"status":"pending","grn_number":null}}`,
		`{"version":0,"action":"record","reason":"r","grn":{"status":"pending","status":"issued"}}`,
		`{"version":0,"action":"record","reason":"r","grn":{"status":"pending"},"source_snapshot":{}}`,
		`{"version":1,"action":"revise","reason":"r","grn":{}}`,
	} {
		if _, e := GRNsDecode(strings.NewReader(v)); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
