package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type workspaceHandler struct{ s *application.WorkspaceService }

func workspaceFail(w http.ResponseWriter, r *http.Request, e error) bool {
	if e == nil {
		return false
	}
	var invalid *domain.WorkspaceInvalid
	if errors.As(e, &invalid) {
		write(w, 422, map[string]any{"detail": []map[string]any{{"loc": []string{"body", invalid.Field}, "msg": invalid.Message}}})
		return true
	}
	status := 500
	detail := "Purchase operation failed"
	switch {
	case errors.Is(e, domain.WorkspaceUnauthorized):
		status = 401
		detail = e.Error()
		w.Header().Set("WWW-Authenticate", "Bearer")
	case errors.Is(e, domain.WorkspaceForbidden):
		status = 403
		detail = e.Error()
	case errors.Is(e, domain.WorkspaceMissing):
		status = 404
		detail = e.Error()
	case errors.Is(e, domain.WorkspaceStale):
		status = 409
		detail = e.Error()
	default:
		logging.Ctx(r.Context()).Error().Str("event", "purchase_fms.operation.failed").Msg("Purchase operation failed")
	}
	write(w, status, map[string]string{"detail": detail})
	return true
}
func workspaceNullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func workspaceWorkspace(w domain.Workspace) map[string]any {
	return map[string]any{"project_id": w.ProjectID, "purchase_manager_id": workspaceNullable(w.ManagerID), "version": w.Version, "is_active": w.Active, "updated_at": w.UpdatedAt, "updated_by_id": workspaceNullable(w.UpdatedBy)}
}
func workspaceRow(v domain.WorkspaceRow) map[string]any {
	return map[string]any{"id": v.Project.ID, "project_number": v.Project.Number, "name": v.Project.Name, "project_manager_id": v.Project.ManagerID, "lifecycle_status": v.Project.Lifecycle, "is_active": v.Project.Active, "purchase_manager_id": workspaceNullable(v.Workspace.ManagerID), "ownership_version": v.Workspace.Version, "can_assign": v.CanAssign, "progress_percent": v.Progress.Percent, "progress_status": "purchase_stages_credit", "recorded_packages": nil, "completed_steps": v.Progress.Completed, "next_step": v.Progress.Next, "pending_approval_groups": v.Progress.Pending, "returned_groups": v.Progress.Returned, "approved_groups": v.Progress.Approved}
}
func (h workspaceHandler) list(w http.ResponseWriter, r *http.Request) {
	f := domain.WorkspaceFilter{Page: 1, Size: 20}
	for k, vs := range r.URL.Query() {
		if len(vs) != 1 {
			workspaceFail(w, r, &domain.WorkspaceInvalid{Field: "query", Message: "Use single query values"})
			return
		}
		switch k {
		case "page", "page_size":
			n, e := strconv.ParseInt(vs[0], 10, 32)
			if e != nil {
				workspaceFail(w, r, &domain.WorkspaceInvalid{Field: "query", Message: "Use integer pagination"})
				return
			}
			if k == "page" {
				f.Page = int32(n)
			} else {
				f.Size = int32(n)
			}
		case "sort":
			f.Sort = vs[0]
		case "direction":
			f.Direction = vs[0]
		case "search":
			f.Search = vs[0]
		default:
			workspaceFail(w, r, &domain.WorkspaceInvalid{Field: "query", Message: "Unsupported query parameter"})
			return
		}
	}
	p, e := h.s.Dashboard(r.Context(), bearer(r), f)
	if workspaceFail(w, r, e) {
		return
	}
	items := []map[string]any{}
	for _, v := range p.Rows {
		items = append(items, workspaceRow(v))
	}
	write(w, 200, map[string]any{"items": items, "total": p.Total, "page": p.Page, "page_size": p.Size, "portfolio": p.Portfolio})
}
func (h workspaceHandler) context(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Context(r.Context(), bearer(r), chi.URLParam(r, "id"))
	if workspaceFail(w, r, e) {
		return
	}
	history := []map[string]any{}
	for _, a := range v.History {
		history = append(history, map[string]any{"id": a.ID, "previous_manager_id": workspaceNullable(a.PreviousID), "purchase_manager_id": a.ManagerID, "created_by_id": workspaceNullable(a.ActorID), "reason": a.Reason, "version": a.Version, "created_at": a.CreatedAt})
	}
	write(w, 200, map[string]any{"project": workspaceRow(v.Row), "history": history})
}
func (h workspaceHandler) options(w http.ResponseWriter, r *http.Request) {
	cs, e := h.s.Options(r.Context(), bearer(r), chi.URLParam(r, "id"))
	if workspaceFail(w, r, e) {
		return
	}
	items := []map[string]string{}
	for _, c := range cs {
		items = append(items, map[string]string{"id": c.ID, "name": c.Name})
	}
	write(w, 200, items)
}
func (h workspaceHandler) assign(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]), "application/json") {
		write(w, 415, map[string]string{"detail": "Use application/json"})
		return
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	var payload struct {
		ManagerID string `json:"purchase_manager_id"`
		Version   *int32 `json:"expected_version"`
		Reason    string `json:"reason"`
	}
	e := d.Decode(&payload)
	if e == nil {
		var extra any
		if d.Decode(&extra) != io.EOF {
			e = errors.New("single JSON object required")
		}
	}
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request too large"})
		} else {
			workspaceFail(w, r, &domain.WorkspaceInvalid{Field: "", Message: "Use a single JSON object with supported fields"})
		}
		return
	}
	if payload.Version == nil {
		workspaceFail(w, r, &domain.WorkspaceInvalid{Field: "expected_version", Message: "Use the current ownership version"})
		return
	}
	input := domain.WorkspaceChange{ManagerID: payload.ManagerID, Version: *payload.Version, Reason: payload.Reason}
	v, e := h.s.Assign(r.Context(), bearer(r), chi.URLParam(r, "id"), input)
	if workspaceFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "purchase_fms.owner_assigned").Str("project_id", v.ProjectID).Int32("version", v.Version).Str("actor_id", v.UpdatedBy).Msg("Purchase ownership change committed")
	write(w, 200, workspaceWorkspace(v))
}
