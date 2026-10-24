package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	"io"
	"net/http"
	"strings"
)

type handler struct{ s *application.Service }

func Register(r chi.Router, s *application.Service) {
	h := handler{s}
	r.Get("/api/v1/projects/{project}/responsibilities", h.list)
	r.Get("/api/v1/projects/{project}/responsibilities/options", h.options)
	r.Get("/api/v1/projects/{project}/responsibilities/{id}/history", h.history)
	r.Post("/api/v1/projects/{project}/responsibilities", h.create)
	r.Patch("/api/v1/projects/{project}/responsibilities/{id}", h.update)
}
func token(r *http.Request) string {
	p := strings.Fields(r.Header.Get("Authorization"))
	if len(p) == 2 && strings.EqualFold(p[0], "Bearer") {
		return p[1]
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
		logging.Ctx(r.Context()).Error().Str("event", "project_access.operation.failed").Msg("Project responsibility operation failed")
		write(w, 500, map[string]string{"detail": "Internal server error"})
	}
	return true
}
func (h handler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Workspace(r.Context(), token(r), chi.URLParam(r, "project"))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func (h handler) options(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Options(r.Context(), token(r), chi.URLParam(r, "project"))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func (h handler) history(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.History(r.Context(), token(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func decode(w http.ResponseWriter, r *http.Request) (domain.Input, error) {
	var v domain.Input
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<10))
	if e != nil {
		var size *http.MaxBytesError
		if errors.As(e, &size) {
			return v, &domain.Error{Status: 413, Message: "Request body too large"}
		}
		return v, domain.Invalid("", "Unable to read input")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return v, domain.Invalid("", "Expected one JSON object")
	}
	for _, k := range []string{"user_id", "bundle_id", "responsibility", "permission_codes", "is_active", "version", "reason"} {
		if _, ok := fields[k]; !ok {
			return v, domain.Invalid(k, "This field is required")
		}
	}
	for k, value := range fields {
		if strings.TrimSpace(string(value)) == "null" {
			return v, domain.Invalid(k, "This field must not be null")
		}
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil {
		return v, domain.Invalid("", "Invalid responsibility input")
	}
	return v, nil
}
func (h handler) create(w http.ResponseWriter, r *http.Request) { h.save(w, r, "", 201) }
func (h handler) update(w http.ResponseWriter, r *http.Request) {
	h.save(w, r, chi.URLParam(r, "id"), 200)
}
func (h handler) save(w http.ResponseWriter, r *http.Request, id string, status int) {
	input, e := decode(w, r)
	if fail(w, r, e) {
		return
	}
	v, e := h.s.Save(r.Context(), token(r), chi.URLParam(r, "project"), id, input)
	if fail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "project_access.profile.saved").Str("project_id", v.ProjectID).Str("profile_id", v.ID).Int32("version", v.Version).Msg("Project responsibility saved")
	write(w, status, v)
}
