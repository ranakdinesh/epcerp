package httpx

import (
	"bytes"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx/dtos"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type inspectionsHandler struct {
	s *application.InspectionsService
}

func inspectionsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_inspections.operation.failed", "Material-inspection operation failed")
}

func inspectionsNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		inspectionsFail(w, r, domain.InspectionsInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h inspectionsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !inspectionsNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !inspectionsFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h inspectionsHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !inspectionsNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if inspectionsFail(w, r, e) {
		return
	}
	q, e := dtos.InspectionsDecode(bytes.NewReader(raw))
	if inspectionsFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !inspectionsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_inspections.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("Material-inspection command committed")
		write(w, 200, v)
	}
}
