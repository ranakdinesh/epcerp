package dtos

import (
	"strings"
	"testing"
)

func TestEngineerConfirmationsEngineerStrictCommand(t *testing.T) {
	for _, v := range []string{`{"version":0,"action":"record","reason":"Actual","confirmation":{"status":"confirmed","confirmation_date":"2026-10-11","engineer_name":"Actual engineer","mbook_ref":"MB-1"}}`, `{"version":2,"action":"revise","reason":"Correction"}`} {
		if _, e := EngineerConfirmationsDecode(strings.NewReader(v)); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{
		`{"version":0,"action":"record","reason":"r","confirmation":{"status":"pending","accepted_qty":"forged"}}`,
		`{"version":0,"action":"record","reason":"r","confirmation":{"status":"pending","actor_id":"forged"}}`,
		`{"version":0,"action":"record","reason":"r","confirmation":{"status":"pending","engineer_name":null}}`,
		`{"version":0,"action":"record","reason":"r","confirmation":{"status":"pending","status":"confirmed"}}`,
		`{"version":0,"action":"record","reason":"r","confirmation":{"status":"pending"},"source_snapshot":{}}`,
		`{"version":1,"action":"revise","reason":"r","confirmation":{}}`,
	} {
		if _, e := EngineerConfirmationsDecode(strings.NewReader(v)); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
