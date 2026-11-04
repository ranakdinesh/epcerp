package inbound

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNextTSCWriterDoesNotRegisterLegacyPaidCommands(t *testing.T) {
	r := chi.NewRouter()
	RegisterTSC(r, nil, true)
	for _, path := range []string{"/api/v1/tender-fms/drafts/11111111-1111-4111-8111-111111111111/tsc", "/api/v1/tender-fms/drafts/11111111-1111-4111-8111-111111111111/tsc/22222222-2222-4222-8222-222222222222/answers/one/research"} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
		if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("legacy model command registered: %s (%d)", path, response.Code)
		}
	}
}
