package httpmiddleware

import (
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func reject(w http.ResponseWriter, r *http.Request, code int, detail string) {
	event := "security.request.rejected"
	if code == http.StatusRequestEntityTooLarge {
		event = "security.payload.rejected"
	}
	if code == http.StatusTooManyRequests {
		event = "security.rate_limit.exceeded"
	}
	logging.Ctx(r.Context()).Warn().Str("event", event).Int("status_code", code).Msg("Request rejected by security policy")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(struct {
		Detail string `json:"detail"`
	}{detail})
}

func Headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func requestHost(raw string) (string, bool) {
	// Reject userinfo, multiple Host values, malformed ports and URL fragments.
	if raw == "" || strings.HasSuffix(raw, ":") || strings.ContainsAny(raw, " /\\\t\r\n,@?#") {
		return "", false
	}
	parsed, err := url.Parse("http://" + raw)
	if err != nil || parsed.Host != raw || parsed.User != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return "", false
	}
	if port := parsed.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", false
		}
	}
	return host, host != ""
}

func TrustedHosts(hosts []string) func(http.Handler) http.Handler {
	allowed := append([]string(nil), hosts...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, valid := requestHost(r.Host)
			if valid {
				for _, entry := range allowed {
					if host == entry || (strings.HasPrefix(entry, "*.") && strings.HasSuffix(host, entry[1:]) && len(host) > len(entry[1:])) {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			reject(w, r, http.StatusBadRequest, "Invalid host header")
		})
	}
}

// HTTPS trusts only an actual TLS connection, never caller-supplied proxy headers.
func HTTPS(force bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if force && r.TLS == nil {
				target := &url.URL{Scheme: "https", Host: r.Host, Path: r.URL.Path, RawPath: r.URL.RawPath, RawQuery: r.URL.RawQuery}
				http.Redirect(w, r, target.String(), http.StatusTemporaryRedirect)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
