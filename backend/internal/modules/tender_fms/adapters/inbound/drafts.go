package inbound

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpinput"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http"
	"strconv"
)

func RegisterDrafts(r chi.Router, s *application.Drafts) {
	r.Get("/api/v1/tender-fms/drafts", func(w http.ResponseWriter, r *http.Request) {
		page, size := 1, 20
		var e error
		if v := r.URL.Query().Get("page"); v != "" {
			page, e = strconv.Atoi(v)
		}
		if e == nil {
			if v := r.URL.Query().Get("page_size"); v != "" {
				size, e = strconv.Atoi(v)
			}
		}
		if e != nil || page < 1 || page > 100000 || size < 1 || size > 100 {
			captureFailure(w, domain.Invalid("Invalid page"))
			return
		}
		sort := r.URL.Query().Get("sort")
		direction := r.URL.Query().Get("direction")
		if sort != "" && sort != "name" && sort != "created" || direction != "" && direction != "asc" && direction != "desc" {
			captureFailure(w, domain.Invalid("Invalid ordering"))
			return
		}
		items, total, e := s.List(r.Context(), captureToken(r), domain.DraftQuery{Search: r.URL.Query().Get("search"), SortName: sort == "name", Ascending: direction == "asc", Limit: int32(size), Offset: int32((page - 1) * size)})
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total})
	})
	r.Post("/api/v1/tender-fms/drafts", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 8192, &in)) {
			return
		}
		d, e := s.Create(r.Context(), captureToken(r), r.Header.Get("Idempotency-Key"), in.Name)
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(d)
	})
	r.Get("/api/v1/tender-fms/drafts/{draft}", func(w http.ResponseWriter, r *http.Request) {
		d, e := s.Get(r.Context(), captureToken(r), chi.URLParam(r, "draft"))
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(d)
	})
	r.Post("/api/v1/tender-fms/drafts/{draft}/batches", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Receipt string `json:"receipt_id"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 8192, &in)) {
			return
		}
		if captureFailure(w, s.Attach(r.Context(), captureToken(r), chi.URLParam(r, "draft"), in.Receipt)) {
			return
		}
		captureHeaders(w)
		w.WriteHeader(204)
	})
	r.Get("/api/v1/tender-fms/drafts/{draft}/documents/{document}/{kind}", func(w http.ResponseWriter, r *http.Request) {
		kind := chi.URLParam(r, "kind")
		b, e := s.Artifact(r.Context(), captureToken(r), chi.URLParam(r, "draft"), chi.URLParam(r, "document"), kind)
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if kind == "result" {
			w.Header().Set("Content-Type", "application/json")
		}
		if kind == "original" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment")
		}
		w.Write(b)
	})
	r.Post("/api/v1/tender-fms/drafts/{draft}/documents/{document}/retry", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Version int32 `json:"expected_version"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 8192, &in)) {
			return
		}
		if captureFailure(w, s.Retry(r.Context(), captureToken(r), chi.URLParam(r, "draft"), chi.URLParam(r, "document"), in.Version)) {
			return
		}
		captureHeaders(w)
		w.WriteHeader(204)
	})
}
