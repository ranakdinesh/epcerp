package inbound

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/health/application"
	"github.com/ranakdinesh/epcERP/internal/modules/health/domain"
	"net/http"
	"time"
)

// Register exposes only the two existing, public health contracts.
func Register(mux chi.Router, service *application.Service) {
	mux.Get("/api/v1/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, service.Live())
	})
	mux.Get("/api/v1/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		writeStatus(w, service.Ready(ctx))
	})
}

func writeStatus(w http.ResponseWriter, status domain.Status) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if status == domain.Unavailable {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(struct {
		Status domain.Status `json:"status"`
	}{status})
}
