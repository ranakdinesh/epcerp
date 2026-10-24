package httpx

import (
	"encoding/json"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"io"
	"net/http"
	"strconv"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type manufacturerHandler struct {
	s *masterApplication.ManufacturerService
}

func RegisterManufacturerRoutes(r chi.Router, s *masterApplication.ManufacturerService) {
	h := manufacturerHandler{s}
	r.Get("/api/v1/manufacturers", h.list)
	r.Get("/api/v1/manufacturers/eligibility", h.options)
	r.Post("/api/v1/manufacturers/legacy-import", h.importRow)
	r.Post("/api/v1/manufacturers", h.create)
	r.Patch("/api/v1/manufacturers/{identity}", h.active)
	r.Patch("/api/v1/manufacturers/{identity}/profile", h.profile)
	r.Get("/api/v1/manufacturers/{identity}/history", h.history)
}

func manufacturerFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "manufacturers.operation.failed")
}

func manufacturerInvalid(w http.ResponseWriter, field string) {
	writeJSON(w, 422, map[string]any{"detail": []map[string]any{{"type": "value_error", "loc": []string{"body", field}, "msg": "Invalid or missing value"}}})
}

func manufacturerId(w http.ResponseWriter, r *http.Request) (string, bool) {
	v, e := masterDomain.ManufacturerCanonicalID(chi.URLParam(r, "identity"))
	if e != nil {
		writeJSON(w, 422, map[string]any{"detail": []map[string]any{{"type": "value_error", "loc": []string{"path", "identity"}, "msg": "Input should be a valid UUID"}}})
		return "", false
	}
	return v, true
}
func (h manufacturerHandler) list(w http.ResponseWriter, r *http.Request) {
	values, e := h.s.List(r.Context(), bearerToken(r))
	if manufacturerFail(w, r, e) {
		return
	}
	items := []masterDTO.ManufacturerResponse{}
	for _, v := range values {
		items = append(items, masterDTO.ManufacturerFromDomain(v))
	}
	writeJSON(w, 200, items)
}
func (h manufacturerHandler) history(w http.ResponseWriter, r *http.Request) {
	identity, ok := manufacturerId(w, r)
	if !ok {
		return
	}
	values, e := h.s.History(r.Context(), bearerToken(r), identity)
	if manufacturerFail(w, r, e) {
		return
	}
	items := []map[string]any{}
	for _, v := range values {
		items = append(items, masterDTO.ManufacturerHistoryFromDomain(v))
	}
	writeJSON(w, 200, items)
}
func manufacturerAudit(r *http.Request, v masterDomain.Manufacturer, event string) {
	logging.Ctx(r.Context()).Info().Str("event", event).Str("manufacturer_id", v.ID).Int32("version", v.Version).Str("actor_id", *v.UpdatedBy).Msg("Manufacturer change committed")
}

func (h manufacturerHandler) create(w http.ResponseWriter, r *http.Request) {
	in, ok := manufacturerParse(w, r, "create")
	if !ok {
		return
	}
	v, e := h.s.Create(r.Context(), bearerToken(r), in.Code, in.Name, in.Data, in.Reason)
	if manufacturerFail(w, r, e) {
		return
	}
	manufacturerAudit(r, *v, "manufacturers.created")
	writeJSON(w, 201, masterDTO.ManufacturerFromDomain(*v))
}
func (h manufacturerHandler) profile(w http.ResponseWriter, r *http.Request) {
	identity, ok := manufacturerId(w, r)
	if !ok {
		return
	}
	in, ok := manufacturerParse(w, r, "profile")
	if !ok {
		return
	}
	v, e := h.s.Profile(r.Context(), bearerToken(r), identity, in.Version, in.Name, in.Data, in.Reason)
	if manufacturerFail(w, r, e) {
		return
	}
	manufacturerAudit(r, *v, "manufacturers.updated")
	writeJSON(w, 200, masterDTO.ManufacturerFromDomain(*v))
}
func (h manufacturerHandler) active(w http.ResponseWriter, r *http.Request) {
	identity, ok := manufacturerId(w, r)
	if !ok {
		return
	}
	in, ok := manufacturerParse(w, r, "active")
	if !ok {
		return
	}
	v, e := h.s.SetActive(r.Context(), bearerToken(r), identity, in.Version, in.Active, in.Reason)
	if manufacturerFail(w, r, e) {
		return
	}
	manufacturerAudit(r, *v, "manufacturers.lifecycle.changed")
	writeJSON(w, 200, masterDTO.ManufacturerFromDomain(*v))
}

// Versions are positive JSON integers; string/float/bool coercion is unsupported.

func (h manufacturerHandler) options(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	for k, v := range q {
		if (k != "search" && k != "codes" && k != "skip" && k != "kind" && k != "category_code") || len(v) != 1 {
			manufacturerInvalid(w, "query")
			return
		}
	}
	skip := int64(0)
	var e error
	if q.Get("skip") != "" {
		skip, e = strconv.ParseInt(q.Get("skip"), 10, 32)
		if e != nil {
			manufacturerInvalid(w, "skip")
			return
		}
	}
	var codes []string
	if q.Get("codes") != "" {
		codes = strings.Split(q.Get("codes"), ",")
	}
	v, e := h.s.Options(r.Context(), bearerToken(r), q.Get("search"), codes, int32(skip), q.Get("kind"), q.Get("category_code"))
	if !manufacturerFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h manufacturerHandler) importRow(w http.ResponseWriter, r *http.Request) {
	var in masterDomain.ManufacturerLegacy
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if e := masterDTO.DecodeMasterJSON(d, &in); e != nil {
		var big *http.MaxBytesError
		if errors.As(e, &big) {
			writeJSON(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			manufacturerInvalid(w, "source")
		}
		return
	}
	var extra any
	if masterDTO.DecodeMasterJSON(d, &extra) != io.EOF {
		manufacturerInvalid(w, "source")
		return
	}
	v, e := h.s.Import(r.Context(), bearerToken(r), in)
	if !manufacturerFail(w, r, e) {
		manufacturerAudit(r, *v, "manufacturers.imported")
		writeJSON(w, 200, masterDTO.ManufacturerFromDomain(*v))
	}
}
