package httpx

import (
	"net/http"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type sessionHandler struct{ service *application.SessionService }

func (h *sessionHandler) login(w http.ResponseWriter, r *http.Request) {
	credentials, ok := requestBody(w, r, dtos.ParseLogin)
	if !ok {
		return
	}
	pair, err := h.service.Login(r.Context(), credentials.Identifier, credentials.Password)
	if err != nil {
		audit(r.Context(), "auth.login.failed", true)
	}
	if failed(w, r, err) {
		return
	}
	audit(r.Context(), "auth.login.succeeded", false)
	write(w, 200, dtos.Tokens(pair))
}
func refreshPayload(w http.ResponseWriter, r *http.Request) (string, bool) {
	refresh, ok := requestBody(w, r, dtos.ParseRefresh)
	if !ok {
		return "", false
	}

	return refresh, true
}
func (h *sessionHandler) refresh(w http.ResponseWriter, r *http.Request) {
	raw, ok := refreshPayload(w, r)
	if !ok {
		return
	}
	pair, err := h.service.Refresh(r.Context(), raw)
	if err != nil {
		audit(r.Context(), "auth.refresh.failed", true)
	}
	if failed(w, r, err) {
		return
	}
	audit(r.Context(), "auth.refresh.succeeded", false)
	write(w, 200, dtos.Tokens(pair))
}
func (h *sessionHandler) logout(w http.ResponseWriter, r *http.Request) {
	raw, ok := refreshPayload(w, r)
	if !ok {
		return
	}
	if failed(w, r, h.service.Logout(r.Context(), raw)) {
		return
	}
	audit(r.Context(), "auth.logout.succeeded", false)
	w.WriteHeader(204)
}
func (h *sessionHandler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.Me(r.Context(), bearer(r))
	if failed(w, r, err) {
		return
	}
	write(w, 200, map[string]any{"id": user.ID, "username": user.Username, "email": user.Email, "first_name": user.FirstName, "last_name": user.LastName, "mobile": user.Mobile, "is_active": user.IsActive, "is_superuser": user.IsSuperuser, "permissions": domain.EffectivePermissions(*user), "must_change_password": user.MustChangePassword})
}
func (h *sessionHandler) activity(w http.ResponseWriter, r *http.Request) {
	page, ok := requestBody(w, r, dtos.ParseActivity)
	if !ok {
		return
	}
	if failed(w, r, h.service.Activity(r.Context(), bearer(r), page)) {
		return
	}
	w.WriteHeader(204)
}
