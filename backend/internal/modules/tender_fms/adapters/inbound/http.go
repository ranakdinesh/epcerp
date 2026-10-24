package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"mime"
	"net/http"
	"sort"
	"strings"
	"time"
)

const UploadBytes = 32 << 20

func IsUploadPath(path string) bool { return path == "/api/v1/tender-fms/upload" }

type handler struct{ s *application.Service }

func Register(r chi.Router, s *application.Service) {
	h := handler{s}
	r.Get("/api/v1/tender-fms", h.list)
	r.Post("/api/v1/tender-fms/upload", h.upload)
	r.Get("/api/v1/tender-fms/{id}/history", h.history)
	r.Get("/api/v1/tender-fms/{id}/documents/{doc}/{kind}", h.artifact)
	r.Post("/api/v1/tender-fms/{id}/documents/{doc}/retry", h.retry)
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
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
		write(w, d.Status, map[string]string{"detail": d.Detail})
	} else {
		logging.Ctx(r.Context()).Error().Str("event", "tender_fms.operation.failed").Msg("Tender operation failed")
		write(w, 500, map[string]string{"detail": "Internal server error"})
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
func (h handler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), token(r))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func (h handler) history(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.History(r.Context(), token(r), chi.URLParam(r, "id"))
	if !fail(w, r, e) {
		sort.SliceStable(v, func(i, j int) bool { return v[i].CreatedAt.After(v[j].CreatedAt) })
		write(w, 200, v)
	}
}
func (h handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, UploadBytes+(64<<10))
	reader, e := r.MultipartReader()
	if e != nil {
		fail(w, r, domain.Invalid("Choose one document using multipart upload"))
		return
	}
	var data []byte
	filename, existing := "", ""
	seen := map[string]bool{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			write(w, 413, map[string]string{"detail": "Upload exceeds permitted size or is malformed"})
			return
		}
		name := part.FormName()
		if seen[name] || (name != "file" && name != "intake_id") {
			part.Close()
			fail(w, r, domain.Invalid("Upload accepts one file and an optional intake_id"))
			return
		}
		seen[name] = true
		limit := int64(256)
		if name == "file" {
			limit = UploadBytes
			filename = part.FileName()
		}
		b, err := io.ReadAll(io.LimitReader(part, limit+1))
		part.Close()
		if err != nil || int64(len(b)) > limit {
			write(w, 413, map[string]string{"detail": "Document exceeds the 32 MiB limit"})
			return
		}
		if name == "file" {
			data = b
		} else {
			existing = string(b)
		}
	}
	if filename == "" || len(data) == 0 {
		fail(w, r, domain.Invalid("Choose a nonempty PDF, Excel or Word document"))
		return
	}
	v, e := h.s.Upload(r.Context(), token(r), existing, filename, data)
	if !fail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "tender_fms.source.queued").Str("intake_id", v.ID).Msg("Tender source upload committed")
		write(w, 202, v)
	}
}
func (h handler) artifact(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if kind != "original" && kind != "text" && kind != "result" {
		fail(w, r, domain.Missing)
		return
	}
	v, e := h.s.Artifact(r.Context(), token(r), chi.URLParam(r, "id"), chi.URLParam(r, "doc"), kind)
	if fail(w, r, e) {
		return
	}
	w.Header().Set("Content-Type", v.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": v.Name}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Encrypted-Storage", "true")
	w.WriteHeader(200)
	_, _ = w.Write(v.Data)
}
func (h handler) retry(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version int32 `json:"version"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(new(any)) != io.EOF || in.Version < 1 {
		fail(w, r, domain.Invalid("Provide only a positive reading version"))
		return
	}
	v, e := h.s.Retry(r.Context(), token(r), chi.URLParam(r, "id"), chi.URLParam(r, "doc"), in.Version)
	if !fail(w, r, e) {
		write(w, 202, v)
	}
}

func UploadGuard(s *application.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if fail(w, r, s.AuthorizeUpload(r.Context(), token(r))) {
				return
			}
			controller := http.NewResponseController(w)
			_ = controller.SetReadDeadline(time.Now().Add(60 * time.Second))
			_ = controller.SetWriteDeadline(time.Now().Add(75 * time.Second))
			next.ServeHTTP(w, r)
		})
	}
}
