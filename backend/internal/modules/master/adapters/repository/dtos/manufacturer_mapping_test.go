package dtos

import (
	"strings"
	"testing"
)

func TestManufacturerLegacySnapshotNumberPrecision(t *testing.T) {
	v, e := ManufacturerDataFromJSON([]byte(`{"legacy_quantity":9007199254740993,"nested":{"value":12345678901234567890}}`))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := MarshalMasterJSON(v)
	if e != nil || !strings.Contains(string(raw), "9007199254740993") || !strings.Contains(string(raw), "12345678901234567890") {
		t.Fatal("legacy snapshot rounded", string(raw), e)
	}
}
