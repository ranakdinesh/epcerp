package inbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type ManagerOptions interface {
	Options(context.Context, string) ([]domain.Candidate, error)
}

// AP05 is metadata-only and side-effect-free; no employee email or permissions leak.
func RegisterManagerOptions(r chi.Router, s ManagerOptions) {
	r.Get("/api/v1/tender-fms/assignment-options", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		fail := func(e error) {
			var requestBytes [16]byte
			_, _ = rand.Read(requestBytes[:])
			requestID := hex.EncodeToString(requestBytes[:])
			status, code, detail := 503, "dependency_unavailable", "Cannot read eligible Tender Managers"
			var de *domain.Error
			if errors.As(e, &de) {
				status = de.Status
				detail = de.Detail
				code = map[int]string{401: "unauthorized", 403: "forbidden", 404: "not_found", 422: "invalid_request", 429: "rate_limited"}[status]
			}
			if status == 429 {
				w.Header().Set("Retry-After", "60")
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "detail": detail, "request_id": requestID, "retryable": status == 503 || status == 429})
		}
		h := r.Header.Get("Authorization")
		parts := strings.Fields(h)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			fail(domain.Unauthorized)
			return
		}
		// Authenticate before returning query validation details.
		rows, e := s.Options(r.Context(), parts[1])
		if e != nil {
			fail(e)
			return
		}
		q, e := parseOptionsPage(r)
		if e != nil {
			fail(e)
			return
		}
		start := (q.page - 1) * q.size
		end := start + q.size
		if start > len(rows) {
			start = len(rows)
		}
		if end > len(rows) {
			end = len(rows)
		}
		type candidate struct {
			ID   string `json:"id"`
			Name string `json:"display_name"`
		}
		items := []candidate{}
		for _, v := range rows[start:end] {
			name := []rune(v.DisplayName)
			if len(name) > 255 {
				name = name[:255]
			}
			items = append(items, candidate{v.ID, string(name)})
		}
		_ = json.NewEncoder(w).Encode(struct {
			Items []candidate `json:"items"`
			Total int         `json:"total"`
			Page  int         `json:"page"`
			Size  int         `json:"page_size"`
		}{items, len(rows), q.page, q.size})
	})
}

type optionsPage struct{ page, size int }

func parseOptionsPage(r *http.Request) (optionsPage, error) {
	p := optionsPage{1, 20}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return p, domain.Invalid("Malformed query parameters")
	}
	for key, v := range q {
		if (key != "page" && key != "page_size") || len(v) != 1 {
			return p, domain.Invalid("Only one page and page_size are allowed")
		}
	}
	if q.Has("page") {
		p.page, e = strconv.Atoi(q.Get("page"))
		if e != nil {
			return p, domain.Invalid("Invalid page")
		}
	}
	if q.Has("page_size") {
		p.size, e = strconv.Atoi(q.Get("page_size"))
		if e != nil {
			return p, domain.Invalid("Invalid page size")
		}
	}
	if p.page < 1 || p.page > 100000 || (p.size != 20 && p.size != 50 && p.size != 100) {
		return p, domain.Invalid("Invalid pagination")
	}
	return p, nil
}
