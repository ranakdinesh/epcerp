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

type inspectionReportsHandler struct {
	s *application.InspectionReportsService
}

func inspectionReportsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_inspection_reports.operation.failed", "IR / IRN operation failed")
}

func inspectionReportsNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		inspectionReportsFail(w, r, domain.InspectionReportsInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h inspectionReportsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !inspectionReportsNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !inspectionReportsFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h inspectionReportsHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !inspectionReportsNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if inspectionReportsFail(w, r, e) {
		return
	}
	q, e := dtos.InspectionReportsDecode(bytes.NewReader(raw))
	if inspectionReportsFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !inspectionReportsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_inspection_reports.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("IR / IRN command committed")
		write(w, 200, v)
	}
}
