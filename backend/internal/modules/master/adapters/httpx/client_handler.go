package httpx

import (
	"errors"
	"net/http"

	"strconv"
	"strings"
	"unicode/utf8"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type clientHandler struct {
	s *masterApplication.ClientService
}

func RegisterClientRoutes(r chi.Router, s *masterApplication.ClientService) {
	h := clientHandler{s}
	r.Get("/api/v1/clients", h.list)
	r.Post("/api/v1/clients", h.save)
	r.Get("/api/v1/clients/{client_id}", h.get)
	r.Patch("/api/v1/clients/{client_id}", h.save)
	r.Post("/api/v1/clients/{client_id}/deactivate", h.status)
	r.Post("/api/v1/clients/{client_id}/activate", h.status)
}

func clientFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "clients.operation.failed")
}

func clientInvalid(w http.ResponseWriter, source, field string) {
	writeJSON(w, 422, map[string]any{"detail": []map[string]any{{"type": "value_error", "loc": []string{source, field}, "msg": "Invalid or missing value"}}})
}

func clientId(w http.ResponseWriter, r *http.Request) (string, bool) {
	v, e := masterDomain.ClientCanonicalID(chi.URLParam(r, "client_id"))
	if e != nil {
		clientInvalid(w, "path", "client_id")
		return "", false
	}
	return v, true
}
func (h clientHandler) get(w http.ResponseWriter, r *http.Request) {
	v, ok := clientId(w, r)
	if !ok {
		return
	}
	c, e := h.s.Get(r.Context(), bearerToken(r), v)
	if !clientFail(w, r, e) {
		writeJSON(w, 200, c)
	}
}
func (h clientHandler) status(w http.ResponseWriter, r *http.Request) {
	v, ok := clientId(w, r)
	if !ok {
		return
	}
	active := strings.HasSuffix(r.URL.Path, "/activate")
	c, e := h.s.SetActive(r.Context(), bearerToken(r), v, active)
	if clientFail(w, r, e) {
		return
	}
	kind := "clients.deactivated"
	if active {
		kind = "clients.activated"
	}
	logging.Ctx(r.Context()).Warn().Str("event", kind).Str("client_id", c.ID).Str("actor_id", *c.UpdatedBy).Msg("Client availability changed")
	writeJSON(w, 200, c)
}
func (h clientHandler) save(w http.ResponseWriter, r *http.Request) {
	v := ""
	if r.Method == http.MethodPatch {
		var ok bool
		v, ok = clientId(w, r)
		if !ok {
			return
		}
	}
	in, field, e := clientParse(w, r)
	if e != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(e, &tooLarge) {
			writeJSON(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			clientInvalid(w, "body", field)
		}
		return
	}
	c, e := h.s.Save(r.Context(), bearerToken(r), v, in)
	if clientFail(w, r, e) {
		return
	}
	code := 200
	kind := "clients.updated"
	if r.Method == http.MethodPost {
		code = 201
		kind = "clients.created"
	}
	logging.Ctx(r.Context()).Info().Str("event", kind).Str("client_id", c.ID).Str("actor_id", *c.UpdatedBy).Msg("Client master saved")
	writeJSON(w, code, c)
}
func (h clientHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := masterDomain.ClientFilter{Page: 1, Size: 20, Sort: "legal_name"}
	for _, p := range []struct {
		name string
		dest *int32
		max  int64
	}{{"page", &f.Page, 2147483647}, {"page_size", &f.Size, 100}} {
		if q.Has(p.name) {
			v, e := strconv.ParseInt(masterDomain.ClientTrim(q.Get(p.name)), 10, 32)
			if e != nil || v < 1 || v > p.max {
				clientInvalid(w, "query", p.name)
				return
			}
			*p.dest = int32(v)
		}
	}
	if int64(f.Page-1)*int64(f.Size) > 2147483647 {
		clientInvalid(w, "query", "page")
		return
	}
	for _, p := range []struct {
		name string
		dest *string
		max  int
	}{{"search", &f.Search, 128}, {"client_type", &f.ClientType, 64}} {
		if q.Has(p.name) {
			v := q.Get(p.name)
			n := utf8.RuneCountInString(v)
			if n < 1 || n > p.max {
				clientInvalid(w, "query", p.name)
				return
			}
			*p.dest = v
		}
	}
	if q.Has("status") {
		v := q.Get("status")
		if v != "active" && v != "inactive" {
			clientInvalid(w, "query", "status")
			return
		}
		active := v == "active"
		f.Active = &active
	}
	if q.Has("sort") {
		v := q.Get("sort")
		i := strings.LastIndex(v, "_")
		if i < 0 {
			clientInvalid(w, "query", "sort")
			return
		}
		f.Sort = v[:i]
		dir := v[i+1:]
		if (f.Sort != "legal_name" && f.Sort != "client_code" && f.Sort != "created_at") || (dir != "asc" && dir != "desc") {
			clientInvalid(w, "query", "sort")
			return
		}
		f.Descending = dir == "desc"
	}
	page, e := h.s.List(r.Context(), bearerToken(r), f)
	if !clientFail(w, r, e) {
		writeJSON(w, 200, page)
	}
}

// Exact key validation avoids encoding/json's case-insensitive struct field matching.
