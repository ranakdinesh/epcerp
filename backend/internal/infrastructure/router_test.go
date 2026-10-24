package infrastructure

import (
	"bytes"
	"github.com/ranakdinesh/epcERP/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRouterPayloadPoliciesPreserveGuardsAndLimits(t *testing.T) {
	logger, err := NewLoggerWithWriter(&bytes.Buffer{}, "info")
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(config.Config{
		AllowedHosts: []string{"localhost"}, MaxRequestBodyBytes: 1024,
		RateLimitRequests: 120, RateLimitWindow: time.Minute,
	}, logger)
	router.AddPayloadPolicy(func(r *http.Request) bool { return r.URL.Path == "/upload" }, 4096, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Allow") != "true" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	for _, path := range []string{"/upload", "/regular"} {
		router.Post(path, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	}
	for _, test := range []struct {
		name, path string
		size       int
		allowed    bool
		status     int
	}{
		{"guard before buffering", "/upload", 5000, false, 401},
		{"capability limit", "/upload", 2048, true, 204},
		{"capability overflow", "/upload", 5000, true, 413},
		{"default overflow", "/regular", 2048, true, 413},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://localhost"+test.path, strings.NewReader(strings.Repeat("x", test.size)))
			if test.allowed {
				request.Header.Set("X-Test-Allow", "true")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("got status %d, want %d", response.Code, test.status)
			}
			if response.Header().Get("X-Request-ID") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("base middleware missing")
			}
		})
	}
}
