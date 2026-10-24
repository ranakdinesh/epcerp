package dtos

import (
	"strings"
	"testing"
)

func TestClientClearanceClientClearanceStrictCommand(t *testing.T) {
	for _, v := range []string{`{"version":0,"action":"record","reason":"Record actual client MDCC","clearance":{"status":"cleared","mdcc_ref":"MDCC-1","clearance_date":"2026-10-11","cleared_by_authority":"Client","remarks":"Actual instructions","file_name":"ref.pdf"}}`, `{"version":2,"action":"revise","reason":"Client correction"}`} {
		if _, e := ClientClearanceDecode(strings.NewReader(v)); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{`{"version":0,"action":"record","reason":"x","clearance":{"status":"cleared","actor_id":"forged"}}`, `{"version":0,"action":"record","reason":"x","clearance":{"status":"pending","uom":"forged"}}`, `{"version":0,"action":"record","reason":"x","clearance":{"status":"pending"},"source_snapshot":{}}`, `{"version":0,"action":"record","reason":"x","clearance":{"status":"pending","mdcc_ref":null}}`, `{"version":1,"action":"revise","reason":"x","clearance":{}}`, `{"version":0,"action":"record","reason":"x","clearance":{"status":"pending","status":"cleared"}}`} {
		if _, e := ClientClearanceDecode(strings.NewReader(v)); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
