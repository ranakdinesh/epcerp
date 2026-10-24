package httpx

import (
	"net/http"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
)

type userHandler struct {
	service     *application.UserService
	credentials *application.CredentialService
}

func (h *userHandler) users(w http.ResponseWriter, r *http.Request) {
	filter, err := dtos.ParseUserFilter(r.URL.Query())
	if err != nil {
		validationFailure(w, err)
		return
	}
	page, err := h.service.Users(r.Context(), bearer(r), filter)
	if failed(w, r, err) {
		return
	}
	items := []dtos.UserResponse{}
	for _, user := range page.Items {
		items = append(items, dtos.User(user))
	}
	pages := (page.Total + int64(filter.PageSize) - 1) / int64(filter.PageSize)
	write(w, 200, map[string]any{"items": items, "page": filter.Page, "page_size": filter.PageSize, "total": page.Total, "total_pages": pages})
}
func (h *userHandler) user(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	user, err := h.service.User(r.Context(), bearer(r), id)
	if failed(w, r, err) {
		return
	}
	write(w, 200, dtos.User(*user))
}
func (h *userHandler) createUser(w http.ResponseWriter, r *http.Request) {
	request, ok := requestBody(w, r, dtos.ParseCreateUser)
	if !ok {
		return
	}
	user, err := h.credentials.Create(r.Context(), bearer(r), request)
	if failed(w, r, err) {
		return
	}
	audit(r.Context(), "users.created", false)
	write(w, 201, dtos.User(*user))
}
func (h *userHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	request, ok := requestBody(w, r, dtos.ParseUpdateUser)
	if !ok {
		return
	}
	user, err := h.service.UpdateUser(r.Context(), bearer(r), id, request)
	if failed(w, r, err) {
		return
	}
	audit(r.Context(), "users.updated", false)
	write(w, 200, dtos.User(*user))
}
func (h *userHandler) password(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	reason, ok := requestBody(w, r, dtos.ParseResetReason)
	if !ok {
		return
	}
	if failed(w, r, h.credentials.Reset(r.Context(), bearer(r), id, reason)) {
		return
	}
	audit(r.Context(), "users.password_replaced", false)
	w.WriteHeader(204)
}
func (h *userHandler) deactivate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	user, err := h.service.Deactivate(r.Context(), bearer(r), id)
	if failed(w, r, err) {
		return
	}
	audit(r.Context(), "users.deactivated", true)
	write(w, 200, dtos.User(*user))
}
func (h *userHandler) online(w http.ResponseWriter, r *http.Request) {
	records, err := h.service.Online(r.Context(), bearer(r))
	if failed(w, r, err) {
		return
	}
	write(w, 200, map[string]any{"online_count": len(records), "items": dtos.Activities(records)})
}
