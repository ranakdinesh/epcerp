package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type emptyRecoveryRepository struct{ ports.RecoveryRepository }

func (emptyRecoveryRepository) FindEmail(context.Context, string) (*domain.User, error) {
	return nil, nil
}

type recoveryUnit struct{}

func (recoveryUnit) Within(ctx context.Context, f func(context.Context, ports.RecoveryRepository) error) error {
	return f(ctx, emptyRecoveryRepository{})
}

type recoveryHTTPMail struct{ enabled bool }

func (m recoveryHTTPMail) Ready() bool                                      { return m.enabled }
func (recoveryHTTPMail) SendRecovery(context.Context, string, string) error { return nil }

type recoveryHTTPTokens struct{ ports.Tokens }

func (recoveryHTTPTokens) Refresh() (string, string) { return "raw", "hash" }
func recoveryRouter(enabled bool) http.Handler {
	router := chi.NewRouter()
	Register(router, Services{Recovery: application.NewRecovery(recoveryUnit{}, nil, recoveryHTTPTokens{}, recoveryHTTPMail{enabled})})
	return router
}
func TestRecoveryHTTPValidationAndRateLimit(t *testing.T) {
	router := recoveryRouter(true)
	request := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = "192.0.2.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	first := request(`{"email":"unknown@example.invalid"}`)
	if first.Code != 202 || first.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("generic response changed", first.Code)
	}
	if second := request(`{"email":"another@example.invalid"}`); second.Code != 202 || second.Body.String() != first.Body.String() {
		t.Fatal("existence response differs")
	}
	if request(`{"email":"invalid"}`).Code != 422 {
		t.Fatal("invalid email accepted")
	}
	if request(`{"email":"unknown@example.invalid","role":"admin"}`).Code != 422 {
		t.Fatal("extra fields accepted")
	}
	if request(`{"email":"unknown@example.invalid"}`).Code != 202 {
		t.Fatal("early rate limit")
	}
	if w := request(`{"email":"unknown@example.invalid"}`); w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("recovery flood allowed")
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(`{"email":"unknown@example.invalid"}`))
	w := httptest.NewRecorder()
	recoveryRouter(false).ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("disabled provider pretended to send", w.Code)
	}
}
