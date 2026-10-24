package httpx

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx/dtos"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type advancesHandler struct{ s *application.AdvancesService }

func advancesFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_advances.operation.failed", "Advance-request operation failed")
}

func advancesNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		advancesFail(w, r, domain.AdvancesInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h advancesHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !advancesNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !advancesFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h advancesHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !advancesNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if advancesFail(w, r, e) {
		return
	}
	q, e := dtos.AdvancesDecode(bytes.NewReader(raw))
	if advancesFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !advancesFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_advances.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("Advance-request command committed")
		write(w, 200, v)
	}
}

func (h advancesHandler) queue(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	for k, v := range values {
		if k != "offset" || len(v) != 1 || v[0] == "" {
			advancesFail(w, r, domain.AdvancesInvalid("query", "Use only one optional offset"))
			return
		}
	}
	offset := int64(0)
	var e error
	if raw := values.Get("offset"); raw != "" {
		offset, e = strconv.ParseInt(raw, 10, 32)
		if e != nil || offset < 0 || offset > 1000000 {
			advancesFail(w, r, domain.AdvancesInvalid("offset", "Use an offset between 0 and 1000000"))
			return
		}
	}
	page, e := h.s.Queue(r.Context(), bearer(r), int32(offset))
	if advancesFail(w, r, e) {
		return
	}
	write(w, 200, page)
}
