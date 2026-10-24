package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuotationApprovalsStrictSavedSourceCommands(t *testing.T) {
	for _, q := range []struct {
		action, body string
		valid        bool
	}{{"submit", `{"version":0,"reason":"Choose quote","quote_key":"22222222-2222-4222-8222-222222222222:1"}`, true}, {"review", `{"version":1,"reason":"Decide","attempt_id":"22222222-2222-4222-8222-222222222222","decisions":[{"line_id":"11111111-1111-4111-8111-111111111111","decision":"accept","comment":"Reviewed"}]}`, true}, {"submit", `{"version":null,"reason":"x","quote_key":"22222222-2222-4222-8222-222222222222:1"}`, false}, {"submit", `{"version":0,"reason":"x","quote_key":"22222222-2222-4222-8222-222222222222:1","total":"1.00"}`, false}, {"review", `{"version":1,"reason":"x","attempt_id":"22222222-2222-4222-8222-222222222222","decisions":[]}`, false}, {"review", `{"version":1,"reason":"x","attempt_id":"22222222-2222-4222-8222-222222222222","decisions":[{"line_id":"11111111-1111-4111-8111-111111111111","decision":"accept","comment":"Reviewed","budget":"0.00"}]}`, false}, {"submit", `{"version":0,"reason":"x","quote_key":"22222222-2222-4222-8222-222222222222:1"} {}`, false}} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(q.body))
		_, e := quotationApprovalsDecode(httptest.NewRecorder(), r, q.action)
		if (e == nil) != q.valid {
			t.Fatal("strict command", q.action, q.valid, e)
		}
	}
}

func TestQuotationApprovalsReviewSupportsBoundedHundredItemMultilingualComments(t *testing.T) {
	rows := make([]map[string]string, 0, 100)
	for i := 0; i < 100; i++ {
		rows = append(rows, map[string]string{"line_id": fmt.Sprintf("%08x-1111-4111-8111-111111111111", i+1), "decision": "accept", "comment": strings.Repeat("文", 2000)})
	}
	raw, e := json.Marshal(map[string]any{"version": 1, "reason": "Review all current items", "attempt_id": "22222222-2222-4222-8222-222222222222", "decisions": rows})
	if e != nil || len(raw) < 512<<10 {
		t.Fatal("fixture size", e)
	}
	q, e := quotationApprovalsDecode(httptest.NewRecorder(), httptest.NewRequest("POST", "/", bytes.NewReader(raw)), "review")
	if e != nil || len(q.Decisions) != 100 {
		t.Fatal("valid bounded review rejected", e)
	}
}
