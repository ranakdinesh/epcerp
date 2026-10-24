package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/application"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
	"io"
	"net/http"
	"strings"
)

type handler struct{ s *application.Service }

func Register(r chi.Router, s *application.Service) {
	h := handler{s}
	r.Get("/api/v1/administration/director", h.read)
	r.Get("/api/v1/administration/director/options", h.options)
	r.Post("/api/v1/administration/director", h.save)
}
func token(r *http.Request) string {
	v := strings.Fields(r.Header.Get("Authorization"))
	if len(v) == 2 && strings.EqualFold(v[0], "Bearer") {
		return v[1]
	}
	return ""
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, r *http.Request, e error) bool {
	if e == nil {
		return false
	}
	var d *domain.Error
	if errors.As(e, &d) {
		if d.Status == 401 {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
		if d.Field != "" {
			write(w, d.Status, map[string]any{"detail": []map[string]any{{"loc": []string{"body", d.Field}, "msg": d.Message, "type": "value_error"}}})
		} else {
			write(w, d.Status, map[string]string{"detail": d.Message})
		}
	} else {
		logging.Ctx(r.Context()).Error().Str("event", "organisation_governance.operation.failed").Msg("Director designation operation failed")
		write(w, 500, map[string]string{"detail": "Internal server error"})
	}
	return true
}
func (h handler) read(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Workspace(r.Context(), token(r))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func (h handler) options(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Options(r.Context(), token(r))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func decode(w http.ResponseWriter, r *http.Request) (domain.Input, error) {
	var v domain.Input
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
	if e != nil {
		var limit *http.MaxBytesError
		if errors.As(e, &limit) {
			return v, &domain.Error{Status: 413, Message: "Request body too large"}
		}
		return v, domain.Invalid("", "Unable to read input")
	}
	var f map[string]json.RawMessage
	if json.Unmarshal(raw, &f) != nil || f == nil {
		return v, domain.Invalid("", "Expected one JSON object")
	}
	for _, k := range []string{"user_id", "version", "is_active", "reason"} {
		if _, ok := f[k]; !ok {
			return v, domain.Invalid(k, "This field is required")
		}
	}
	for k, value := range f {
		if strings.TrimSpace(string(value)) == "null" {
			return v, domain.Invalid(k, "This field must not be null")
		}
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil {
		return v, domain.Invalid("", "Invalid Director designation input")
	}
	return v.Validate()
}
func (h handler) save(w http.ResponseWriter, r *http.Request) {
	input, e := decode(w, r)
	if fail(w, r, e) {
		return
	}
	v, e := h.s.Save(r.Context(), token(r), input)
	if fail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "organisation_governance.designation.saved").Str("designation_id", v.Record.ID).Int32("version", v.Record.Version).Msg("Director designation saved")
	write(w, 200, v)
}
