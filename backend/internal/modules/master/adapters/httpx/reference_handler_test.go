package httpx

import (
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"net/http/httptest"
	"strings"
	"testing"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func TestReferenceStrictReferenceBoundary(t *testing.T) {
	for _, body := range []string{`{"name":"City","code":"ci","state_id":null}`, `{"name":"  Client Type  "}`} {
		in, _, e := referenceParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if e != nil || in.Name == "" {
			t.Fatal(in, e)
		}
	}
	for _, body := range []string{`{"Name":"City"}`, `{"name":"City","is_active":true}`, `{"name":"City","code":null}`, `{"name":"City","state_id":"invalid"}`, `{"name":" "}`, `{"name":"City"} {}`, `[]`} {
		if _, _, e := referenceParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body))); e == nil {
			t.Fatal("invalid accepted", body)
		}
	}
	if _, _, e := referenceParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", 4097)))); e == nil {
		t.Fatal("oversized accepted")
	}
}
func TestReferenceResponseAndBooleanContract(t *testing.T) {
	raw, e := masterDTO.MarshalMasterJSON(masterDTO.ReferenceFromDomain(masterDomain.Reference{ID: "id", Name: "State", Code: "ST", Active: true}))
	if e != nil || !strings.Contains(string(raw), `"state_id":null`) || strings.Contains(string(raw), "CreatedAt") {
		t.Fatal(string(raw), e)
	}
	for _, s := range []string{"true", "1", "on", "YES"} {
		if b, ok := masterDTO.ReferenceBoolean(s); !b || !ok {
			t.Fatal(s)
		}
	}
	if _, ok := masterDTO.ReferenceBoolean("invalid"); ok {
		t.Fatal("invalid boolean")
	}
}
