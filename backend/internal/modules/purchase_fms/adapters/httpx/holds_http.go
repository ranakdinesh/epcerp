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

type holdsHandler struct{ s *application.HoldsService }

func holdsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_holds.operation.failed", "Purchase hold operation failed")
}

func (h holdsHandler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), bearer(r), chi.URLParam(r, "project"))
	if !holdsFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h holdsHandler) history(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.History(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"))
	if !holdsFail(w, r, e) {
		write(w, 200, v)
	}
}

func holdsDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			holdsFail(w, r, domain.HoldsInvalid("", "Unable to read input"))
		}
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		holdsFail(w, r, domain.HoldsInvalid("", "Expected a JSON object"))
		return false
	}
	for key, value := range object {
		if key != "expected_release_at" && strings.TrimSpace(string(value)) == "null" {
			holdsFail(w, r, domain.HoldsInvalid(key, "This field must not be null"))
			return false
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(target); e != nil {
		holdsFail(w, r, domain.HoldsInvalid("", "Invalid hold input; use timezone-aware RFC3339 dates"))
		return false
	}
	if e = decoder.Decode(&struct{}{}); e != io.EOF {
		holdsFail(w, r, domain.HoldsInvalid("", "Expected one JSON object"))
		return false
	}
	return true
}
func (h holdsHandler) create(w http.ResponseWriter, r *http.Request) {
	var input domain.HoldsCreate
	if !holdsDecode(w, r, &input) {
		return
	}
	v, e := h.s.Create(r.Context(), bearer(r), chi.URLParam(r, "project"), input)
	if !holdsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_holds.created").Str("hold_id", v.ID).Msg("Purchase hold committed")
		write(w, 201, v)
	}
}
func (h holdsHandler) release(w http.ResponseWriter, r *http.Request) {
	var input domain.HoldsDecision
	if !holdsDecode(w, r, &input) {
		return
	}
	v, e := h.s.Release(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "id"), input)
	if !holdsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_holds.released").Str("hold_id", v.ID).Int32("version", v.Version).Msg("Purchase hold released")
		write(w, 200, v)
	}
}
