package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type indentsHandler struct{ s *application.IndentsService }

func (h indentsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		indentsFail(w, r, domain.IndentsInvalid("query", "Indent workspace does not accept query parameters"))
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"))
	if !indentsFail(w, r, e) {
		write(w, 200, v)
	}
}

func indentsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_indents.operation.failed", "Purchase indent operation failed")
}

func (h indentsHandler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), bearer(r), chi.URLParam(r, "project"))
	if !indentsFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h indentsHandler) history(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.History(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"))
	if !indentsFail(w, r, e) {
		write(w, 200, v)
	}
}

func indentsDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 512<<10))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			indentsFail(w, r, domain.IndentsInvalid("", "Unable to read input"))
		}
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		indentsFail(w, r, domain.IndentsInvalid("", "Expected a JSON object"))
		return false
	}
	for key, value := range object {
		if strings.TrimSpace(string(value)) == "null" {
			indentsFail(w, r, domain.IndentsInvalid(key, "This field must not be null"))
			return false
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(target); e != nil {
		indentsFail(w, r, domain.IndentsInvalid("", "Invalid indent input"))
		return false
	}
	if e = decoder.Decode(&struct{}{}); e != io.EOF {
		indentsFail(w, r, domain.IndentsInvalid("", "Expected one JSON object"))
		return false
	}
	return true
}
func (h indentsHandler) save(w http.ResponseWriter, r *http.Request) {
	input := domain.IndentsSave{Version: 1}
	if !indentsDecode(w, r, &input) {
		return
	}
	v, e := h.s.Save(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"), input)
	if !indentsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_indents.saved").Str("indent_id", v.ID).Int32("version", v.Version).Msg("Purchase indent committed")
		status := 200
		if r.Method == "POST" {
			status = 201
		}
		write(w, status, v)
	}
}
func (h indentsHandler) action(w http.ResponseWriter, r *http.Request) {
	var input domain.IndentsAction
	if !indentsDecode(w, r, &input) {
		return
	}
	v, e := h.s.Transition(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"), input)
	if !indentsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_indents.transition").Str("action", input.Action).Str("indent_id", v.ID).Int32("version", v.Version).Msg("Purchase indent transition committed")
		write(w, 200, v)
	}
}
