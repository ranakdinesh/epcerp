package dtos

import (
	"strings"
	"testing"
)

func TestInspectionsInspectionDecoderRetainsExplicitWitnessFlagsAndRejectsForgedSource(t *testing.T) {
	valid := `{"version":0,"action":"record","reason":"actual inspection","inspection":{"status":"not_scheduled","witness_points":[{"code":"WP1","title":"Actual test","is_witness":true,"is_cleared":false}]}}`
	q, e := InspectionsDecode(strings.NewReader(valid))
	if e != nil || q.Inspection == nil || q.Inspection.Points[0].Cleared == nil || *q.Inspection.Points[0].Cleared {
		t.Fatal("Explicit false witness result was lost", e)
	}
	for _, bad := range []string{strings.Replace(valid, `"version":0`, `"version":0,"version":1`, 1), strings.Replace(valid, `"reason":"actual inspection"`, `"reason":"actual inspection","actor_id":"forged"`, 1), strings.Replace(valid, `"status":"not_scheduled"`, `"status":"not_scheduled","qap_approved":true`, 1), valid + valid, `{"version":1,"action":"revise","reason":"r","inspection":{}}`, `{"version":0,"action":"record","reason":"r","inspection":null}`, strings.Repeat("x", 65537)} {
		if _, e := InspectionsDecode(strings.NewReader(bad)); e == nil {
			t.Fatal("Forged/malformed command accepted")
		}
	}
	if _, e := InspectionsDecode(strings.NewReader(`{"version":1,"action":"revise","reason":"actual correction"}`)); e != nil {
		t.Fatal("Revision command rejected", e)
	}
}
