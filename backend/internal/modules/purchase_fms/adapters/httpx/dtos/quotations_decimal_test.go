package dtos

import (
	"encoding/json"
	"testing"
)

func TestQuotationsDecimalPreservesPrecision(t *testing.T) {
	for _, raw := range []string{`999999999999.9999`, `"999999999999.9999"`} {
		var v QuotationsDecimal
		if e := json.Unmarshal([]byte(raw), &v); e != nil || v != "999999999999.9999" {
			t.Fatalf("precision lost: %s %v", v, e)
		}
	}
	for _, raw := range []string{`null`, `true`, `1e2`, `-1`, `"NaN"`, `"1.2.3"`} {
		var v QuotationsDecimal
		if e := json.Unmarshal([]byte(raw), &v); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
