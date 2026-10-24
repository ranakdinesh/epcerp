package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManufacturerStrictManufacturerPayload(t *testing.T) {
	body := `{"code":"MAKE","name":" Make ","data":{"eligible":[" ACTIVE "],"notes":" Notes "},"reason":" reason "}`
	v, ok := manufacturerParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body)), "create")
	if !ok || v.Name != "Make" || v.Data.Eligible[0] != "ACTIVE" || v.Data.Notes != "Notes" {
		t.Fatal(v)
	}
	for _, body := range []string{`{"code":"lower","name":"Make","data":{"eligible":["A"]},"reason":"reason"}`, `{"code":"MAKE","name":"Make","data":{"eligible":[null]},"reason":"reason"}`, `{"code":"MAKE","name":"Make","data":{"eligible":["A"],"extra":true},"reason":"reason"}`, `{"Code":"MAKE","name":"Make","data":{"eligible":["A"]},"reason":"reason"}`, `{"is_active":null,"version":1,"reason":"reason"}`} {
		if _, ok := manufacturerParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body)), "create"); ok {
			t.Fatal("invalid accepted")
		}
	}
}
