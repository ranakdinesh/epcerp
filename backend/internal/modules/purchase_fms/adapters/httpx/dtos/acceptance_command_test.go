package dtos

import (
	"strings"
	"testing"
)

func TestAcceptanceDecodeAcceptanceCommand(t *testing.T) {
	good := `{"version":0,"action":"record","reason":"Vendor response","response":{"status":"accepted","accepted_date":"2026-10-11","document_reference":"Email ref","remarks":"Confirmed"}}`
	if q, e := AcceptanceDecode(strings.NewReader(good)); e != nil || q.Response.Status != "accepted" {
		t.Fatal(e)
	}
	for _, s := range []string{`null`, `[]`, good + good, `{"version":0,"version":1,"action":"record","reason":"Vendor response","response":{"status":"pending"}}`, `{"version":0,"action":"record","reason":"Vendor response","response":null}`, strings.Replace(good, `"version":0,`, "", 1), strings.Replace(good, `"response":{`, `"supplier_id":"forged","response":{`, 1), strings.Replace(good, `"status":"accepted"`, `"status":"accepted","total":"0.00"`, 1), strings.Repeat(" ", 65537)} {
		if _, e := AcceptanceDecode(strings.NewReader(s)); e == nil {
			t.Fatal("accepted undeclared, ambiguous or malformed command")
		}
	}
}
