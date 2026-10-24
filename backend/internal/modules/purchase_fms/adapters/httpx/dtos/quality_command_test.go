package dtos

import (
	"strings"
	"testing"
)

func TestQualityQualityStrictCommands(t *testing.T) {
	valid := []string{`{"version":0,"action":"record","reason":"Actual certificate","certificate_code":"TC","details":{"cert_number":"TC-1","issued_by":"Lab","valid_until":"2026-10-11","file_name":"ref.pdf"}}`, `{"version":1,"action":"verify","reason":"Checked","certificate_code":"TC"}`, `{"version":2,"action":"revoke","reason":"Correct attestation","certificate_code":"TC"}`, `{"version":2,"action":"revise","reason":"New report"}`}
	for _, v := range valid {
		if _, e := QualityDecode(strings.NewReader(v)); e != nil {
			t.Fatal(e)
		}
	}
	invalid := []string{`{"version":0,"action":"record","reason":"x","certificate_code":"TC","details":{"verified":true}}`, `{"version":1,"action":"verify","reason":"x","certificate_code":"TC","details":{}}`, `{"version":1,"action":"verify","reason":"x","certificate_code":"TC","verified_by_id":"forged"}`, `{"version":1,"action":"verify","reason":"x","certificate_code":"TC","certificate_code":"TR"}`, `{"version":1,"action":"record","reason":"x","certificate_code":"TC","details":{"issued_by":null}}`, `{"version":1,"action":"revise","reason":"x","certificate_code":""}`, `{"version":1,"action":"verify","reason":"x","certificate_code":"TC","details":[]}`, `{"action":"verify","reason":"x","certificate_code":"TC"}`}
	for _, v := range invalid {
		if _, e := QualityDecode(strings.NewReader(v)); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
