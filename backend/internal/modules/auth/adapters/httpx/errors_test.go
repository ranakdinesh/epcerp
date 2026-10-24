package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func TestDomainErrorKindsPreserveHTTPContract(t *testing.T) {
	for _, test := range []struct {
		err    *domain.Error
		status int
	}{
		{domain.ErrUnauthorized, http.StatusUnauthorized},
		{domain.ErrForbidden, http.StatusForbidden},
		{domain.ErrUserMissing, http.StatusNotFound},
		{domain.ErrRoleMissing, http.StatusNotFound},
		{domain.ErrUserConflict, http.StatusConflict},
		{domain.ErrUnknownPermission, http.StatusUnprocessableEntity},
		{domain.ErrLastSuperadmin, http.StatusUnprocessableEntity},
		{&domain.Error{Kind: "unknown", Detail: "operation failed"}, http.StatusInternalServerError},
	} {
		response := httptest.NewRecorder()
		if !failed(response, httptest.NewRequest(http.MethodGet, "/", nil), test.err) || response.Code != test.status {
			t.Fatalf("%s: got %d want %d", test.err.Kind, response.Code, test.status)
		}
		if test.status == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatal("missing authentication challenge")
		}
	}
}
