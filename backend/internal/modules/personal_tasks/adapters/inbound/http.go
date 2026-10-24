package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/application"
	"github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/domain"
	"net/http"
	"strconv"
	"strings"
)

func Query(r *http.Request) (domain.Query, error) {
	q := domain.Query{}
	for k, values := range r.URL.Query() {
		if len(values) != 1 {
			return q, domain.Invalid("query", "Use each query field once")
		}
		v := values[0]
		switch k {
		case "page", "page_size":
			n, e := strconv.ParseInt(v, 10, 32)
			if e != nil || n < 1 {
				return q, domain.Invalid(k, "Use a positive page or size")
			}
			if k == "page" {
				q.Page = int32(n)
			} else {
				q.Size = int32(n)
			}
		case "search":
			q.Search = v
		case "role":
			q.Role = v
		case "state":
			q.State = v
		default:
			return q, domain.Invalid(k, "Query field not supported")
		}
	}
	return q.Validate()
}
func Register(r chi.Router, s *application.Service) {
	r.Get("/api/v1/my-tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		q, e := Query(r)
		var p domain.Page
		if e == nil {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			p, e = s.Page(r.Context(), token, q)
		}
		if e != nil {
			var d *domain.Error
			if errors.As(e, &d) {
				w.WriteHeader(d.Status)
				_ = json.NewEncoder(w).Encode(map[string]string{"detail": d.Message})
			} else {
				w.WriteHeader(500)
				_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Cannot load personal tasks"})
			}
			return
		}
		_ = json.NewEncoder(w).Encode(p)
	})
}
