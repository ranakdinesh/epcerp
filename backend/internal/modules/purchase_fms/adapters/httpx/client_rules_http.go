package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type clientRulesHandler struct {
	s *application.ClientRulesService
}

func clientRulesFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "client_purchase_rules.operation.failed", "Client purchase policy operation failed")
}

func (h clientRulesHandler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), bearer(r), chi.URLParam(r, "client"))
	if !clientRulesFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h clientRulesHandler) history(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.History(r.Context(), bearer(r), chi.URLParam(r, "client"), chi.URLParam(r, "id"))
	if !clientRulesFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h clientRulesHandler) save(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			clientRulesFail(w, r, domain.ClientRulesInvalid("", "Unable to read input"))
		}
		return
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		clientRulesFail(w, r, domain.ClientRulesInvalid("", "Expected a JSON object"))
		return
	}
	for _, key := range []string{"data", "version", "is_active", "reason"} {
		if value, ok := object[key]; ok && string(value) == "null" {
			clientRulesFail(w, r, domain.ClientRulesInvalid(key, "This field must not be null"))
			return
		}
	}
	var data map[string]json.RawMessage
	if value, ok := object["data"]; !ok || json.Unmarshal(value, &data) != nil || data == nil {
		clientRulesFail(w, r, domain.ClientRulesInvalid("data", "Provide client purchase policy data"))
		return
	}
	for _, key := range []string{"scope_code", "mode", "allowed", "excluded", "effective_from", "evidence"} {
		if value, ok := data[key]; ok && string(value) == "null" {
			clientRulesFail(w, r, domain.ClientRulesInvalid(key, "This field must not be null"))
			return
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	input := domain.ClientRulesSave{Version: 1, Active: true}
	if e := decoder.Decode(&input); e != nil {
		var big *http.MaxBytesError
		if errors.As(e, &big) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			clientRulesFail(w, r, domain.ClientRulesInvalid("", "Invalid client policy input"))
		}
		return
	}
	if e := decoder.Decode(&struct{}{}); e != io.EOF {
		clientRulesFail(w, r, domain.ClientRulesInvalid("", "Expected one JSON object"))
		return
	}
	v, e := h.s.Save(r.Context(), bearer(r), chi.URLParam(r, "client"), chi.URLParam(r, "id"), input)
	if !clientRulesFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "client_purchase_rules.saved").Str("rule_id", v.ID).Int32("version", v.Version).Msg("Client purchase policy committed")
		status := 200
		if r.Method == "POST" {
			status = 201
		}
		write(w, status, v)
	}
}
