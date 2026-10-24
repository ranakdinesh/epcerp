package httpx

import (
	"github.com/go-chi/chi/v5"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"net/http"
)

func AwardsRegister(r chi.Router, s *application.AwardsService) {
	if s == nil {
		return
	}
	r.Route("/api/v1/projects/{project}/purchase-awards", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if !ordersNoQuery(w, r) {
				return
			}
			v, e := s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"))
			if !ordersFail(w, r, e) {
				write(w, 200, v)
			}
		})
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			if !ordersNoQuery(w, r) {
				return
			}
			var q domain.AwardCommand
			if !workflowDecode(w, r, &q) {
				return
			}
			v, e := s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), q)
			if !ordersFail(w, r, e) {
				write(w, 200, v)
			}
		})
	})
}
