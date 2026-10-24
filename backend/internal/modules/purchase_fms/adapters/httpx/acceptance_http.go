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

type acceptanceHandler struct {
	s *application.AcceptanceService
}

func acceptanceFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_acceptance.operation.failed", "Vendor-acceptance operation failed")
}

func acceptanceNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		acceptanceFail(w, r, domain.AcceptanceInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h acceptanceHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !acceptanceNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !acceptanceFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h acceptanceHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !acceptanceNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if acceptanceFail(w, r, e) {
		return
	}
	q, e := dtos.AcceptanceDecode(bytes.NewReader(raw))
	if acceptanceFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !acceptanceFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_acceptance.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("Vendor-acceptance command committed")
		write(w, 200, v)
	}
}
