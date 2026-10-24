package httpmiddleware

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"net/http"
	"regexp"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func requestID(candidate string) string {
	if len(candidate) == 36 && uuidPattern.MatchString(candidate) {
		return candidate
	}
	var value [16]byte
	_, _ = rand.Read(value[:])
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}

func safeMethod(method string) string {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
		return method
	default:
		return "OTHER"
	}
}

// Requests correlates adapter logs and emits only allowlisted HTTP metadata.
// Use on the chi router before registering routes. Route templates replace raw
// paths so identifiers, personal data and unknown-path secrets are never logged.
func Requests(logger logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			id := requestID(r.Header.Get("X-Request-ID"))
			requestLogger := logger.With(logging.String("request_id", id))
			r = r.WithContext(logging.WithContext(r.Context(), requestLogger))
			w.Header().Set("X-Request-ID", id)
			response := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			panicked := false
			defer func() {
				status := response.Status()
				if status == 0 {
					status = http.StatusOK
					if panicked {
						status = http.StatusInternalServerError
					}
				}
				route := "<unmatched>"
				if routing := chi.RouteContext(r.Context()); routing != nil && routing.RoutePattern() != "" {
					route = routing.RoutePattern()
				}
				level := logging.InfoLevel
				if status >= 500 || panicked {
					level = logging.ErrorLevel
				} else if status >= 400 {
					level = logging.WarnLevel
				}
				requestLogger.WithLevel(level).Str("event", "http.request.completed").
					Str("method", safeMethod(r.Method)).Str("route", route).
					Int("status_code", status).Float64("duration_ms", float64(time.Since(started))/float64(time.Millisecond)).
					Bool("panicked", panicked).Msg("HTTP request completed")
			}()
			defer func() {
				if value := recover(); value != nil {
					panicked = true
					// Preserve net/http abort behavior, including partially sent responses.
					if value == http.ErrAbortHandler || response.Status() != 0 {
						panic(http.ErrAbortHandler)
					}
					response.Header().Set("Content-Type", "application/json")
					response.Header().Set("Cache-Control", "no-store")
					response.Header().Set("X-Content-Type-Options", "nosniff")
					response.WriteHeader(http.StatusInternalServerError)
					_, _ = response.Write([]byte("{\"detail\":\"Internal server error\"}\n"))
				}
			}()
			next.ServeHTTP(response, r)
		})
	}
}
