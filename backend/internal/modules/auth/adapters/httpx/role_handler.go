package httpx

import (
	"net/http"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
)

type roleHandler struct{ service *application.RoleService }

func (h *roleHandler) roles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.Roles(r.Context(), bearer(r))
	if failed(w, r, err) {
		return
	}
	items := []dtos.RoleResponse{}
	for _, role := range roles {
		items = append(items, dtos.Role(role))
	}
	write(w, 200, items)
}
func (h *roleHandler) permissions(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Permissions(r.Context(), bearer(r))
	if failed(w, r, err) {
		return
	}
	write(w, 200, dtos.Permissions(items))
}
func (h *roleHandler) createRole(w http.ResponseWriter, r *http.Request) {
	request, ok := requestBody(w, r, dtos.ParseCreateRole)
	if !ok {
		return
	}
	role, err := h.service.CreateRole(r.Context(), bearer(r), request)
	if failed(w, r, err) {
		return
	}
	audit(r.Context(), "roles.created", false)
	write(w, 201, dtos.Role(*role))
}
func (h *roleHandler) updateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "role_id")
	if !ok {
		return
	}
	request, ok := requestBody(w, r, dtos.ParseUpdateRole)
	if !ok {
		return
	}
	role, err := h.service.UpdateRole(r.Context(), bearer(r), id, request)
	if failed(w, r, err) {
		return
	}
	audit(r.Context(), "roles.updated", false)
	write(w, 200, dtos.Role(*role))
}
func (h *roleHandler) finance(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.FinancePresets(r.Context(), bearer(r))
	if failed(w, r, err) {
		return
	}
	items := []dtos.RoleResponse{}
	for _, role := range roles {
		items = append(items, dtos.Role(role))
	}
	audit(r.Context(), "roles.finance.installed", false)
	write(w, 200, items)
}
