package dtos

import (
	"strings"
	"testing"
)

func TestOrdersCommandsRejectForgedPricesActorsDuplicateNullAndWrongActionFields(t *testing.T) {
	good := `{"version":1,"action":"release","reason":"Release reviewed order"}`
	if _, e := OrdersDecode(strings.NewReader(good)); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{"action":"release","reason":"Missing version"}`, `{"version":1,"action":"release","reason":"r","actor_id":"forged"}`, `{"version":1,"action":"release","reason":"r","total":"100"}`, `{"version":1,"version":2,"action":"release","reason":"r"}`, `{"version":1,"action":"release","reason":null}`, `{"version":1,"action":"release","reason":"r","details":{}}`, good + good, `{"version":1,"action":"prepare","reason":"r","details":{"date":"2026-10-11","date":"2026-10-12"}}`, strings.Repeat(" ", 65537)} {
		if _, e := OrdersDecode(strings.NewReader(raw)); e == nil {
			t.Fatal("invalid PO command accepted", raw[:min(len(raw), 160)])
		}
	}
}
