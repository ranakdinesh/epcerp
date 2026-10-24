package infrastructure

import (
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/config"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"net/http"
)

// Router owns transport middleware; capability routes and guards are composed externally.
type Router struct {
	chi.Router
	payloadPolicies []payloadPolicy
}

type payloadPolicy struct {
	matches func(*http.Request) bool
	limit   int64
	guard   func(http.Handler) http.Handler
}

func NewRouter(configuration config.Config, logger logging.Logger) *Router {
	router := &Router{Router: chi.NewRouter()}
	router.Use(httpmiddleware.Requests(logger))
	router.Use(httpmiddleware.Headers)
	router.Use(httpmiddleware.TrustedHosts(configuration.AllowedHosts))
	router.Use(httpmiddleware.RateLimit(httpmiddleware.NewLimiter(configuration.RateLimitRequests, configuration.RateLimitWindow)))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, policy := range router.payloadPolicies {
				if policy.matches(r) {
					handler := httpmiddleware.PayloadLimit(policy.limit)(next)
					if policy.guard != nil {
						handler = policy.guard(handler)
					}
					handler.ServeHTTP(w, r)
					return
				}
			}
			httpmiddleware.PayloadLimit(configuration.MaxRequestBodyBytes)(next).ServeHTTP(w, r)
		})
	})
	router.Use(httpmiddleware.HTTPS(configuration.ForceHTTPS))
	router.Use(httpmiddleware.CORS(configuration.CORSOrigins))
	return router
}

// AddPayloadPolicy must be called before serving. Guards execute before buffering
// capability upload bodies; a nil guard uses the normal route authorization.
func (r *Router) AddPayloadPolicy(matches func(*http.Request) bool, limit int64, guard func(http.Handler) http.Handler) {
	r.payloadPolicies = append(r.payloadPolicies, payloadPolicy{matches: matches, limit: limit, guard: guard})
}
