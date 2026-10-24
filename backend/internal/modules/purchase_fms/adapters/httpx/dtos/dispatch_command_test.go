package dtos

import (
	"strings"
	"testing"
)

func TestDispatchMaterialDispatchStrictCommand(t *testing.T) {
	for _, v := range []string{`{"version":0,"action":"record","reason":"Record actual dispatch","dispatch":{"status":"dispatched","lr_number":"LR-1","dispatch_date":"2026-10-11","transporter_name":"Actual transporter","file_name":"ref.pdf"}}`, `{"version":2,"action":"revise","reason":"Client correction"}`} {
		if _, e := DispatchDecode(strings.NewReader(v)); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{`{"version":0,"action":"record","reason":"x","dispatch":{"status":"dispatched","actor_id":"forged"}}`, `{"version":0,"action":"record","reason":"x","dispatch":{"status":"pending","uom":"forged"}}`, `{"version":0,"action":"record","reason":"x","dispatch":{"status":"pending"},"source_snapshot":{}}`, `{"version":0,"action":"record","reason":"x","dispatch":{"status":"pending","lr_number":null}}`, `{"version":1,"action":"revise","reason":"x","dispatch":{}}`, `{"version":0,"action":"record","reason":"x","dispatch":{"status":"pending","status":"dispatched"}}`} {
		if _, e := DispatchDecode(strings.NewReader(v)); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
