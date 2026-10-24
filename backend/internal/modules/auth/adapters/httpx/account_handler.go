package httpx

import (
	"net/http"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
)

type accountHandler struct{ service *application.AccountService }

func (h *accountHandler) password(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	change, ok := requestBody(w, r, dtos.ParseAccountPassword)
	if !ok {
		return
	}
	e := h.service.Change(r.Context(), bearer(r), change.Current, nil, &change.Value)
	audit(r.Context(), "account.password.changed", e != nil)
	if !failed(w, r, e) {
		w.WriteHeader(204)
	}
}

func (h *accountHandler) email(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	change, ok := requestBody(w, r, dtos.ParseAccountEmail)
	if !ok {
		return
	}
	e := h.service.Change(r.Context(), bearer(r), change.Current, &change.Value, nil)
	audit(r.Context(), "account.email.changed", e != nil)
	if !failed(w, r, e) {
		w.WriteHeader(204)
	}
}
