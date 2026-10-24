package httpx

import (
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"net/http"
)

func (h *accountHandler) usernameAvailable(w http.ResponseWriter, r *http.Request) {
	available, e := h.service.UsernameAvailable(r.Context(), bearer(r), r.URL.Query().Get("username"))
	if !failed(w, r, e) {
		w.Header().Set("Cache-Control", "private, no-store")
		write(w, 200, dtos.UsernameAvailability{Available: available})
	}
}
func (h *accountHandler) username(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	p, ok := requestBody(w, r, dtos.ParseUsernameChange)
	if !ok {
		return
	}
	e := h.service.ChangeUsername(r.Context(), bearer(r), *p.Username, *p.Expected)
	audit(r.Context(), "account.username.changed", e != nil)
	if !failed(w, r, e) {
		w.WriteHeader(204)
	}
}
