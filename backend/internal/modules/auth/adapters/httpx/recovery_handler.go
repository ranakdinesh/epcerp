package httpx

import (
	"errors"
	"net/http"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
)

type recoveryHandler struct{ service *application.RecoveryService }

func (h *recoveryHandler) request(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	request, ok := requestBody(w, r, dtos.ParseRecoveryRequest)
	if !ok {
		return
	}
	err := h.service.Request(r.Context(), request.Email)
	if errors.Is(err, application.ErrRecoveryUnavailable) {
		write(w, 503, dtos.RecoveryMessage{Message: "Email recovery is not configured yet. Contact your administrator."})
		return
	}
	if err != nil {
		// Keep both delivery and storage failures private so responses cannot enumerate users.
		audit(r.Context(), "auth.recovery.delivery_failed", true)
	}
	write(w, 202, dtos.RecoveryMessage{Message: "If an active account uses this email, password reset instructions will be sent. Check your inbox and spam folder."})
}
func (h *recoveryHandler) complete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	request, ok := requestBody(w, r, dtos.ParseRecoveryComplete)
	if !ok {
		return
	}
	err := h.service.Complete(r.Context(), request.Token, request.Password)
	audit(r.Context(), "auth.recovery.completed", err != nil)
	if !failed(w, r, err) {
		w.WriteHeader(204)
	}
}
func recoveryLimit() func(http.Handler) http.Handler {
	return httpmiddleware.RateLimit(httpmiddleware.NewLimiter(5, 15*time.Minute))
}
