package httpx

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"net/http"
)

func LOIDocumentsRegister(r chi.Router, s *application.LOIDocumentsService) {
	if s == nil {
		return
	}
	r.Route("/api/v1/projects/{project}/vendor-lois/{attempt}", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if !ordersNoQuery(w, r) {
				return
			}
			v, e := s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "attempt"))
			if !ordersFail(w, r, e) {
				write(w, 200, v)
			}
		})
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			if !ordersNoQuery(w, r) {
				return
			}
			var q domain.LOICommand
			if !workflowDecode(w, r, &q) {
				return
			}
			v, e := s.Generate(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "attempt"), q)
			if !ordersFail(w, r, e) {
				write(w, 200, v)
			}
		})
		r.Post("/files/{id}/complete-review", func(w http.ResponseWriter, r *http.Request) {
			if !ordersNoQuery(w, r) {
				return
			}
			var q domain.LOIReviewCommand
			if !workflowDecode(w, r, &q) {
				return
			}
			v, e := s.CompleteReview(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "attempt"), chi.URLParam(r, "id"), q)
			if !ordersFail(w, r, e) {
				write(w, 200, v)
			}
		})
		r.Get("/files/{id}", func(w http.ResponseWriter, r *http.Request) {
			if !ordersNoQuery(w, r) {
				return
			}
			data, name, e := s.Download(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "attempt"), chi.URLParam(r, "id"))
			if ordersFail(w, r, e) {
				return
			}
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Encrypted-Storage", "MinIO")
			w.WriteHeader(200)
			_, _ = w.Write(data)
		})
	})
}
