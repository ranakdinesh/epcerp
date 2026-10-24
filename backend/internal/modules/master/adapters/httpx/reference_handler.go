package httpx

import (
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"net/http"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type referenceHandler struct {
	service *masterApplication.ReferenceService
}

func RegisterReferenceRoutes(r chi.Router, s *masterApplication.ReferenceService) {
	h := referenceHandler{s}
	r.Get("/api/v1/client-references/{kind}", h.list)
	r.Post("/api/v1/client-references/{kind}", h.save)
	r.Patch("/api/v1/client-references/{kind}/{reference_id}", h.save)
	r.Post("/api/v1/client-references/{kind}/{reference_id}/deactivate", h.status)
	r.Post("/api/v1/client-references/{kind}/{reference_id}/activate", h.status)
}

func referenceFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "references.operation.failed")
}

func referenceInvalid(w http.ResponseWriter, source, field string) {
	writeJSON(w, 422, map[string]any{"detail": []map[string]any{{"type": "value_error", "loc": []string{source, field}, "msg": "Invalid or missing value"}}})
}

func referenceId(w http.ResponseWriter, r *http.Request) (string, bool) {
	v, e := masterDomain.ReferenceCanonicalID(chi.URLParam(r, "reference_id"))
	if e != nil {
		referenceInvalid(w, "path", "reference_id")
		return "", false
	}
	return v, true
}

func (h referenceHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := ""
	if q.Has("state_id") {
		var e error
		state, e = masterDomain.ReferenceCanonicalID(q.Get("state_id"))
		if e != nil {
			referenceInvalid(w, "query", "state_id")
			return
		}
	}
	include := false
	if q.Has("include_inactive") {
		var ok bool
		include, ok = masterDTO.ReferenceBoolean(q.Get("include_inactive"))
		if !ok {
			referenceInvalid(w, "query", "include_inactive")
			return
		}
	}
	items, e := h.service.List(r.Context(), bearerToken(r), chi.URLParam(r, "kind"), state, !include)
	if referenceFail(w, r, e) {
		return
	}
	views := []masterDTO.ReferenceResponse{}
	for _, v := range items {
		views = append(views, masterDTO.ReferenceFromDomain(v))
	}
	writeJSON(w, 200, views)
}
func (h referenceHandler) save(w http.ResponseWriter, r *http.Request) {
	referenceID := ""
	if r.Method == http.MethodPatch {
		var ok bool
		referenceID, ok = referenceId(w, r)
		if !ok {
			return
		}
	}
	in, field, e := referenceParse(w, r)
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			writeJSON(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			referenceInvalid(w, "body", field)
		}
		return
	}
	k := chi.URLParam(r, "kind")
	v, e := h.service.Save(r.Context(), bearerToken(r), k, referenceID, in)
	if referenceFail(w, r, e) {
		return
	}
	code := 200
	event := "client_references.updated"
	if r.Method == http.MethodPost {
		code = 201
		event = "client_references.created"
	}
	referenceAudit(r, k, *v, event)
	writeJSON(w, code, masterDTO.ReferenceFromDomain(*v))
}
func (h referenceHandler) status(w http.ResponseWriter, r *http.Request) {
	referenceID, ok := referenceId(w, r)
	if !ok {
		return
	}
	k := chi.URLParam(r, "kind")
	active := strings.HasSuffix(r.URL.Path, "/activate")
	v, e := h.service.SetActive(r.Context(), bearerToken(r), k, referenceID, active)
	if referenceFail(w, r, e) {
		return
	}
	event := "client_references.deactivated"
	if active {
		event = "client_references.activated"
	}
	referenceAudit(r, k, *v, event)
	writeJSON(w, 200, masterDTO.ReferenceFromDomain(*v))
}
func referenceAudit(r *http.Request, k string, v masterDomain.Reference, event string) {
	logging.Ctx(r.Context()).Info().Str("event", event).Str("reference_kind", k).Str("reference_id", v.ID).Str("actor_id", *v.UpdatedBy).Msg("Reference administration committed")
}

// Max valid Unicode request is below 1 KiB; retain an explicit 4 KiB ceiling.
