package httpmiddleware

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func invoke(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
func ok(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }

func TestTrustedHostRulesAndHTTPS(t *testing.T) {
	handler := Headers(TrustedHosts([]string{"localhost", "*.example.com", "::1"})(HTTPS(true)(http.HandlerFunc(ok))))
	for _, test := range []struct {
		host string
		tls  bool
		code int
	}{
		{"localhost:8001", false, 307}, {"localhost", true, 204}, {"api.example.com", true, 204}, {"deep.api.example.com", true, 204},
		{"example.com", true, 400}, {"badexample.com", true, 400}, {"api.example.com.attacker.com", true, 400}, {"bad-host", true, 400},
		{"localhost@attacker.com", true, 400}, {"localhost:0", true, 400}, {"localhost,attacker.com", true, 400}, {"[::1]:8001", true, 204},
	} {
		request := httptest.NewRequest("GET", "http://localhost/api/v1/health/live?value=kept", nil)
		request.Host = test.host
		request.Header.Set("X-Forwarded-Proto", "https")
		request.Header.Set("X-Forwarded-Host", "localhost")
		if test.tls {
			request.TLS = &tls.ConnectionState{}
		}
		response := invoke(handler, request)
		if response.Code != test.code {
			t.Fatalf("host %s TLS %v: got %d", test.host, test.tls, response.Code)
		}
		if response.Header().Get("X-Frame-Options") != "DENY" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing denial headers")
		}
		if test.code == 307 && response.Header().Get("Location") != "https://localhost:8001/api/v1/health/live?value=kept" {
			t.Fatal("redirect lost request target")
		}
	}
}
func TestCORSAllowsOnlyConfiguredOriginMethodsAndHeaders(t *testing.T) {
	handler := CORS([]string{"http://localhost:3000"})(http.HandlerFunc(ok))
	for _, test := range []struct {
		origin, method, headers string
		code                    int
		allow                   bool
	}{
		{"http://localhost:3000", "POST", "Authorization,Content-Type,X-Request-ID", 200, true},
		{"http://attacker.com", "POST", "", 400, false}, {"http://localhost:3000", "TRACE", "", 400, false},
		{"http://localhost:3000", "POST", "X-Unapproved", 400, false},
		{"http://localhost:3000", "", "", 204, true}, {"http://attacker.com", "", "", 204, false},
	} {
		method := "GET"
		if test.method != "" {
			method = "OPTIONS"
		}
		request := httptest.NewRequest(method, "/", nil)
		request.Header.Set("Origin", test.origin)
		request.Header.Set("Access-Control-Request-Method", test.method)
		request.Header.Set("Access-Control-Request-Headers", test.headers)
		response := invoke(handler, request)
		if response.Code != test.code {
			t.Fatalf("CORS: got %d want %d", response.Code, test.code)
		}
		if (response.Header().Get("Access-Control-Allow-Origin") != "") != test.allow {
			t.Fatal("origin leaked or missing")
		}
		if test.allow && response.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Fatal("missing credentials policy")
		}
		if response.Header().Get("Vary") == "" {
			t.Fatal("missing cache origin variance")
		}
	}
}
func TestPayloadGateRejectsDeclaredAndStreamedBodiesBeforeWork(t *testing.T) {
	for _, test := range []struct {
		body, declared string
		unknown        bool
		code           int
	}{
		{"1234", "", false, 204}, {"12345", "", false, 413}, {"12345", "", true, 413}, {"1234", "", true, 204},
		{"", "5", false, 413}, {"", "invalid", false, 400}, {"", "-1", false, 400}, {"12345", "1", true, 413},
	} {
		called := false
		handler := PayloadLimit(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			body, err := io.ReadAll(r.Body)
			if err != nil || string(body) != test.body {
				t.Fatal("accepted body changed")
			}
			w.WriteHeader(204)
		}))
		request := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
		if test.unknown {
			request.ContentLength = -1
		}
		if test.declared != "" {
			request.Header.Set("Content-Length", test.declared)
		}
		response := invoke(handler, request)
		if response.Code != test.code || called != (test.code == 204) {
			t.Fatalf("payload gate: got %d, called %v", response.Code, called)
		}
	}
}
func TestLimiterSlidingWindowCapacityAndConcurrency(t *testing.T) {
	limiter := NewLimiter(2, time.Minute)
	now := time.Unix(1000, 0)
	limiter.clock = func() time.Time { return now }
	limiter.capacity = 2
	if !limiter.Allow("a") || !limiter.Allow("a") || limiter.Allow("a") || !limiter.Allow("b") || limiter.Allow("c") {
		t.Fatal("limits or capacity incorrect")
	}
	now = now.Add(time.Minute)
	if !limiter.Allow("c") || !limiter.Allow("a") {
		t.Fatal("expired windows were not reclaimed")
	}
	concurrent := NewLimiter(10, time.Minute)
	var count atomic.Int32
	var group sync.WaitGroup
	for range 100 {
		group.Add(1)
		go func() {
			defer group.Done()
			if concurrent.Allow("one") {
				count.Add(1)
			}
		}()
	}
	group.Wait()
	if count.Load() != 10 {
		t.Fatal("concurrent limit bypass")
	}
}
func TestRateMiddlewareHealthBypassAndForwardedAddressSpoofing(t *testing.T) {
	handler := RateLimit(NewLimiter(1, time.Minute))(http.HandlerFunc(ok))
	request := func(path, remote, forwarded string) *http.Request {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = remote
		r.Header.Set("X-Forwarded-For", forwarded)
		return r
	}
	if invoke(handler, request("/api/v1/projects", "127.0.0.1:1", "first")).Code != 204 {
		t.Fatal("first request denied")
	}
	response := invoke(handler, request("/api/v1/projects", "127.0.0.1:2", "second"))
	if response.Code != 429 || response.Header().Get("Retry-After") != "60" {
		t.Fatal("forwarded address bypassed limit")
	}
	for range 3 {
		if invoke(handler, request("/api/v1/health/live", "127.0.0.1:3", "")).Code != 204 {
			t.Fatal("health incorrectly throttled")
		}
	}
	if invoke(handler, request("/api/v1/projects", "127.0.0.2:1", "")).Code != 204 {
		t.Fatal("clients not isolated")
	}
}
