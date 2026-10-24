package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func TestVendorDocumentStrictImportEnvelope(t *testing.T) {
	for _, body := range []string{`{"unknown":true}`, `{} {}`, strings.Repeat("x", 524289)} {
		w := httptest.NewRecorder()
		var in masterDomain.VendorDocumentInput
		if vendorDocumentDecode(w, httptest.NewRequest("POST", "/", strings.NewReader(body)), &in) {
			t.Fatal("invalid body accepted")
		}
	}
}
