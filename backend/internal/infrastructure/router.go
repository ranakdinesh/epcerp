package infrastructure

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/config"
	"github.com/ranakdinesh/epcERP/internal/identity"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"net/http"
	"strings"
)

// Router owns transport middleware; capability routes and guards are composed externally.
type RateIdentity interface {
	Resolve(context.Context, string) (identity.Principal, error)
}

type Router struct {
	RateIdentity RateIdentity
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
	router.Use(httpmiddleware.RateLimitByIdentity(httpmiddleware.NewLimiter(configuration.RateLimitRequests, configuration.RateLimitWindow), func(r *http.Request) string {
		if router.RateIdentity == nil {
			return ""
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return ""
		}
		p, e := router.RateIdentity.Resolve(r.Context(), parts[1])
		if e != nil || !p.Active {
			return ""
		}
		return p.ID
	}))
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
