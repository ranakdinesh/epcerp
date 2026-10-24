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

type engineerConfirmationsHandler struct {
	s *application.EngineerConfirmationsService
}

func engineerConfirmationsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_engineer_confirmations.operation.failed", "Engineer confirmation operation failed")
}

func engineerConfirmationsNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		engineerConfirmationsFail(w, r, domain.EngineerConfirmationsInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h engineerConfirmationsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !engineerConfirmationsNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !engineerConfirmationsFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h engineerConfirmationsHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !engineerConfirmationsNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if engineerConfirmationsFail(w, r, e) {
		return
	}
	q, e := dtos.EngineerConfirmationsDecode(bytes.NewReader(raw))
	if engineerConfirmationsFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !engineerConfirmationsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_engineer_confirmations.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("Engineer confirmation command committed")
		write(w, 200, v)
	}
}
