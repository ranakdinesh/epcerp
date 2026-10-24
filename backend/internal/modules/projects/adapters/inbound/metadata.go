package inbound

import (
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/application"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"net/http"
	"time"
)

type metadataHandler struct{ s *application.MetadataService }

func RegisterMetadata(r chi.Router, s *application.MetadataService) {
	h := metadataHandler{s}
	r.Post("/api/v1/projects/{id}/schedule-preview", h.preview)
	r.Patch("/api/v1/projects/{id}", h.correct)
	r.Get("/api/v1/projects/{id}/metadata-history", h.history)
	r.Get("/api/v1/projects/{id}/metadata-options", h.options)
}
func (h metadataHandler) correct(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		ClientID string `json:"client_id"`
		Reason   string `json:"reason"`
		Version  int32  `json:"expected_version"`
		CaptureRequest
	}
	if !parse(w, r, &input) {
		return
	}
	p, e := h.s.Correct(r.Context(), token(r), chi.URLParam(r, "id"), domain.Correction{Name: input.Name, ClientID: input.ClientID, Reason: input.Reason, ExpectedVersion: input.Version, Capture: input.capture()})
	if fail(w, r, e) {
		return
	}
	audit(r, *p, "projects.metadata.updated")
	write(w, 200, view(*p))
}
func (h metadataHandler) history(w http.ResponseWriter, r *http.Request) {
	rows, e := h.s.History(r.Context(), token(r), chi.URLParam(r, "id"))
	if fail(w, r, e) {
		return
	}
	out := []map[string]any{}
	for _, v := range rows {
		out = append(out, map[string]any{"id": v.ID, "project_version": v.Version, "reason": v.Reason, "created_by_id": v.ActorID, "created_at": v.OccurredAt.Format(time.RFC3339Nano), "before": view(v.Before), "after": view(v.After)})
	}
	write(w, 200, out)
}
func (h metadataHandler) options(w http.ResponseWriter, r *http.Request) {
	rows, e := h.s.Options(r.Context(), token(r), chi.URLParam(r, "id"))
	if fail(w, r, e) {
		return
	}
	out := []map[string]string{}
	for _, v := range rows {
		out = append(out, map[string]string{"id": v.ID, "name": v.Name})
	}
	write(w, 200, map[string]any{"clients": out})
}

func (h metadataHandler) preview(w http.ResponseWriter, r *http.Request) {
	var input CaptureRequest
	if !parse(w, r, &input) {
		return
	}
	v, e := h.s.Preview(r.Context(), token(r), chi.URLParam(r, "id"), input.capture())
	if fail(w, r, e) {
		return
	}
	write(w, 200, v)
}
