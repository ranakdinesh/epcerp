package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"net/http"
	"strings"
)

func ownershipFailure(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	code := 500
	detail := "Cannot complete tender handover"
	var d *domain.Error
	if errors.As(e, &d) {
		code = d.Status
		detail = d.Detail
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": detail})
	return true
}
func RegisterOwnership(router chi.Router, s *application.OwnershipService) {
	router.Get("/api/v1/tender-fms/{intake_id}/ownership", func(w http.ResponseWriter, r *http.Request) {
		id, e := domain.ID(chi.URLParam(r, "intake_id"))
		if ownershipFailure(w, e) {
			return
		}
		v, choices, e := s.Options(r.Context(), strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), id)
		if ownershipFailure(w, e) {
			return
		}
		out := []map[string]string{}
		for _, c := range choices {
			out = append(out, map[string]string{"id": c.ID, "name": c.Name})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"owner_id": v.OwnerID, "version": v.Version, "candidates": out})
	})
	router.Post("/api/v1/tender-fms/{intake_id}/ownership", func(w http.ResponseWriter, r *http.Request) {
		id, e := domain.ID(chi.URLParam(r, "intake_id"))
		if ownershipFailure(w, e) {
			return
		}
		var p struct {
			OwnerID string `json:"owner_id"`
			Version int32  `json:"version"`
			Reason  string `json:"reason"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		dec.DisallowUnknownFields()
		if e = dec.Decode(&p); e != nil {
			var limit *http.MaxBytesError
			if errors.As(e, &limit) {
				w.WriteHeader(413)
			} else {
				w.WriteHeader(422)
			}
			return
		}
		if e = dec.Decode(new(any)); e != io.EOF {
			w.WriteHeader(422)
			return
		}
		v, e := s.Handover(r.Context(), strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), id, domain.OwnershipInput{OwnerID: p.OwnerID, Version: p.Version, Reason: strings.TrimSpace(p.Reason)})
		if ownershipFailure(w, e) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"owner_id": v.OwnerID, "version": v.Version})
	})
}
