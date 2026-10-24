package httpx

import (
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"net/http"
)

type offboardingHandler struct {
	s *application.OffboardingService
}

func (h *offboardingHandler) list(w http.ResponseWriter, r *http.Request) {
	values, e := h.s.Cases(r.Context(), bearer(r))
	if failed(w, r, e) {
		return
	}
	out := []dtos.OffboardingResponse{}
	for _, v := range values {
		out = append(out, dtos.Offboarding(v))
	}
	write(w, 200, out)
}
func (h *offboardingHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "case_id")
	if !ok {
		return
	}
	v, e := h.s.Get(r.Context(), bearer(r), id)
	if failed(w, r, e) {
		return
	}
	write(w, 200, dtos.Offboarding(*v))
}
func (h *offboardingHandler) create(w http.ResponseWriter, r *http.Request) {
	p, ok := requestBody(w, r, dtos.ParseDeparture)
	if !ok {
		return
	}
	v, e := h.s.Create(r.Context(), bearer(r), p.UserID, p.Reason)
	if failed(w, r, e) {
		return
	}
	audit(r.Context(), "offboarding.opened", false)
	write(w, 201, dtos.Offboarding(v))
}
func (h *offboardingHandler) candidates(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "case_id")
	if !ok {
		return
	}
	values, e := h.s.Candidates(r.Context(), bearer(r), id)
	if failed(w, r, e) {
		return
	}
	out := []dtos.ReplacementResponse{}
	for _, v := range values {
		out = append(out, dtos.Replacement(v))
	}
	write(w, 200, out)
}
func (h *offboardingHandler) change(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "case_id")
	if !ok {
		return
	}
	p, ok := requestBody(w, r, dtos.ParseHandover)
	if !ok {
		return
	}
	v, e := h.s.Change(r.Context(), bearer(r), id, chi.URLParam(r, "action"), p)
	if failed(w, r, e) {
		return
	}
	audit(r.Context(), "offboarding.changed", false)
	write(w, 200, dtos.Offboarding(v))
}

func (h *offboardingHandler) history(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "case_id")
	if !ok {
		return
	}
	values, e := h.s.History(r.Context(), bearer(r), id)
	if failed(w, r, e) {
		return
	}
	out := []dtos.OffboardingChangeResponse{}
	for _, v := range values {
		out = append(out, dtos.OffboardingChange(v))
	}
	write(w, 200, out)
}
