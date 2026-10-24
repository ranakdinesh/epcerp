package inbound

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http/httptest"
	"testing"
)

type optionsFixture struct{}

func (optionsFixture) Options(_ context.Context, token string) ([]domain.Candidate, error) {
	if token == "rate-limited" {
		return nil, &domain.Error{Status: 429, Detail: "Too many requests"}
	}
	if token != "allowed" {
		return nil, domain.Forbidden
	}
	out := []domain.Candidate{}
	for range 25 {
		out = append(out, domain.Candidate{ID: "18e66d87-c4db-4771-bbdb-03ad3684e345", DisplayName: "Eligible manager"})
	}
	return out, nil
}
func TestManagerOptionsStrictPagedMetadata(t *testing.T) {
	r := chi.NewRouter()
	RegisterManagerOptions(r, optionsFixture{})
	for _, c := range []struct {
		query, auth string
		status      int
	}{
		{"", "", 401}, {"", "Bearer rate-limited", 429}, {"?page=0", "Bearer denied", 403}, {"?page=0", "Bearer allowed", 422},
		{"?organisation=forged", "Bearer allowed", 422}, {"?page=1&page=2", "Bearer allowed", 422},
		{"?page=%GG", "Bearer allowed", 422}, {"?page_size=200", "Bearer allowed", 422}, {"?page=2", "Bearer allowed", 200},
	} {
		q := httptest.NewRequest("GET", "/api/v1/tender-fms/assignment-options"+c.query, nil)
		q.Header.Set("Authorization", c.auth)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, q)
		if out.Code != c.status {
			t.Fatal(c, out.Code, out.Body.String())
		}
		var body map[string]any
		if e := json.Unmarshal(out.Body.Bytes(), &body); e != nil {
			t.Fatal(e)
		}
		if c.status == 200 {
			if len(body["items"].([]any)) != 5 || body["total"] != float64(25) || len(body) != 4 {
				t.Fatal(body)
			}
			for _, v := range body["items"].([]any) {
				if len(v.(map[string]any)) != 2 {
					t.Fatal("candidate leaks fields")
				}
			}
		} else if len(body) != 4 || body["request_id"] == "" {
			t.Fatal("invalid error contract", body)
		}
		if c.status == 429 && out.Header().Get("Retry-After") != "60" {
			t.Fatal("missing rate retry header")
		}
	}
}
