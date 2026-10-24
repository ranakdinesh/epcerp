package inbound

import (
	"encoding/json"
	"testing"
)

func TestPurchaseEnabledRequiresBooleanDecision(t *testing.T) {
	for _, raw := range []string{`{"purchase_enabled":null}`, `{"purchase_enabled":"true"}`, `{"purchase_enabled":1}`} {
		var request CaptureRequest
		if json.Unmarshal([]byte(raw), &request) == nil {
			t.Fatal("invalid purchase enablement accepted")
		}
	}
	for _, raw := range []string{`{"purchase_enabled":true}`, `{"purchase_enabled":false}`} {
		var request CaptureRequest
		if e := json.Unmarshal([]byte(raw), &request); e != nil {
			t.Fatal(e)
		}
	}
}
