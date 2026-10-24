package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/domain"
	"io"
	"net/http"
	"strings"
)

type handler struct{ s *application.Service }

func Register(r chi.Router, s *application.Service) {
	h := handler{s}
	r.Get("/api/v1/projects/{project}/items", h.list)
	r.Get("/api/v1/projects/{project}/items/catalogue", h.catalogue)
	r.Get("/api/v1/projects/{project}/items/{id}/history", h.history)
	r.Post("/api/v1/projects/{project}/items", h.save)
	r.Patch("/api/v1/projects/{project}/items/{id}", h.save)
}
func token(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
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
		logging.Ctx(r.Context()).Error().Str("event", "project_items.operation.failed").Msg("Project material requirement operation failed")
		write(w, 500, map[string]string{"detail": "Internal server error"})
	}
	return true
}
func (h handler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), token(r), chi.URLParam(r, "project"))
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
func (h handler) save(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			fail(w, r, domain.Invalid("", "Unable to read input"))
		}
		return
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		fail(w, r, domain.Invalid("", "Expected a JSON object"))
		return
	}
	for _, key := range []string{"item_id", "quantity", "required_date", "version", "is_active", "reason"} {
		if value, ok := object[key]; ok && string(value) == "null" {
			fail(w, r, domain.Invalid(key, "This field must not be null"))
			return
		}
	}
	for _, key := range []string{"item_id", "quantity", "required_date", "allocations", "alternatives"} {
		if value, ok := object[key]; ok && string(value) == "null" {
			fail(w, r, domain.Invalid(key, "This field must not be null"))
			return
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	input := domain.Save{Version: 1, Active: true}
	if e := decoder.Decode(&input); e != nil {
		var big *http.MaxBytesError
		if errors.As(e, &big) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			fail(w, r, domain.Invalid("", "Invalid material requirement input"))
		}
		return
	}
	if e := decoder.Decode(&struct{}{}); e != io.EOF {
		fail(w, r, domain.Invalid("", "Expected one JSON object"))
		return
	}
	v, e := h.s.Save(r.Context(), token(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"), input)
	if !fail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "project_items.saved").Str("demand_id", v.ID).Int32("version", v.Version).Msg("Project material requirement committed")
		status := 200
		if r.Method == "POST" {
			status = 201
		}
		write(w, status, v)
	}
}

func (h handler) catalogue(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Catalogue(r.Context(), token(r), chi.URLParam(r, "project"))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
