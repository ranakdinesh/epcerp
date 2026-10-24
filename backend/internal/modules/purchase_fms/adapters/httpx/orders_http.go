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

type ordersHandler struct{ s *application.OrdersService }

func ordersFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_orders.operation.failed", "Purchase-order operation failed")
}

func ordersNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		ordersFail(w, r, domain.OrdersInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h ordersHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !ordersNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !ordersFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h ordersHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !ordersNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if ordersFail(w, r, e) {
		return
	}
	q, e := dtos.OrdersDecode(bytes.NewReader(raw))
	if ordersFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !ordersFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_orders.command.committed").Str("action", q.Action).Str("case_id", v.Order.ID).Msg("Purchase-order command committed")
		write(w, 200, v)
	}
}

func (h ordersHandler) document(w http.ResponseWriter, r *http.Request) {
	if !ordersNoQuery(w, r) {
		return
	}
	v, e := h.s.Document(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !ordersFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_orders.print_requested").Str("order_id", v.ID).Msg("Purchase-order document requested")
		write(w, 200, v)
	}
}
