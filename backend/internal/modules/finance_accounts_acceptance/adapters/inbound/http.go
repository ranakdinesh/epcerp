package inbound

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/application"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
	"io"
	"net/http"
	"strings"
)

type handler struct{ s *application.Service }

func Register(r chi.Router, s *application.Service) {
	h := handler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/accounts-acceptance", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}
func token(r *http.Request) string {
	a := r.Header.Get("Authorization")
	if strings.HasPrefix(a, "Bearer ") {
		return strings.TrimPrefix(a, "Bearer ")
	}
	return ""
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, r *http.Request, e error) bool {
	if e == nil {
		return false
	}
	var d *domain.Error
	if errors.As(e, &d) {
		if d.Status == 422 && d.Field != "" {
			write(w, d.Status, map[string]any{"detail": []map[string]any{{"loc": []string{"body", d.Field}, "msg": d.Message, "type": "value_error"}}})
		} else {
			write(w, d.Status, map[string]string{"detail": d.Message})
		}
	} else {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			logging.Ctx(r.Context()).Error().Str("event", "finance_accounts_acceptance.operation.failed").Msg("Accounts acceptance operation failed")
			write(w, 500, map[string]string{"detail": "Internal server error"})
		}
	}
	return true
}
func noQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		fail(w, r, domain.Invalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h handler) workspace(w http.ResponseWriter, r *http.Request) {
	if !noQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), token(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func (h handler) execute(w http.ResponseWriter, r *http.Request) {
	if !noQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if fail(w, r, e) {
		return
	}
	q, e := Decode(bytes.NewReader(raw))
	if fail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), token(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !fail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "finance_accounts_acceptance.command.committed").Str("action", q.Action).Str("case_id", v.Record.ID).Msg("Accounts acceptance command committed")
		write(w, 200, v)
	}
}
