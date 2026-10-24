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

type gRNsHandler struct{ s *application.GRNsService }

func gRNsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_grns.operation.failed", "GRN inwarding operation failed")
}

func gRNsNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		gRNsFail(w, r, domain.GRNsInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h gRNsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !gRNsNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !gRNsFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h gRNsHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !gRNsNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if gRNsFail(w, r, e) {
		return
	}
	q, e := dtos.GRNsDecode(bytes.NewReader(raw))
	if gRNsFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !gRNsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_grns.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("GRN inwarding command committed")
		write(w, 200, v)
	}
}
