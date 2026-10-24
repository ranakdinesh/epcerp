package httpmiddleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
)

// PayloadLimit checks both declared and streamed sizes BEFORE invoking a use case.
// Per-route larger limits must be added only with the migrated upload capability.
func PayloadLimit(maximum int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw := r.Header.Get("Content-Length"); raw != "" {
				length, err := strconv.ParseInt(raw, 10, 64)
				if err != nil || length < 0 {
					reject(w, r, 400, "Invalid Content-Length")
					return
				}
				if length > maximum {
					reject(w, r, 413, "Request body is too large")
					return
				}
			}
			if r.ContentLength > maximum {
				reject(w, r, 413, "Request body is too large")
				return
			}
			if r.Body != nil && r.Body != http.NoBody {
				body := http.MaxBytesReader(w, r.Body, maximum)
				content, err := io.ReadAll(body)
				_ = body.Close()
				if err != nil {
					var sizeError *http.MaxBytesError
					if errors.As(err, &sizeError) {
						reject(w, r, 413, "Request body is too large")
					} else {
						reject(w, r, 400, "Invalid request body")
					}
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(content))
			}
			next.ServeHTTP(w, r)
		})
	}
}
