package inbound

import (
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStrictBudgetCommands(t *testing.T) {
	valid := `{"action":"save","version":0,"reason":"Draft preparation","data":{"total":99999999999999.99,"currency":"INR","owner_id":"211bac51-6cf7-4224-b7cc-4c5af0d47e85","heads":[],"allocations":[]}}`
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"exact decimal", valid, 0},
		{"missing version", strings.Replace(valid, `"version":0,`, "", 1), 422},
		{"null version", strings.Replace(valid, `"version":0`, `"version":null`, 1), 422},
		{"unknown audit", strings.Replace(valid, `"heads":[]`, `"created_by_id":"forged","heads":[]`, 1), 422},
		{"null array", strings.Replace(valid, `"heads":[]`, `"heads":null`, 1), 422},
		{"nullable row", strings.Replace(valid, `"heads":[]`, `"heads":[null]`, 1), 422},
		{"exponent", strings.Replace(valid, `99999999999999.99`, `1e4`, 1), 422},
		{"float version", strings.Replace(valid, `"version":0`, `"version":0.5`, 1), 422},
		{"approval saved source", `{"action":"approve","version":1,"reason":"Approve"}`, -1},
		{"revision saved source", `{"action":"revise","version":1,"reason":"Revise"}`, -1},
		{"approval forged data", `{"action":"approve","version":1,"reason":"Approve","data":null}`, 422},
		{"revision ignored prices", `{"action":"revise","version":1,"reason":"Revise","prices":[]}`, 422},
		{"ignored prices", strings.Replace(valid, `"reason":"Draft preparation"`, `"reason":"Draft preparation","prices":[{"bo_id":"211bac51-6cf7-4224-b7cc-4c5af0d47e85","line_id":"211bac51-6cf7-4224-b7cc-4c5af0d47e85","bo_version":1,"quantity":"1","changed":false}]`, 1), 422},
		{"trailing JSON", valid + `{}`, 422},
		{"oversized", strings.Repeat(" ", 512<<10) + valid, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, e := decode(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(tc.body)))
			if tc.status == -1 {
				if e != nil {
					t.Fatal("saved source command rejected", e)
				}
				return
			}
			if tc.status == 0 {
				if e != nil || v.Data.Total != "99999999999999.99" {
					t.Fatal("lost exact decimal", e, v.Data)
				}
				return
			}
			var d *domain.Error
			if !errors.As(e, &d) || d.Status != tc.status {
				t.Fatal("unexpected error", e)
			}
		})
	}
}
func TestNestedBudgetDTOsExcludeAuditAndSource(t *testing.T) {
	head := `{"id":"a","parent_id":null,"name":"Purchase","kind":"purchase","amount":"1.00","owner_id":"b","is_active":false}`
	allocation := `{"id":"a","bo_id":"b","line_id":"c","head_id":"d","amount":"0","owner_id":"e","is_active":false,"unit_price":null,"pricing_quantity":null}`
	body := func(h, a string) string {
		return `{"action":"save","version":1,"reason":"Correction","data":{"total":"1","currency":"INR","owner_id":"b","heads":[` + h + `],"allocations":[` + a + `]}}`
	}
	v, e := decode(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body(head, allocation))))
	if e != nil || v.Data.Heads[0].Active || v.Data.Allocations[0].Source != nil {
		t.Fatal(v, e)
	}
	for _, field := range []string{`"created_by_id":"forged"`, `"updated_at":"2026-10-10"`, `"source":{"sku":"forged"}`} {
		forged := strings.TrimSuffix(allocation, "}") + "," + field + "}"
		if _, e = decode(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body(head, forged)))); e == nil {
			t.Fatal("accepted server field", field)
		}
	}
	if _, e = decode(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body(strings.Replace(head, `"is_active":false`, `"is_active":null`, 1), allocation)))); e == nil {
		t.Fatal("null active accepted")
	}
}
