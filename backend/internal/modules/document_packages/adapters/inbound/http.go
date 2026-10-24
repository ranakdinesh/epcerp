package inbound

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpinput"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/application"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
	"net/http"
	"strconv"
	"strings"
)

type receiptFile struct {
	ID        string  `json:"file_id"`
	ClientKey string  `json:"client_file_key"`
	Filename  string  `json:"filename"`
	Version   int64   `json:"version"`
	State     string  `json:"state"`
	Size      int64   `json:"size_bytes"`
	SHA256    *string `json:"sha256"`
	Error     *string `json:"error_code"`
}
type receiptDTO struct {
	Error      *string       `json:"error_code"`
	ID         string        `json:"id"`
	Version    int64         `json:"version"`
	State      string        `json:"state"`
	Revision   int64         `json:"limit_profile_revision"`
	Expires    string        `json:"expires_at"`
	Files      []receiptFile `json:"files"`
	Members    int64         `json:"member_count"`
	Unresolved int64         `json:"unresolved_count"`
}

func pointer(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func write(w http.ResponseWriter, status int, r domain.Receipt, replay bool) {
	headers(w)
	if replay {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.Header().Set("Location", "/api/v1/tender-fms/receipts/"+r.ID)
	out := receiptDTO{pointer(r.ErrorCode), r.ID, r.Version, r.State, r.LimitRevision, r.ExpiresAt.UTC().Format("2006-01-02T15:04:05.999999999Z"), []receiptFile{}, r.MemberCount, r.Unresolved}
	for _, f := range r.Files {
		out.Files = append(out.Files, receiptFile{f.ID, f.ClientKey, f.Filename, f.Version, f.State, f.Size, pointer(f.SHA256), pointer(f.ErrorCode)})
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}
func headers(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}
func failure(w http.ResponseWriter, r *http.Request, e error) bool {
	if e == nil {
		return false
	}
	headers(w)
	var requestBytes [16]byte
	_, _ = rand.Read(requestBytes[:])
	status, code, detail := 503, "dependency_unavailable", "Cannot complete document receipt operation"
	var d *domain.Error
	var input *httpinput.Error
	if errors.As(e, &d) {
		status, code, detail = d.Status, d.Code, d.Detail
	} else if errors.As(e, &input) {
		status, code, detail = input.Status, "invalid_request", input.Detail
	}
	if status == 429 {
		w.Header().Set("Retry-After", "60")
	}
	w.WriteHeader(status)
	if r.Method != "HEAD" {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "detail": detail, "request_id": hex.EncodeToString(requestBytes[:]), "retryable": status == 503 || status == 429})
	}
	return true
}
func token(r *http.Request) string {
	p := strings.Fields(r.Header.Get("Authorization"))
	if len(p) == 2 && strings.EqualFold(p[0], "Bearer") {
		return p[1]
	}
	return ""
}
func Register(r chi.Router, s *application.Service) {
	r.Post("/api/v1/tender-fms/receipts", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Files []struct {
				Key      string `json:"client_file_key"`
				Filename string `json:"filename"`
				Kind     string `json:"kind"`
				Size     int64  `json:"size_bytes"`
				Hash     string `json:"sha256"`
			} `json:"files"`
		}
		if failure(w, r, s.Authorize(r.Context(), token(r))) {
			return
		}
		if failure(w, r, httpinput.Decode(w, r, 32<<10, &body)) {
			return
		}
		files := []domain.Declaration{}
		for _, f := range body.Files {
			files = append(files, domain.Declaration{ClientKey: f.Key, Filename: f.Filename, Kind: f.Kind, Size: f.Size, SHA256: f.Hash})
		}
		out, replay, e := s.Create(r.Context(), token(r), r.Header.Get("Idempotency-Key"), files)
		if failure(w, r, e) {
			return
		}
		write(w, 201, out, replay)
	})
	r.Get("/api/v1/tender-fms/receipts/{receipt_id}", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			failure(w, r, domain.Fail(422, "validation_failed", "No query fields are supported"))
			return
		}
		out, e := s.Get(r.Context(), token(r), chi.URLParam(r, "receipt_id"))
		if failure(w, r, e) {
			return
		}
		write(w, 200, out, false)
	})
	r.MethodFunc("HEAD", "/api/v1/tender-fms/receipts/{receipt_id}/files/{file_id}", func(w http.ResponseWriter, r *http.Request) {
		if failure(w, r, s.Preflight(r.Context(), token(r), chi.URLParam(r, "receipt_id"), chi.URLParam(r, "file_id"))) {
			return
		}
		headers(w)
		w.WriteHeader(204)
	})
	r.Put("/api/v1/tender-fms/receipts/{receipt_id}/files/{file_id}", func(w http.ResponseWriter, r *http.Request) {
		if failure(w, r, s.Preflight(r.Context(), token(r), chi.URLParam(r, "receipt_id"), chi.URLParam(r, "file_id"))) {
			return
		}
		if r.Header.Get("Content-Type") != "application/octet-stream" {
			failure(w, r, domain.Fail(415, "unsupported_media", "Use application/octet-stream"))
			return
		}
		version, e := strconv.ParseInt(r.Header.Get("X-Upload-Version"), 10, 64)
		if e != nil {
			failure(w, r, domain.Fail(422, "validation_failed", "Current upload version is required"))
			return
		}
		out, replay, e := s.Upload(r.Context(), token(r), chi.URLParam(r, "receipt_id"), chi.URLParam(r, "file_id"), r.Header.Get("Idempotency-Key"), version, http.MaxBytesReader(w, r.Body, domain.ZipBytes+1))
		if failure(w, r, e) {
			return
		}
		write(w, 200, out, replay)
	})
	r.Post("/api/v1/tender-fms/receipts/{receipt_id}/seal", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "receipt_id")
		if _, e := s.Get(r.Context(), token(r), id); failure(w, r, e) {
			return
		}
		var body struct {
			Version int64 `json:"expected_version"`
		}
		if failure(w, r, httpinput.Decode(w, r, 32<<10, &body)) {
			return
		}
		out, replay, e := s.Seal(r.Context(), token(r), id, r.Header.Get("Idempotency-Key"), body.Version)
		if failure(w, r, e) {
			return
		}
		write(w, 202, out, replay)
	})
}

// IsUploadPath is used only to select a larger raw-byte policy, with an exact
// service authorization guard still running before any body read.
func IsUploadPath(p string) bool {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	return len(parts) == 7 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "tender-fms" && parts[3] == "receipts" && parts[5] == "files" && domain.UUID(parts[4]) == nil && domain.UUID(parts[6]) == nil
}
