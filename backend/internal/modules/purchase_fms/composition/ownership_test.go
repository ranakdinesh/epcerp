package composition

import (
	"github.com/go-chi/chi/v5"
	accounts "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance"
	items "github.com/ranakdinesh/epcERP/internal/modules/project_items"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeOwnershipRetiresPurchaseRoutesAndUploadGuards(t *testing.T) {
	t.Setenv("EPCERP_PURCHASING_OWNER", "nextjs")
	t.Setenv("EPCERP_SUPPLIER_DOCUMENT_KEY", "3333333333333333333333333333333333333333333333333333333333333333")
	t.Setenv("VTPL_JWT_SECRET_KEY", "synthetic-purchasing-native-ownership-test-secret")
	t.Setenv("VTPL_JWT_ISSUER", "epcERP")
	t.Setenv("VTPL_JWT_AUDIENCE", "epcERP-api")
	t.Setenv("VTPL_STORAGE_SECRET_KEY", "synthetic-purchase-storage-secret")
	t.Setenv("VTPL_STORAGE_ACCESS_KEY", "synthetic-purchase-storage")
	t.Setenv("VTPL_STORAGE_ENDPOINT", "127.0.0.1:1")
	t.Setenv("VTPL_STORAGE_BUCKET", "synthetic-purchasing")
	router := chi.NewRouter()
	result, err := New(router, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	items.New(items.Dependencies{Router: router})
	accounts.New(accounts.Dependencies{Router: router})
	for _, path := range []string{"/api/v1/purchase-fms", "/api/v1/projects/00000000-0000-4000-8000-000000000000/items", "/api/v1/projects/00000000-0000-4000-8000-000000000000/purchase-workflow/00000000-0000-4000-8000-000000000001/accounts-acceptance", "/api/v1/quotation-approvals", "/api/v1/purchase-advances", "/api/v1/indent-dispatch-notifications", "/api/v1/projects/00000000-0000-4000-8000-000000000000/purchase-workflow/00000000-0000-4000-8000-000000000001/technical/uploads/document/1"} {
		w := httptest.NewRecorder()
		result.UploadGuard(router).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 404 {
			t.Fatalf("%s retained Go ownership: %d", path, w.Code)
		}
	}
	if result.TaskProviders.Workflow == nil || result.ProjectItems == nil || result.Accounts == nil {
		t.Fatal("Native HTTP ownership must preserve remaining consumers' transaction-bound readers")
	}
}
func TestInvalidPurchaseOwnerFailsBeforeConstruction(t *testing.T) {
	t.Setenv("EPCERP_PURCHASING_OWNER", "invalid")
	if _, err := New(chi.NewRouter(), nil, nil); err == nil {
		t.Fatal("invalid purchasing owner accepted")
	}
}
