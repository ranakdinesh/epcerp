package dtos

import (
	"strings"
	"testing"
)

func TestInspectionReportsReportDeclaredCommandShapes(t *testing.T) {
	valid := `{"version":0,"action":"record","reason":"Actual report","report":{"status":"pending","cleared_quantity":"0"}}`
	q, e := InspectionReportsDecode(strings.NewReader(valid))
	if e != nil || q.Report == nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(valid, `"status":"pending"`, `"status":"pending","uom":"forged"`, 1), strings.Replace(valid, `"status":"pending"`, `"status":"pending","actor_id":"forged"`, 1), strings.Replace(valid, `"version":0`, `"version":0,"version":1`, 1), strings.Replace(valid, `"status":"pending"`, `"status":"pending","status":"issued"`, 1), valid + valid, `{"version":0,"action":"record","reason":"r","report":null}`, `{"version":1,"action":"revise","reason":"r","report":{"status":"pending"}}`, `{"action":"record","reason":"r","report":{"status":"pending"}}`, strings.Repeat("x", 65537)} {
		if _, e := InspectionReportsDecode(strings.NewReader(bad)); e == nil {
			t.Fatal("undeclared report allowed")
		}
	}
	if _, e = InspectionReportsDecode(strings.NewReader(`{"version":2,"action":"revise","reason":"Actual report correction"}`)); e != nil {
		t.Fatal(e)
	}
}
