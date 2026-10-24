package httpmiddleware

import (
	"net/http"
	"strings"
)

var methods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true}
var requestHeaders = map[string]bool{"authorization": true, "content-type": true, "x-request-id": true, "accept": true, "accept-language": true, "content-language": true}

const allowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
const allowedHeaders = "Accept, Accept-Language, Authorization, Content-Language, Content-Type, X-Request-ID"

func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range origins {
		allowed[origin] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Add("Vary", "Origin")
			preflight := r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != ""
			if preflight {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
				if !allowed[origin] {
					reject(w, r, 400, "Disallowed CORS origin")
					return
				}
				if !methods[r.Header.Get("Access-Control-Request-Method")] {
					reject(w, r, 400, "Disallowed CORS method")
					return
				}
				for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
					header = strings.TrimSpace(strings.ToLower(header))
					if header != "" && !requestHeaders[header] {
						reject(w, r, 400, "Disallowed CORS headers")
						return
					}
				}
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if preflight {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = w.Write([]byte("OK"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
