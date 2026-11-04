package httpx

import (
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"net/http"
	"strings"
	"testing"
)

func TestNativeDocumentOwnershipRetainsFinanceReconciliation(t *testing.T) {
	r := &infrastructure.Router{Router: chi.NewRouter()}
	Register(r, Services{ClientsHTTPDisabled: true, ReferencesHTTPDisabled: true, CatalogueHTTPDisabled: true, VendorProfilesHTTPDisabled: true, VendorDocumentsHTTPDisabled: true})
	found := false
	err := chi.Walk(r.Router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.Contains(path, "/documents") {
			if method != "POST" || path != "/api/v1/suppliers/{supplier_id}/documents/reconcile-approvals" {
				t.Errorf("retired document route remains: %s %s", method, path)
			} else {
				found = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Finance reconciliation route missing")
	}
}

func TestNativeFinanceOwnershipRetiresDecisionsAssignmentsAndReconciliation(t *testing.T) {
	r := &infrastructure.Router{Router: chi.NewRouter()}
	Register(r, Services{ClientsHTTPDisabled: true, ReferencesHTTPDisabled: true, CatalogueHTTPDisabled: true, VendorProfilesHTTPDisabled: true, VendorDocumentsHTTPDisabled: true, VendorVerificationHTTPDisabled: true})
	if err := chi.Walk(r.Router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.Contains(path, "/verification") || strings.Contains(path, "reconcile-approvals") {
			t.Errorf("retired Finance route remains: %s %s", method, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
