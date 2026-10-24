package dtos

import (
	"strings"
	"testing"
)

func TestTechnicalCommandRejectsCallerAuthorityAndCapturedRevisionMetadata(t *testing.T) {
	for _, body := range []string{`{"version":0,"action":"configure","reason":"Create","actor_id":"forged"}`, `{"version":2,"action":"submit","reason":"Submit","submission":{"revision":2}}`, `{"version":3,"action":"client_approval","reason":"Approve","approval":{"recorded_by_id":"forged"}}`, `{} {}`, `null`, `{"version":0,"version":1,"action":"configure","reason":"Create"}`, `{"version":0,"action":"configure","reason":"Create","approval":{}}`, `{"version":0,"action":"configure","reason":null}`, strings.Repeat(" ", 65537)} {
		if _, e := TechnicalDecode(strings.NewReader(body)); e == nil {
			t.Fatal("unsafe command accepted")
		}
	}
	q, e := TechnicalDecode(strings.NewReader(`{"version":3,"action":"team_review","reason":"Reject","approve":false,"remarks":"Correct revision"}`))
	if e != nil || q.Approve == nil || *q.Approve {
		t.Fatal("explicit rejection lost", e)
	}
}
