package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSourcingStrictActionCommands(t *testing.T) {
	for _, body := range []string{`{"action":"decide","version":1,"reason":"Ambiguous decision","approve":false,"approve":true}`, `null`, `{"action":"decide","version":1,"reason":"Decision","approve":null}`, `{"action":"decide","version":1,"reason":"Decision","approve":true,"supplier_id":"33333333-3333-4333-8333-333333333333"}`, `{"action":"reopen","reason":"Review"}`, `{"action":"reopen","version":null,"reason":"Review"}`, `{"action":"reopen","version":1,"reason":"Review","actor_id":"forged"}`, `{"action":"reopen","version":1,"reason":"Review"} {}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		if _, ok := sourcingDecode(w, r); ok || w.Code != 422 {
			t.Fatal("accepted unsafe action", body, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", 65<<10)))
	w := httptest.NewRecorder()
	if _, ok := sourcingDecode(w, r); ok || w.Code != 413 {
		t.Fatal("unbounded command")
	}
	r = httptest.NewRequest("POST", "/", strings.NewReader(`{"action":"decide","version":1,"reason":"Reject","approve":false}`))
	w = httptest.NewRecorder()
	q, ok := sourcingDecode(w, r)
	if !ok || q.Approve == nil || *q.Approve {
		t.Fatal("explicit rejection lost")
	}
}
