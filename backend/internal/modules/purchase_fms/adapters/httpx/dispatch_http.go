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

type dispatchHandler struct{ s *application.DispatchService }

func dispatchFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_dispatch.operation.failed", "Material dispatch operation failed")
}

func dispatchNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		dispatchFail(w, r, domain.DispatchInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h dispatchHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !dispatchNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !dispatchFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h dispatchHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !dispatchNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if dispatchFail(w, r, e) {
		return
	}
	q, e := dtos.DispatchDecode(bytes.NewReader(raw))
	if dispatchFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !dispatchFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_dispatch.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("Material dispatch command committed")
		write(w, 200, v)
	}
}
