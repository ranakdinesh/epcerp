package httpx

import (
	"testing"
)

func TestVendorDocumentLargerUploadLimitIsScoped(t *testing.T) {
	for _, s := range []struct {
		path string
		want bool
	}{{"/api/v1/suppliers/11111111-1111-4111-8111-111111111111/documents", true}, {"/api/v1/suppliers/11111111-1111-4111-8111-111111111111/documents/legacy-import", false}, {"/api/v1/suppliers/no/documents", false}, {"/api/v1/suppliers", false}, {"/api/v1/suppliers/11111111-1111-4111-8111-111111111111/documents/", false}} {
		if VendorDocumentIsUploadPath(s.path) != s.want {
			t.Fatal("bad scope", s.path)
		}
	}
}
