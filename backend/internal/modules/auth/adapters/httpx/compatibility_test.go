package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCompatibilityDoesNotExposeMigratedAuthOperations(t *testing.T) {
	router := chi.NewRouter()
	RegisterCompatibility(router, Services{})
	for _, route := range []struct{ method, path string }{
		{"POST", "/api/v1/auth/login"}, {"POST", "/api/v1/auth/refresh"},
		{"POST", "/api/v1/auth/logout"}, {"GET", "/api/v1/auth/me"},
		{"POST", "/api/v1/auth/me/password"}, {"GET", "/api/v1/auth/me/sessions"},
		{"POST", "/api/v1/auth/forgot-password"}, {"POST", "/api/v1/auth/reset-password"},
		{"POST", "/api/v1/auth/verify-email"}, {"GET", "/api/v1/users"},
		{"POST", "/api/v1/users"}, {"GET", "/api/v1/roles"}, {"POST", "/api/v1/roles"},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
			if w.Code != http.StatusNotFound {
				t.Fatalf("migrated operation still exposed: %d", w.Code)
			}
		})
	}
}
