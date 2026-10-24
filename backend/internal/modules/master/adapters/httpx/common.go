package httpx

import (
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"

	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ranakdinesh/epcERP/internal/logging"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func bearerToken(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(masterDTO.TransportValue(value))
}

func httpStatus(kind domain.ErrorKind) int {
	switch kind {
	case domain.Unauthenticated:
		return http.StatusUnauthorized
	case domain.AccessDenied:
		return http.StatusForbidden
	case domain.NotFound:
		return http.StatusNotFound
	case domain.Conflict:
		return http.StatusConflict
	case domain.InvalidInput:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func failMaster(w http.ResponseWriter, r *http.Request, err error, event string) bool {
	if err == nil {
		return false
	}
	var failure *domain.Error
	if errors.As(err, &failure) {
		status := httpStatus(failure.Kind)
		if status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
		if failure.Field != "" {
			writeJSON(w, status, map[string]any{"detail": []map[string]any{{"loc": []string{"body", failure.Field}, "msg": failure.Detail, "type": "value_error"}}})
		} else {
			writeJSON(w, status, map[string]string{"detail": failure.Detail})
		}
	} else {
		logging.Ctx(r.Context()).Error().Str("event", event).Msg("Master operation failed")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
	}
	return true
}
