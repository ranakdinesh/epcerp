package dtos

import (
	"encoding/json"
	"testing"
	"time"
)

func TestManufacturerCoercionsAndTimestampContracts(t *testing.T) {
	for _, raw := range []string{`1`} {
		if v, ok := manufacturerVersion(json.RawMessage(raw)); !ok || v != 1 {
			t.Fatal(raw)
		}
	}
	for _, raw := range []string{`1.0`, `"1"`, `true`, `null`, `0`, `1.5`, `2147483648`, `"1e0"`, `"0x1p0"`} {
		if _, ok := manufacturerVersion(json.RawMessage(raw)); ok {
			t.Fatal(raw)
		}
	}
	v := time.Date(2026, 10, 9, 8, 0, 0, 123000000, time.UTC)
	if manufacturerStamp(v) != "2026-10-09T08:00:00.123Z" {
		t.Fatal("RFC3339 precision")
	}
	if b, ok := JsonBoolean(json.RawMessage(`false`)); !ok || b {
		t.Fatal("bool")
	}
}
