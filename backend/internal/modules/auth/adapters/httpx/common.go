package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	"github.com/ranakdinesh/epcERP/helpers"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func write(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}
func failed(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	var denied *domain.Error
	if errors.As(err, &denied) {
		code := errorStatus(denied.Kind)
		if code == 401 {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
		write(w, code, map[string]string{"detail": denied.Detail})
	} else {
		logging.Ctx(r.Context()).Error().Str("event", "auth.operation.failed").Msg("Authentication operation failed")
		write(w, 500, map[string]string{"detail": "Internal server error"})
	}
	return true
}
func audit(ctx context.Context, event string, denied bool) {
	logger := logging.Ctx(ctx)
	if denied {
		logger.Warn().Str("event", event).Msg("Authentication decision")
	} else {
		logger.Info().Str("event", event).Msg("Authentication decision")
	}
}
func bearer(r *http.Request) string {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		return ""
	}
	return fields[1]
}
func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id, err := helpers.CanonicalID(chi.URLParam(r, name))
	if err != nil {
		write(w, 422, map[string]any{"detail": []dtos.Issue{{Type: "uuid_parsing", Loc: []string{"path", name}, Msg: "Input should be a valid UUID"}}})
		return "", false
	}
	return id, true
}
func errorStatus(kind domain.ErrorKind) int {
	switch kind {
	case domain.Unauthorized:
		return http.StatusUnauthorized
	case domain.Forbidden:
		return http.StatusForbidden
	case domain.Missing:
		return http.StatusNotFound
	case domain.Conflict:
		return http.StatusConflict
	case domain.Unavailable:
		return http.StatusServiceUnavailable
	case domain.Invalid:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
