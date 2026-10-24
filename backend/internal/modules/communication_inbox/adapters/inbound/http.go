package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/application"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type handler struct{ s *application.Service }

func Register(r chi.Router, s *application.Service) {
	h := handler{s}
	r.Get("/api/v1/inbox", h.list)
	r.Get("/api/v1/inbox/{notice}", h.read)
	r.Post("/api/v1/inbox/{notice}/read", h.receipt)
	r.Post("/api/v1/inbox/{notice}/acknowledge", h.receipt)
}
func token(r *http.Request) string {
	v := strings.Fields(r.Header.Get("Authorization"))
	if len(v) == 2 && strings.EqualFold(v[0], "Bearer") {
		return v[1]
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
		if d.Status == 401 {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
		if d.Field != "" {
			write(w, d.Status, map[string]any{"detail": []map[string]any{{"loc": []string{"body", d.Field}, "msg": d.Message, "type": "value_error"}}})
		} else {
			write(w, d.Status, map[string]string{"detail": d.Message})
		}
	} else {
		logging.Ctx(r.Context()).Error().Str("event", "communication_inbox.operation.failed").Msg("Inbox operation failed")
		write(w, 500, map[string]string{"detail": "Internal server error"})
	}
	return true
}
func (h handler) list(w http.ResponseWriter, r *http.Request) {
	q := domain.Query{Page: 1, Size: 20}
	values := r.URL.Query()
	for k, v := range values {
		if (k != "page" && k != "size" && k != "status") || len(v) != 1 {
			fail(w, r, domain.Invalid(k, "Unsupported inbox query"))
			return
		}
	}
	for k, p := range map[string]*int32{"page": &q.Page, "size": &q.Size} {
		if v := values.Get(k); v != "" {
			n, e := strconv.ParseInt(v, 10, 32)
			if e != nil {
				fail(w, r, domain.Invalid(k, "Use a whole page or size"))
				return
			}
			*p = int32(n)
		}
	}
	q.Status = values.Get("status")
	v, e := h.s.List(r.Context(), token(r), q)
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func (h handler) read(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query()) != 0 {
		fail(w, r, domain.Invalid("query", "Unsupported notice query"))
		return
	}
	v, e := h.s.Read(r.Context(), token(r), chi.URLParam(r, "notice"))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func decode(w http.ResponseWriter, r *http.Request) (int32, error) {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<10))
	if e != nil {
		var limit *http.MaxBytesError
		if errors.As(e, &limit) {
			return 0, &domain.Error{Status: 413, Message: "Request body too large"}
		}
		return 0, domain.Invalid("", "Unable to read receipt")
	}
	var f map[string]json.RawMessage
	if json.Unmarshal(raw, &f) != nil || len(f) != 1 || f["version"] == nil || strings.TrimSpace(string(f["version"])) == "null" {
		return 0, domain.Invalid("version", "Provide only the current notice version")
	}
	var v int32
	if json.Unmarshal(f["version"], &v) != nil || v < 1 {
		return 0, domain.Invalid("version", "A positive whole notice version is required")
	}
	return v, nil
}
func (h handler) receipt(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query()) != 0 {
		fail(w, r, domain.Invalid("query", "Unsupported receipt query"))
		return
	}
	version, e := decode(w, r)
	if fail(w, r, e) {
		return
	}
	action := "read"
	if strings.HasSuffix(r.URL.Path, "/acknowledge") {
		action = "acknowledge"
	}
	v, e := h.s.Receipt(r.Context(), token(r), chi.URLParam(r, "notice"), version, action)
	if fail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "communication_inbox.receipt.saved").Str("notice_id", v.Notice.ID).Int32("version", v.Notice.Version).Msg("Notice receipt recorded")
	write(w, 200, v)
}
