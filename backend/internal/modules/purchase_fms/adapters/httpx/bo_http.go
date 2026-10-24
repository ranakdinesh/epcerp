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

type bOHandler struct{ s *application.BOService }

func bOFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_bo.operation.failed", "Purchase BO list operation failed")
}

func (h bOHandler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), bearer(r), chi.URLParam(r, "project"))
	if !bOFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h bOHandler) history(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.History(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"))
	if !bOFail(w, r, e) {
		write(w, 200, v)
	}
}

// BO drafts accept 100 rows with bounded notes; 512 KiB covers their validated payload.
func bODecode(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 512<<10))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			bOFail(w, r, domain.BOInvalid("", "Unable to read input"))
		}
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		bOFail(w, r, domain.BOInvalid("", "Expected a JSON object"))
		return false
	}
	for _, key := range []string{"version", "reason", "lines"} {
		if value, ok := object[key]; ok && strings.TrimSpace(string(value)) == "null" {
			bOFail(w, r, domain.BOInvalid(key, "This field must not be null"))
			return false
		}
	}
	if rawLines, ok := object["lines"]; ok {
		var lines []map[string]json.RawMessage
		if json.Unmarshal(rawLines, &lines) != nil {
			bOFail(w, r, domain.BOInvalid("lines", "Expected item row objects"))
			return false
		}
		for _, line := range lines {
			if line == nil {
				bOFail(w, r, domain.BOInvalid("lines", "Expected item row objects"))
				return false
			}
			for _, key := range []string{"id", "reference", "notes"} {
				if value, ok := line[key]; ok && strings.TrimSpace(string(value)) == "null" {
					bOFail(w, r, domain.BOInvalid(key, "This field must not be null"))
					return false
				}
			}
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(target); e != nil {
		bOFail(w, r, domain.BOInvalid("", "Invalid BO input"))
		return false
	}
	if e = decoder.Decode(&struct{}{}); e != io.EOF {
		bOFail(w, r, domain.BOInvalid("", "Expected one JSON object"))
		return false
	}
	return true
}
func (h bOHandler) save(w http.ResponseWriter, r *http.Request) {
	var input domain.BOSave
	if !bODecode(w, r, &input) {
		return
	}
	v, e := h.s.Save(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"), input)
	if !bOFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_bo.saved").Str("bo_id", v.ID).Int32("version", v.Version).Msg("Purchase BO list committed")
		write(w, 200, v)
	}
}
func (h bOHandler) change(w http.ResponseWriter, r *http.Request) {
	var input domain.BOChange
	if !bODecode(w, r, &input) {
		return
	}
	action := "complete"
	if strings.HasSuffix(r.URL.Path, "/reopen") {
		action = "reopen"
	}
	v, e := h.s.Change(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"), action, input)
	if !bOFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_bo."+action).Str("bo_id", v.ID).Int32("version", v.Version).Msg("Purchase BO transition committed")
		write(w, 200, v)
	}
}

func (h bOHandler) pdf(w http.ResponseWriter, r *http.Request) {
	data, code, e := h.s.Print(r.Context(), bearer(r), chi.URLParam(r, "project"))
	if bOFail(w, r, e) {
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `attachment; filename="purchase-items-`+strings.Map(func(c rune) rune {
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '-' {
			return c
		}
		return -1
	}, code)+`.pdf"`)
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func (h bOHandler) registerChange(w http.ResponseWriter, r *http.Request) {
	var in domain.BORegisterChange
	if !bODecode(w, r, &in) {
		return
	}
	action := "complete"
	if strings.HasSuffix(r.URL.Path, "register-reopen") {
		action = "reopen"
	}
	v, e := h.s.ChangeRegister(r.Context(), bearer(r), chi.URLParam(r, "project"), action, in)
	if !bOFail(w, r, e) {
		write(w, 200, v)
	}
}
