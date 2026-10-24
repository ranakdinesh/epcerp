package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type workflowHandler struct{ s *application.WorkflowService }

func workflowFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_workflow.operation.failed", "Purchase package operation failed")
}

func workflowParams(r *http.Request) (string, string) {
	return chi.URLParam(r, "project"), chi.URLParam(r, "package")
}
func workflowDecode(w http.ResponseWriter, r *http.Request, out any) bool {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			write(w, 422, map[string]string{"detail": "Invalid command body"})
		}
		return false
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		write(w, 422, map[string]string{"detail": "Expected a command object"})
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(out); e != nil {
		write(w, 422, map[string]string{"detail": "Use the documented command fields only"})
		return false
	}
	if e = d.Decode(&struct{}{}); e != io.EOF {
		write(w, 422, map[string]string{"detail": "Expected one command object"})
		return false
	}
	return true
}
func workflowNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) > 0 {
		workflowFail(w, r, domain.WorkflowInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h workflowHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.WorkflowPageFilter{}
	for key, values := range q {
		if len(values) != 1 {
			workflowFail(w, r, domain.WorkflowInvalid("query", "Use one value per parameter"))
			return
		}
		switch key {
		case "page", "page_size":
			v, e := strconv.ParseInt(values[0], 10, 32)
			if e != nil || v <= 0 {
				workflowFail(w, r, domain.WorkflowInvalid(key, "Use a positive integer"))
				return
			}
			if key == "page" {
				f.Page = int32(v)
			} else {
				f.Size = int32(v)
			}
		case "search":
			f.Search = values[0]
		case "sort":
			f.Sort = values[0]
		case "direction":
			f.Direction = values[0]
		default:
			workflowFail(w, r, domain.WorkflowInvalid("query", "Unsupported package query parameter"))
			return
		}
	}
	var e error
	f, e = f.Validate()
	if workflowFail(w, r, e) {
		return
	}
	p, _ := workflowParams(r)
	v, e := h.s.Listing(r.Context(), bearer(r), p, f)
	if !workflowFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h workflowHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !workflowNoQuery(w, r) {
		return
	}
	p, id := workflowParams(r)
	v, e := h.s.Workspace(r.Context(), bearer(r), p, id)
	if !workflowFail(w, r, e) {
		write(w, 200, v)
	}
}
func workflowCommitted(r *http.Request, action string) {
	logging.Ctx(r.Context()).Info().Str("event", "purchase_workflow.command.committed").Str("action", action).Msg("Purchase package command committed")
}
func (h workflowHandler) create(w http.ResponseWriter, r *http.Request) {
	var q domain.WorkflowCreate
	if !workflowDecode(w, r, &q) {
		return
	}
	p, _ := workflowParams(r)
	v, e := h.s.Create(r.Context(), bearer(r), p, q)
	if !workflowFail(w, r, e) {
		workflowCommitted(r, "create")
		write(w, 201, v)
	}
}
func (h workflowHandler) assign(w http.ResponseWriter, r *http.Request) {
	var q domain.WorkflowAssign
	if !workflowDecode(w, r, &q) {
		return
	}
	p, id := workflowParams(r)
	v, e := h.s.Assign(r.Context(), bearer(r), p, id, q)
	if !workflowFail(w, r, e) {
		workflowCommitted(r, "assign")
		write(w, 200, v)
	}
}
func (h workflowHandler) Command(w http.ResponseWriter, r *http.Request) {
	var q domain.WorkflowCommand
	if !workflowDecode(w, r, &q) {
		return
	}
	number, e := strconv.Atoi(chi.URLParam(r, "step"))
	if e != nil {
		workflowFail(w, r, domain.WorkflowInvalid("step", "Use a numbered preparation step"))
		return
	}
	p, id := workflowParams(r)
	v, e := h.s.Command(r.Context(), bearer(r), p, id, number, q)
	if !workflowFail(w, r, e) {
		workflowCommitted(r, q.Action)
		write(w, 200, v)
	}
}
