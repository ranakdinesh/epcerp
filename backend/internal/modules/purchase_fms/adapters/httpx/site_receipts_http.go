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

type siteReceiptsHandler struct {
	s *application.SiteReceiptsService
}

func siteReceiptsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_site_receipts.operation.failed", "Site material receipt operation failed")
}

func siteReceiptsNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		siteReceiptsFail(w, r, domain.SiteReceiptsInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h siteReceiptsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !siteReceiptsNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !siteReceiptsFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h siteReceiptsHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !siteReceiptsNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if siteReceiptsFail(w, r, e) {
		return
	}
	q, e := dtos.SiteReceiptsDecode(bytes.NewReader(raw))
	if siteReceiptsFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !siteReceiptsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_site_receipts.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("Site material receipt command committed")
		write(w, 200, v)
	}
}
