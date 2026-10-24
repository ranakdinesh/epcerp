package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ranakdinesh/epcERP/internal/logging"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func bearer(r *http.Request) string {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
		return fields[1]
	}
	return ""
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, r *http.Request, err error, event, message string) bool {
	if err == nil {
		return false
	}
	var rule *domain.Error
	if errors.As(err, &rule) {
		if rule.Status == 401 {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
		if rule.Field != "" {
			write(w, rule.Status, map[string]any{"detail": []map[string]any{{"loc": []string{"body", rule.Field}, "msg": rule.Message, "type": "value_error"}}})
		} else {
			write(w, rule.Status, map[string]string{"detail": rule.Message})
		}
	} else {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			logging.Ctx(r.Context()).Error().Str("event", event).Msg(message)
			write(w, 500, map[string]string{"detail": "Internal server error"})
		}
	}
	return true
}
