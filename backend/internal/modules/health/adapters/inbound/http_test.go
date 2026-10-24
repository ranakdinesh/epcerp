package inbound

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/health/application"
	"net/http/httptest"
	"strings"
	"testing"
)

type probe struct {
	err      error
	deadline bool
}

func (p *probe) Check(ctx context.Context) error { _, p.deadline = ctx.Deadline(); return p.err }

func TestHealthContracts(t *testing.T) {
	for _, test := range []struct {
		path string
		err  error
		code int
		body string
	}{
		{"live", nil, 200, `{"status":"ok"}`},
		{"ready", nil, 200, `{"status":"ok"}`},
		{"ready", errors.New("password=private"), 503, `{"status":"unavailable"}`},
	} {
		p := &probe{err: test.err}
		mux := chi.NewRouter()
		Register(mux, application.New(p))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/health/"+test.path, nil))
		if response.Code != test.code || strings.TrimSpace(response.Body.String()) != test.body {
			t.Fatalf("got %d %s", response.Code, response.Body.String())
		}
		if response.Header().Get("Content-Type") != "application/json" {
			t.Fatal("missing content type")
		}
		if test.path == "ready" && !p.deadline {
			t.Fatal("readiness query has no deadline")
		}
	}
}

func TestMethodsAndUnknownCapabilities(t *testing.T) {
	mux := chi.NewRouter()
	Register(mux, application.New(&probe{}))
	for _, test := range []struct {
		method, path string
		code         int
	}{
		{"POST", "/api/v1/health/live", 405}, {"POST", "/api/v1/health/ready", 405},
		{"GET", "/api/v1/projects", 404},
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.code {
			t.Fatalf("%s %s: got %d", test.method, test.path, response.Code)
		}
	}
}
