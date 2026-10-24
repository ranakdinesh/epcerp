package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func TestVendorStrictImportEnvelope(t *testing.T) {
	for _, body := range []string{`{"unknown":true}`, `{} {}`, strings.Repeat("x", 131073)} {
		w := httptest.NewRecorder()
		var in masterDomain.VendorInput
		if vendorDecode(w, httptest.NewRequest("POST", "/", strings.NewReader(body)), &in) {
			t.Fatal("invalid body accepted")
		}
	}
}
