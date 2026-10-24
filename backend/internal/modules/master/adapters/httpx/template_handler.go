package httpx

import (
	"encoding/json"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"io"
	"net/http"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type templateHandler struct {
	s *masterApplication.TemplateService
}

func RegisterTemplateRoutes(r chi.Router, s *masterApplication.TemplateService) {
	h := templateHandler{s}
	r.Get("/api/v1/configuration-templates/{id}", h.get)
	r.Post("/api/v1/configuration-templates/{id}/preview", h.preview)
	r.Post("/api/v1/configuration-templates/{id}/publish", h.publish)
	r.Post("/api/v1/configuration-templates/import", h.importRows)
}

func templateFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "configuration_templates.operation.failed")
}

func templateDecode(w http.ResponseWriter, r *http.Request, limit int64, out any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	d.DisallowUnknownFields()
	if e := masterDTO.DecodeMasterJSON(d, out); e != nil {
		var max *http.MaxBytesError
		if errors.As(e, &max) {
			writeJSON(w, 413, map[string]string{"detail": "Request exceeds permitted size"})
		} else {
			writeJSON(w, 422, map[string]string{"detail": "Invalid JSON or unknown field"})
		}
		return false
	}
	var extra any
	if masterDTO.DecodeMasterJSON(d, &extra) != io.EOF {
		writeJSON(w, 422, map[string]string{"detail": "One JSON object is required"})
		return false
	}
	return true
}
func (h templateHandler) get(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Get(r.Context(), bearerToken(r), chi.URLParam(r, "id"))
	if !templateFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h templateHandler) preview(w http.ResponseWriter, r *http.Request) {
	var in masterDTO.TemplatePreviewRequest

	if !templateDecode(w, r, 16<<10, &in) {
		return
	}
	v, e := h.s.Preview(r.Context(), bearerToken(r), chi.URLParam(r, "id"), in.Values)
	if !templateFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h templateHandler) importRows(w http.ResponseWriter, r *http.Request) {
	var in masterDTO.TemplateImportRowsRequest

	if !templateDecode(w, r, 1<<20, &in) {
		return
	}
	v, e := h.s.Import(r.Context(), bearerToken(r), templateDefinitions(in.Templates))
	if templateFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "configuration_templates.import.committed").Int("created", v.Created).Int("unchanged", v.Unchanged).Msg("Template adoption committed")
	writeJSON(w, 200, v)
}

func (h templateHandler) publish(w http.ResponseWriter, r *http.Request) {
	var in masterDTO.TemplatePublishRequest

	if !templateDecode(w, r, 1<<20, &in) {
		return
	}
	if in.ExpectedRevision == nil {
		templateFail(w, r, masterDomain.TemplateInvalid("Expected revision is required"))
		return
	}
	v, e := h.s.Publish(r.Context(), bearerToken(r), chi.URLParam(r, "id"), in.Definition.Domain(), *in.ExpectedRevision, in.Reason)
	if !templateFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "configuration_templates.publish.committed").Int32("revision", v.Revision).Msg("Configuration revision published")
		writeJSON(w, 200, v)
	}
}

func templateDefinitions(values []masterDTO.TemplateDefinitionDTO) []masterDomain.TemplateDefinition {
	out := make([]masterDomain.TemplateDefinition, len(values))
	for i, v := range values {
		out[i] = v.Domain()
	}
	return out
}
