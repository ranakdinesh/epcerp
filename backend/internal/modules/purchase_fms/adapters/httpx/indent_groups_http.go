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

type indentGroupsHandler struct {
	s *application.IndentGroupsService
}

func indentGroupsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_indent_groups.operation.failed", "Grouped purchase indent operation failed")
}

func (h indentGroupsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "bo"))
	if !indentGroupsFail(w, r, e) {
		write(w, 200, v)
	}
}
func indentGroupsDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			indentGroupsFail(w, r, domain.IndentGroupsInvalid("", "Unable to read input"))
		}
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		indentGroupsFail(w, r, domain.IndentGroupsInvalid("", "Expected a JSON object"))
		return false
	}
	for key, value := range object {
		if key != "group_id" && key != "line_id" && key != "vendor_id" && key != "sent_date" && strings.TrimSpace(string(value)) == "null" {
			indentGroupsFail(w, r, domain.IndentGroupsInvalid(key, "This field must not be null"))
			return false
		}
	}
	if raw, ok := object["groups"]; ok {
		var groups []map[string]json.RawMessage
		if json.Unmarshal(raw, &groups) != nil {
			indentGroupsFail(w, r, domain.IndentGroupsInvalid("groups", "Expected group objects"))
			return false
		}
		for _, g := range groups {
			if g == nil {
				indentGroupsFail(w, r, domain.IndentGroupsInvalid("groups", "Expected group objects"))
				return false
			}
			for key, value := range g {
				if strings.TrimSpace(string(value)) == "null" {
					indentGroupsFail(w, r, domain.IndentGroupsInvalid(key, "This field must not be null"))
					return false
				}
			}
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(target); e != nil {
		indentGroupsFail(w, r, domain.IndentGroupsInvalid("", "Invalid group command"))
		return false
	}
	if e = decoder.Decode(&struct{}{}); e != io.EOF {
		indentGroupsFail(w, r, domain.IndentGroupsInvalid("", "Expected one JSON object"))
		return false
	}
	return true
}
func (h indentGroupsHandler) execute(w http.ResponseWriter, r *http.Request) {
	var input domain.IndentGroupsCommand
	if !indentGroupsDecode(w, r, &input) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "bo"), input)
	if !indentGroupsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_indent_groups.command").Str("action", input.Action).Str("bo_id", chi.URLParam(r, "bo")).Msg("Grouped purchase indent command committed")
		write(w, 200, v)
	}
}
