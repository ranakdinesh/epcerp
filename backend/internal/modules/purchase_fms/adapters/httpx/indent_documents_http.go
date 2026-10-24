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

type indentDocumentsHandler struct {
	s *application.IndentDocumentsService
}

func indentDocumentsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_indent_documents.operation.failed", "Vendor indent operation failed")
}

func (h indentDocumentsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "bo"))
	if !indentDocumentsFail(w, r, e) {
		write(w, 200, v)
	}
}
func indentDocumentsDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			indentDocumentsFail(w, r, domain.IndentDocumentsInvalid("", "Unable to read input"))
		}
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		indentDocumentsFail(w, r, domain.IndentDocumentsInvalid("", "Expected a JSON object"))
		return false
	}
	for key, value := range object {
		if key != "group_id" && key != "document_id" && key != "vendor_id" && key != "sent_date" && strings.TrimSpace(string(value)) == "null" {
			indentDocumentsFail(w, r, domain.IndentDocumentsInvalid(key, "This field must not be null"))
			return false
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(target); e != nil {
		indentDocumentsFail(w, r, domain.IndentDocumentsInvalid("", "Invalid indent document command"))
		return false
	}
	if e = decoder.Decode(&struct{}{}); e != io.EOF {
		indentDocumentsFail(w, r, domain.IndentDocumentsInvalid("", "Expected one JSON object"))
		return false
	}
	return true
}
func (h indentDocumentsHandler) execute(w http.ResponseWriter, r *http.Request) {
	var input domain.IndentDocumentsCommand
	if !indentDocumentsDecode(w, r, &input) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "bo"), input)
	if !indentDocumentsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_indent_documents.command").Str("action", input.Action).Str("bo_id", chi.URLParam(r, "bo")).Msg("Vendor indent command committed")
		write(w, 200, v)
	}
}

func (h indentDocumentsHandler) notifications(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Notifications(r.Context(), bearer(r))
	if !indentDocumentsFail(w, r, e) {
		write(w, 200, v)
	}
}
