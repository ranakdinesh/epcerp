package httpmiddleware_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func events(t *testing.T, buffer *bytes.Buffer) []map[string]any {
	t.Helper()
	var result []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buffer.String()), "\n") {
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid JSON log: %v", err)
		}
		if _, err := time.Parse(time.RFC3339Nano, event["timestamp"].(string)); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(event["timestamp"].(string), "Z") {
			t.Fatal("timestamp must be UTC")
		}
		if event["service"] != "epcERP" {
			t.Fatal("missing service identity")
		}
		result = append(result, event)
	}
	return result
}

func TestLevelConfiguration(t *testing.T) {
	for _, level := range []string{"", "TRACE", "debug", " INFO ", "warn", "error"} {
		if _, err := infrastructure.NewLoggerWithWriter(&bytes.Buffer{}, level); err != nil {
			t.Fatalf("valid level %q rejected", level)
		}
	}
	for _, level := range []string{"invalid", "fatal", "panic", "disabled"} {
		if _, err := infrastructure.NewLoggerWithWriter(&bytes.Buffer{}, level); err == nil {
			t.Fatalf("invalid level %q accepted", level)
		}
	}
	var output bytes.Buffer
	logger, _ := infrastructure.NewLoggerWithWriter(&output, "WARN")
	logger.Info().Msg("filtered")
	logger.Warn().Str("event", "test.warning").Msg("included")
	if got := events(t, &output); len(got) != 1 || got[0]["level"] != "warn" {
		t.Fatal("level filtering failed")
	}
}

func TestRequestCorrelationAndSafeMetadata(t *testing.T) {
	var output bytes.Buffer
	logger, _ := infrastructure.NewLoggerWithWriter(&output, "info")
	router := chi.NewRouter()
	router.Use(httpmiddleware.Requests(logger))
	router.Get("/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		logging.Ctx(r.Context()).Info().Str("event", "adapter.completed").Msg("Adapter completed")
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest("GET", "/projects/private-name?token=query-secret", strings.NewReader("body-secret"))
	request.Header.Set("Authorization", "Bearer auth-secret")
	request.Header.Set("Cookie", "session=cookie-secret")
	id := "12345678-1234-4234-8234-123456789abc"
	request.Header.Set("X-Request-ID", id)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 204 || response.Header().Get("X-Request-ID") != id {
		t.Fatal("correlation changed response")
	}
	got := events(t, &output)
	if len(got) != 2 {
		t.Fatalf("expected adapter and HTTP events, got %d", len(got))
	}
	for _, event := range got {
		if event["request_id"] != id {
			t.Fatal("lost context correlation")
		}
	}
	event := got[1]
	if event["route"] != "/projects/{id}" || event["status_code"] != float64(204) || event["duration_ms"].(float64) < 0 {
		t.Fatal("incorrect completion metadata")
	}
	for _, secret := range []string{"private-name", "query-secret", "body-secret", "auth-secret", "cookie-secret"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("logged forbidden request data: %s", secret)
		}
	}
}

func TestUnknownRoutesAndUnsafeIDs(t *testing.T) {
	for _, id := range []string{"", "token-secret", "bad\nheader", strings.Repeat("x", 500)} {
		var output bytes.Buffer
		logger, _ := infrastructure.NewLoggerWithWriter(&output, "info")
		router := chi.NewRouter()
		router.Use(httpmiddleware.Requests(logger))
		router.Get("/known", func(http.ResponseWriter, *http.Request) {})
		request := httptest.NewRequest("GET", "/secret-path?password=private", nil)
		request.Header.Set("X-Request-ID", id)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if !regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`).MatchString(response.Header().Get("X-Request-ID")) {
			t.Fatal("missing valid generated ID")
		}
		got := events(t, &output)
		if len(got) != 1 || got[0]["route"] != "<unmatched>" || got[0]["level"] != "warn" {
			t.Fatal("unknown-route logging failed")
		}
		if strings.Contains(output.String(), "secret-path") || strings.Contains(output.String(), "token-secret") {
			t.Fatal("unsafe metadata leaked")
		}
	}
}

func TestPanicAndServerDiagnosticsAreSanitized(t *testing.T) {
	var output bytes.Buffer
	logger, _ := infrastructure.NewLoggerWithWriter(&output, "info")
	router := chi.NewRouter()
	router.Use(httpmiddleware.Requests(logger))
	router.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("password=panic-secret") })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/panic", nil))
	if response.Code != 500 || strings.Contains(response.Body.String(), "panic-secret") {
		t.Fatal("unsafe panic response")
	}
	infrastructure.ServerErrorLogger(logger).Print("database_url=driver-secret")
	got := events(t, &output)
	if len(got) != 2 || got[0]["level"] != "error" || got[0]["panicked"] != true || got[1]["event"] != "http.server.error" {
		t.Fatal("missing sanitized error events")
	}
	if strings.Contains(output.String(), "panic-secret") || strings.Contains(output.String(), "driver-secret") {
		t.Fatal("raw error details leaked")
	}
}

func TestAbortAndStreamingInterfaces(t *testing.T) {
	var output bytes.Buffer
	logger, _ := infrastructure.NewLoggerWithWriter(&output, "info")
	handler := httpmiddleware.Requests(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("lost streaming interface")
		}
		w.WriteHeader(200)
		panic("private partial response panic")
	}))
	func() {
		defer func() {
			if recovered := recover(); recovered != http.ErrAbortHandler {
				t.Fatal("partial response must abort")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/stream", nil))
	}()
	got := events(t, &output)
	if len(got) != 1 || got[0]["level"] != "error" {
		t.Fatal("missing aborted completion event")
	}
}

func TestConcurrentRequestLogsRemainJSONAndIsolated(t *testing.T) {
	var output bytes.Buffer
	logger, _ := infrastructure.NewLoggerWithWriter(&output, "info")
	router := chi.NewRouter()
	router.Use(httpmiddleware.Requests(logger))
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {})
	var group sync.WaitGroup
	for i := range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", fmt.Sprintf("/health?secret=%d", i), nil))
		}()
	}
	group.Wait()
	got := events(t, &output)
	seen := map[string]bool{}
	for _, event := range got {
		seen[event["request_id"].(string)] = true
	}
	if len(got) != 20 || len(seen) != 20 {
		t.Fatal("request logs collided or interleaved")
	}
}
