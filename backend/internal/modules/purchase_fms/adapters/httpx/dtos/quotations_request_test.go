package dtos

import (
	"encoding/json"
	"testing"
)

func TestQuotationsQuotationRequestDefaultsAndRequiredFields(t *testing.T) {
	var req QuotationsCommandRequest
	if e := json.Unmarshal([]byte(`{"version":0,"reason":"Receive","quote":{"delivery_days":0,"lines":[{"line_id":"fixture","unit_price":999999999999.9999}]}}`), &req); e != nil {
		t.Fatal(e)
	}
	cmd, e := req.Command()
	if e != nil {
		t.Fatal(e)
	}
	if cmd.Quote.Freight != "0" || cmd.Quote.OtherCharges != "0" || cmd.Quote.Lines[0].Discount != "0" || cmd.Quote.Lines[0].Tax != "0" || *cmd.Quote.Lines[0].UnitPrice != "999999999999.9999" {
		t.Fatal("request defaults or decimal precision lost")
	}
	req.Version = nil
	if _, e = req.Command(); e == nil {
		t.Fatal("missing version accepted")
	}
	for _, raw := range []string{`{"quote":{"freight":null}}`, `{"version":null}`, `{"quote":{"lines":[{"compliant":null}]}}`, `{"quote":{"lines":[null]}}`, `[]`, `null`} {
		if e = QuotationsNonNull([]byte(raw)); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if e = QuotationsNonNull([]byte(`{"quote":{"attachment_id":null,"total_amount":null,"lines":[{"unit_price":null,"line_total":null,"warranty_months":null}]}}`)); e != nil {
		t.Fatal(e)
	}
}
