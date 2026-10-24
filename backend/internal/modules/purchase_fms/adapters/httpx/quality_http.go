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

type qualityHandler struct{ s *application.QualityService }

func qualityFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_quality.operation.failed", "Quality certificate operation failed")
}

func qualityNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		qualityFail(w, r, domain.QualityInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h qualityHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !qualityNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !qualityFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h qualityHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !qualityNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if qualityFail(w, r, e) {
		return
	}
	q, e := dtos.QualityDecode(bytes.NewReader(raw))
	if qualityFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !qualityFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_quality.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("Quality certificate command committed")
		write(w, 200, v)
	}
}
