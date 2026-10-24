package inbound

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpinput"
	packageContract "github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	readingContract "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type deliveryDTO struct {
	Action string  `json:"source_action_id"`
	Notice *string `json:"notice_id"`
	Email  *string `json:"email_job_id"`
	State  string  `json:"email_state"`
}
type capturedDTO struct {
	ManagerName      string        `json:"manager_display_name"`
	CanEdit          bool          `json:"can_edit"`
	CanAssign        bool          `json:"can_assign"`
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Version          int64         `json:"business_version"`
	Content          int64         `json:"content_revision"`
	Assignment       int64         `json:"assignment_version"`
	Manager          string        `json:"manager_id"`
	Project          *string       `json:"project_id"`
	Estimate         *string       `json:"estimate_id"`
	ProvisionVersion int64         `json:"provisioning_version"`
	State            string        `json:"provisioning_state"`
	Blocker          *string       `json:"blocker_code"`
	Deliveries       []deliveryDTO `json:"deliveries"`
}

func maybe(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
func projectCapture(v domain.Captured) capturedDTO {
	d := capturedDTO{ManagerName: v.ManagerName, CanEdit: v.CanEdit, CanAssign: v.CanAssign, ID: v.Record.ID, Name: v.Record.Name, Version: v.Record.BusinessVersion, Content: v.Record.ContentRevision, Assignment: v.Record.AssignmentVersion, Manager: v.Record.ManagerID, Project: maybe(v.ProjectID), Estimate: maybe(v.EstimateID), ProvisionVersion: v.ProvisionVersion, State: v.ProvisionState, Blocker: maybe(v.Blocker), Deliveries: []deliveryDTO{}}
	for _, a := range v.Deliveries {
		d.Deliveries = append(d.Deliveries, deliveryDTO{a.ActionID, maybe(a.NoticeID), maybe(a.EmailJobID), a.EmailState})
	}
	return d
}
func captureHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}
func captureFailure(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	status, code, detail := 503, "dependency_unavailable", "Cannot complete Tender capture operation"
	var d *domain.Error
	var c *domain.CaptureError
	var p *packageContract.Error
	var input *httpinput.Error
	var readingError *readingContract.Error
	switch {
	case errors.As(e, &d):
		status, detail = d.Status, d.Detail
		code = map[int]string{401: "unauthenticated", 403: "forbidden", 404: "not_found", 409: "stale_version", 422: "validation_failed"}[status]
	case errors.As(e, &c):
		status, code, detail = c.Status, c.Code, c.Detail
	case errors.As(e, &p):
		status, code, detail = p.Status, p.Code, p.Detail
	case errors.As(e, &readingError):
		status, code, detail = readingError.Status, "reading_error", readingError.Detail
	case errors.As(e, &input):
		status, code, detail = input.Status, "invalid_request", input.Detail
	}
	var bytes [16]byte
	_, _ = rand.Read(bytes[:])
	captureHeaders(w)
	if status == 429 {
		w.Header().Set("Retry-After", "60")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "detail": detail, "request_id": hex.EncodeToString(bytes[:]), "retryable": status == 503 || status == 429})
	return true
}
func captureToken(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
func sourceURL(s *string) error {
	if s == nil {
		return nil
	}
	u, e := url.Parse(*s)
	if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || len(*s) > 2048 {
		return domain.Invalid("Use an HTTP or HTTPS source URL without credentials")
	}
	return nil
}
func respondCapture(w http.ResponseWriter, status int, out domain.Captured, replay bool) {
	captureHeaders(w)
	if replay {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.Header().Set("Location", "/api/v1/tender-fms/records/"+out.Record.ID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(projectCapture(out))
}
func RegisterCapture(r chi.Router, s *application.CaptureService) {
	r.Post("/api/v1/tender-fms/records", func(w http.ResponseWriter, r *http.Request) {
		t := captureToken(r)
		if captureFailure(w, s.Authorize(r.Context(), t, true)) {
			return
		}
		var body struct {
			Name      string  `json:"name"`
			Receipt   string  `json:"receipt_id"`
			Version   int64   `json:"expected_receipt_version"`
			Manager   string  `json:"manager_id"`
			Reference *string `json:"external_reference"`
			URL       *string `json:"source_url"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 32<<10, &body)) || captureFailure(w, sourceURL(body.URL)) {
			return
		}
		out, replay, e := s.Capture(r.Context(), t, r.Header.Get("Idempotency-Key"), domain.CaptureInput{Name: body.Name, ReceiptID: body.Receipt, ReceiptVersion: body.Version, ManagerID: body.Manager, ExternalReference: body.Reference, SourceURL: body.URL})
		if captureFailure(w, e) {
			return
		}
		respondCapture(w, 202, out, replay)
	})
	r.Get("/api/v1/tender-fms/records", func(w http.ResponseWriter, r *http.Request) {
		if captureFailure(w, s.Authorize(r.Context(), captureToken(r), false)) {
			return
		}
		q, e := url.ParseQuery(r.URL.RawQuery)
		if e != nil {
			captureFailure(w, domain.Invalid("Malformed query parameters"))
			return
		}
		page, size := int64(1), int64(20)
		for key, v := range q {
			if len(v) != 1 || (key != "page" && key != "page_size" && key != "search" && key != "sort" && key != "direction") {
				captureFailure(w, domain.Invalid("Unsupported or repeated register query field"))
				return
			}
		}
		if q.Has("page") {
			page, e = strconv.ParseInt(q.Get("page"), 10, 32)
			if e != nil {
				captureFailure(w, domain.Invalid("Invalid page"))
				return
			}
		}
		if q.Has("page_size") {
			size, e = strconv.ParseInt(q.Get("page_size"), 10, 32)
			if e != nil {
				captureFailure(w, domain.Invalid("Invalid page size"))
				return
			}
		}
		sort, direction := q.Get("sort"), q.Get("direction")
		if sort == "" {
			sort = "created"
		}
		if direction == "" {
			direction = "desc"
		}
		if direction != "asc" && direction != "desc" {
			captureFailure(w, domain.Invalid("Invalid sort direction"))
			return
		}
		rows, total, e := s.ListFiltered(r.Context(), captureToken(r), int32(page), int32(size), q.Get("search"), sort, direction == "desc")
		if captureFailure(w, e) {
			return
		}
		items := []capturedDTO{}
		for _, v := range rows {
			items = append(items, projectCapture(v))
		}
		captureHeaders(w)
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total, "page": page, "page_size": size})
	})
	r.Get("/api/v1/tender-fms/records/{tender_id}", func(w http.ResponseWriter, r *http.Request) {
		out, e := s.Get(r.Context(), captureToken(r), chi.URLParam(r, "tender_id"))
		if captureFailure(w, e) {
			return
		}
		respondCapture(w, 200, out, false)
	})
	r.Post("/api/v1/tender-fms/records/{tender_id}/manager-assignments", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "tender_id")
		if _, e := s.Get(r.Context(), captureToken(r), id); captureFailure(w, e) {
			return
		}
		var body struct {
			Version    int64  `json:"expected_version"`
			Assignment int64  `json:"expected_assignment_version"`
			Manager    string `json:"manager_id"`
			Reason     string `json:"reason"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 32<<10, &body)) {
			return
		}
		out, replay, e := s.Change(r.Context(), captureToken(r), id, r.Header.Get("Idempotency-Key"), domain.ChangeManager{Version: body.Version, AssignmentVersion: body.Assignment, ManagerID: body.Manager, Reason: body.Reason})
		if captureFailure(w, e) {
			return
		}
		respondCapture(w, 200, out, replay)
	})
	r.Patch("/api/v1/tender-fms/records/{tender_id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "tender_id")
		if _, e := s.Get(r.Context(), captureToken(r), id); captureFailure(w, e) {
			return
		}
		var body struct {
			Version   int64           `json:"expected_version"`
			Name      string          `json:"name"`
			Reference json.RawMessage `json:"external_reference"`
			URL       json.RawMessage `json:"source_url"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 32<<10, &body)) {
			return
		}
		var reference, urlValue *string
		if len(body.Reference) > 0 {
			if err := json.Unmarshal(body.Reference, &reference); err != nil {
				captureFailure(w, domain.Invalid("External reference must be a string or null"))
				return
			}
		}
		if len(body.URL) > 0 {
			if err := json.Unmarshal(body.URL, &urlValue); err != nil {
				captureFailure(w, domain.Invalid("Source URL must be a string or null"))
				return
			}
		}
		if captureFailure(w, sourceURL(urlValue)) {
			return
		}
		out, replay, e := s.Patch(r.Context(), captureToken(r), id, r.Header.Get("Idempotency-Key"), domain.Patch{Version: body.Version, Name: body.Name, ExternalReference: reference, SourceURL: urlValue, SetReference: len(body.Reference) > 0, SetURL: len(body.URL) > 0})
		if captureFailure(w, e) {
			return
		}
		respondCapture(w, 200, out, replay)
	})
}
