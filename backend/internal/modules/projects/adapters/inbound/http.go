package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/application"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type handler struct{ s *application.Service }

func Register(r chi.Router, s *application.Service) {
	h := handler{s}
	r.Post("/api/v1/projects/schedule-preview", h.preview)
	r.Get("/api/v1/projects", h.list)
	r.Post("/api/v1/projects", h.create)
	r.Get("/api/v1/projects/assignment-options", h.options)
	r.Get("/api/v1/projects/{id}", h.get)
	r.Patch("/api/v1/projects/{id}/manager", h.transfer)
	r.Get("/api/v1/projects/{id}/manager-history", h.history)
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, r *http.Request, e error) bool {
	if e == nil {
		return false
	}
	status := 500
	detail := "Internal server error"
	var invalid *domain.Invalid
	if errors.As(e, &invalid) {
		write(w, 422, map[string]any{"detail": []map[string]any{{"type": "value_error", "loc": []string{"body", invalid.Field}, "msg": invalid.Message}}})
		return true
	}
	switch {
	case errors.Is(e, domain.ErrUnauthorized):
		status = 401
		detail = e.Error()
		w.Header().Set("WWW-Authenticate", "Bearer")
	case errors.Is(e, domain.ErrForbidden):
		status = 403
		detail = e.Error()
	case errors.Is(e, domain.ErrMissing):
		status = 404
		detail = e.Error()
	case errors.Is(e, domain.ErrConflict), errors.Is(e, domain.ErrStale):
		status = 409
		detail = e.Error()
	default:
		logging.Ctx(r.Context()).Error().Str("event", "projects.operation.failed").Msg("Project operation failed")
	}
	write(w, status, map[string]string{"detail": detail})
	return true
}
func token(r *http.Request) string {
	p := strings.Fields(r.Header.Get("Authorization"))
	if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
		return ""
	}
	return p[1]
}
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func view(p domain.Project) map[string]any {
	v := captureView(p.Capture)
	for k, value := range map[string]any{"id": p.ID, "project_number": p.Number, "name": p.Name, "client_id": p.ClientID, "project_manager_id": p.ManagerID, "version": p.Version, "is_active": p.Active, "lifecycle_status": p.Lifecycle, "created_at": p.CreatedAt.UTC().Format(time.RFC3339Nano), "updated_at": p.UpdatedAt.UTC().Format(time.RFC3339Nano), "created_by_id": nullable(p.CreatedBy), "updated_by_id": nullable(p.UpdatedBy), "procurement_executive_id": nullable(p.ProcurementExecutiveID)} {
		v[k] = value
	}
	return v
}
func parse(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct := r.Header.Get("Content-Type"); !strings.EqualFold(strings.TrimSpace(strings.Split(ct, ";")[0]), "application/json") {
		write(w, 415, map[string]string{"detail": "Use application/json"})
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	e := decoder.Decode(v)
	if e == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			e = errors.New("single JSON object required")
		}
	}
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			fail(w, r, &domain.Invalid{Field: "", Message: "Use a single JSON object with only supported fields"})
		}
		return false
	}
	return true
}
func audit(r *http.Request, p domain.Project, action string) {
	logging.Ctx(r.Context()).Info().Str("event", action).Str("project_id", p.ID).Int32("version", p.Version).Str("actor_id", p.UpdatedBy).Msg("Project ownership change committed")
}
func (h handler) list(w http.ResponseWriter, r *http.Request) {
	f := domain.Filter{Page: 1, PageSize: 50}
	for key, values := range r.URL.Query() {
		if len(values) != 1 {
			fail(w, r, &domain.Invalid{Field: "query", Message: "Use single query values"})
			return
		}
		v := values[0]
		switch key {
		case "search":
			f.Search = v
		case "page", "page_size":
			n, e := strconv.ParseInt(v, 10, 32)
			if e != nil {
				fail(w, r, &domain.Invalid{Field: key, Message: "Use an integer"})
				return
			}
			if key == "page" {
				f.Page = int32(n)
			} else {
				f.PageSize = int32(n)
			}
		default:
			fail(w, r, &domain.Invalid{Field: key, Message: "Unknown query parameter"})
			return
		}
	}
	page, e := h.s.List(r.Context(), token(r), f)
	if fail(w, r, e) {
		return
	}
	items := []map[string]any{}
	for _, p := range page.Items {
		items = append(items, view(p))
	}
	write(w, 200, map[string]any{"items": items, "total": page.Total, "page": page.Page, "page_size": page.PageSize})
}
func (h handler) get(w http.ResponseWriter, r *http.Request) {
	p, e := h.s.Get(r.Context(), token(r), chi.URLParam(r, "id"))
	if fail(w, r, e) {
		return
	}
	write(w, 200, view(*p))
}
func (h handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name                   string `json:"name"`
		ClientID               string `json:"client_id"`
		ManagerID              string `json:"project_manager_id"`
		ProcurementExecutiveID string `json:"procurement_executive_id"`
		CaptureRequest
	}
	if !parse(w, r, &in) {
		return
	}
	p, e := h.s.Create(r.Context(), token(r), domain.Create{Name: in.Name, ClientID: in.ClientID, ManagerID: in.ManagerID, ProcurementExecutiveID: in.ProcurementExecutiveID, Capture: in.capture()})
	if fail(w, r, e) {
		return
	}
	audit(r, *p, "projects.created")
	write(w, 201, view(*p))
}
func (h handler) transfer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ManagerID string `json:"project_manager_id"`
		Reason    string `json:"reason"`
		Version   int32  `json:"expected_version"`
	}
	if !parse(w, r, &in) {
		return
	}
	p, e := h.s.Transfer(r.Context(), token(r), chi.URLParam(r, "id"), domain.Transfer{ManagerID: in.ManagerID, Reason: in.Reason, ExpectedVersion: in.Version})
	if fail(w, r, e) {
		return
	}
	audit(r, *p, "projects.manager_reassigned")
	write(w, 200, view(*p))
}
func (h handler) options(w http.ResponseWriter, r *http.Request) {
	o, e := h.s.Options(r.Context(), token(r))
	if fail(w, r, e) {
		return
	}
	managers := []map[string]string{}
	clients := []map[string]string{}
	for _, v := range o.Managers {
		managers = append(managers, map[string]string{"id": v.ID, "name": v.Name})
	}
	for _, v := range o.Clients {
		clients = append(clients, map[string]string{"id": v.ID, "name": v.Name, "code": v.Code})
	}
	procurement := []map[string]string{}
	for _, v := range o.Procurement {
		procurement = append(procurement, map[string]string{"id": v.ID, "name": v.Name})
	}
	write(w, 200, map[string]any{"managers": managers, "clients": clients, "procurement_executives": procurement})
}
func (h handler) history(w http.ResponseWriter, r *http.Request) {
	rows, e := h.s.History(r.Context(), token(r), chi.URLParam(r, "id"))
	if fail(w, r, e) {
		return
	}
	items := []map[string]any{}
	for _, v := range rows {
		items = append(items, map[string]any{"id": v.ID, "project_id": v.ProjectID, "previous_manager_id": nullable(v.BeforeManagerID), "project_manager_id": v.ManagerID, "project_version": v.Version, "reason": v.Reason, "created_by_id": nullable(v.ActorID), "created_at": v.OccurredAt.UTC().Format(time.RFC3339Nano)})
	}
	write(w, 200, items)
}

func (h handler) preview(w http.ResponseWriter, r *http.Request) {
	var in CaptureRequest
	if !parse(w, r, &in) {
		return
	}
	result, e := h.s.Preview(r.Context(), token(r), in.capture())
	if fail(w, r, e) {
		return
	}
	write(w, 200, result)
}
