package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/application"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/domain"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func Register(r chi.Router, s application.Service) {
	r.Get("/api/v1/work-calendar/holidays", func(w http.ResponseWriter, r *http.Request) {
		year := time.Now().Year()
		q := r.URL.Query()
		if len(q) > 1 || len(q["year"]) > 1 || (len(q) == 1 && q["year"] == nil) {
			reply(w, 422, map[string]string{"detail": "Select a calendar year"})
			return
		}
		if v := q.Get("year"); v != "" {
			year, _ = strconv.Atoi(v)
		}
		rows, e := s.List(r.Context(), token(r), year)
		if failure(w, e) {
			return
		}
		out := []map[string]any{}
		for _, h := range rows {
			out = append(out, view(h))
		}
		reply(w, 200, out)
	})
	save := func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name    string `json:"name"`
			Date    string `json:"holiday_date"`
			Active  *bool  `json:"is_active"`
			Reason  string `json:"reason"`
			Version int32  `json:"expected_version"`
		}
		media, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mediaErr != nil || media != "application/json" {
			reply(w, 415, map[string]string{"detail": "Use application/json"})
			return
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		d.DisallowUnknownFields()
		var extra any
		decodeErr := d.Decode(&in)
		var sizeErr *http.MaxBytesError
		if errors.As(decodeErr, &sizeErr) {
			reply(w, 413, map[string]string{"detail": "Holiday request exceeds 16 KiB"})
			return
		}
		if decodeErr != nil || d.Decode(&extra) != io.EOF || in.Active == nil {
			failure(w, domain.Invalid)
			return
		}
		h, e := s.Save(r.Context(), token(r), domain.Change{ID: chi.URLParam(r, "id"), Name: in.Name, Date: in.Date, Active: *in.Active, Reason: in.Reason, Version: in.Version})
		if failure(w, e) {
			return
		}
		logging.Ctx(r.Context()).Info().Str("event", "work_calendar.saved").Str("holiday_id", h.ID).Int32("version", h.Version).Msg("Holiday and project calculations committed")
		reply(w, 200, view(h))
	}
	r.Post("/api/v1/work-calendar/holidays", save)
	r.Patch("/api/v1/work-calendar/holidays/{id}", save)
}
func token(r *http.Request) string {
	v := strings.Fields(r.Header.Get("Authorization"))
	if len(v) != 2 || !strings.EqualFold(v[0], "Bearer") {
		return ""
	}
	return v[1]
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func failure(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	status := 500
	message := "Calendar operation failed"
	switch {
	case errors.Is(e, domain.Unauthorized):
		status = 401
		message = e.Error()
	case errors.Is(e, domain.Forbidden):
		status = 403
		message = e.Error()
	case errors.Is(e, domain.Invalid):
		status = 422
		message = e.Error()
	case errors.Is(e, domain.Conflict):
		status = 409
		message = e.Error()
	}
	reply(w, status, map[string]string{"detail": message})
	return true
}
func view(h domain.Holiday) map[string]any {
	return map[string]any{"id": h.ID, "name": h.Name, "holiday_date": h.Date, "is_active": h.Active, "version": h.Version}
}
