package httpx

import (
	"encoding/json"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func clientPayload() map[string]any {
	return map[string]any{"legal_name": "Example Client", "client_type_id": "00112233-4455-4677-8899-aabbccddeeff", "industry_id": "00112233-4455-4677-8899-aabbccddeeff", "state_id": "00112233-4455-4677-8899-aabbccddeeff", "city_id": "00112233-4455-4677-8899-aabbccddeeff", "postal_code": "201001", "address_line_1": "Address", "pan": "AAACW4202F", "gstin": "27AAACW4202F1ZQ", "contacts": []any{map[string]any{"full_name": "Test Contact", "email": "CLIENT@EXAMPLE.COM", "phone": "+91 98765 43210"}}}
}
func TestClientStrictClientBoundary(t *testing.T) {
	p := clientPayload()
	p["website"] = "example.com"
	data, _ := masterDTO.MarshalMasterJSON(p)
	in, _, e := clientParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(string(data))))
	if e != nil || in.Website != "https://example.com" || in.Contacts[0].Email != "client@example.com" {
		t.Fatal(in, e)
	}
	for _, field := range []string{"Legal_name", "extra", "legacy_source"} {
		p := clientPayload()
		p[field] = "bad"
		data, _ := masterDTO.MarshalMasterJSON(p)
		if _, _, e := clientParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(string(data)))); e == nil {
			t.Fatal("unknown accepted", field)
		}
	}
	for _, field := range []string{"pan", "gstin", "postal_code", "state_id", "contacts", "address_line_1"} {
		p := clientPayload()
		delete(p, field)
		data, _ := masterDTO.MarshalMasterJSON(p)
		if _, _, e := clientParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(string(data)))); e == nil {
			t.Fatal("missing accepted", field)
		}
	}
}
func TestClientBodyLimitAndInvalidFormats(t *testing.T) {
	if _, _, e := clientParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", 131073)))); e == nil {
		t.Fatal("oversize accepted")
	}
	for _, url := range []string{"javascript:alert(1)", "https://", "https://name:pass@example.com", "https://example.com:99999"} {
		p := clientPayload()
		p["website"] = url
		data, _ := masterDTO.MarshalJSON(p)
		if _, _, err := clientParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(string(data)))); err == nil {
			t.Fatal("unsafe website", url)
		}
	}
	for _, p := range []string{"true"} {
		if b, ok := masterDTO.JsonBoolean(json.RawMessage(p)); !ok || !b {
			t.Fatal("JSON boolean")
		}
	}
}

func TestClientTimestampPrecision(t *testing.T) {
	value := time.Date(2026, 10, 9, 8, 0, 0, 123000000, time.UTC)
	if masterDTO.ClientTimestamp(value) != "2026-10-09T08:00:00.123Z" {
		t.Fatal("fraction precision")
	}
	if masterDTO.ClientTimestamp(value.Truncate(time.Second)) != "2026-10-09T08:00:00Z" {
		t.Fatal("whole second precision")
	}
}

func TestClientContactTypesBoundary(t *testing.T) {
	for _, kind := range []string{"", "EIC", "HOD", "SITE_CONTACT", "UNKNOWN"} {
		p := clientPayload()
		p["contacts"].([]any)[0].(map[string]any)["contact_type"] = kind
		data, _ := masterDTO.MarshalMasterJSON(p)
		in, _, err := clientParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(string(data))))
		if kind == "UNKNOWN" {
			if err == nil {
				t.Fatal("unknown type accepted")
			}
			continue
		}
		if err != nil || in.Contacts[0].Type != kind || in.Contacts[0].Department != "" {
			t.Fatal("type or optional department rejected", err)
		}
	}
	p := clientPayload()
	p["contacts"] = []any{}
	data, _ := masterDTO.MarshalMasterJSON(p)
	if _, _, e := clientParse(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(string(data)))); e != nil {
		t.Fatal("empty contacts rejected", e)
	}
}
