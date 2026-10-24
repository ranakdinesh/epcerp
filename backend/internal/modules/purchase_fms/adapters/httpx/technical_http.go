package httpx

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx/dtos"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type technicalHandler struct {
	s     *application.TechnicalService
	files *application.TechnicalEvidenceService
}

func technicalFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_technical.operation.failed", "Technical operation failed")
}

func technicalNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) != 0 {
		technicalFail(w, r, domain.TechnicalInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h technicalHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !technicalNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !technicalFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h technicalHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !technicalNoQuery(w, r) {
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if technicalFail(w, r, e) {
		return
	}
	q, e := dtos.TechnicalDecode(bytes.NewReader(raw))
	if technicalFail(w, r, e) {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if !technicalFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_technical.command.committed").Str("action", q.Action).Str("case_id", v.Case.ID).Msg("Technical command committed")
		write(w, 200, v)
	}
}
func technicalUploadCommand(purpose, version string) (domain.TechnicalUpload, error) {
	v, e := strconv.ParseInt(version, 10, 32)
	if e != nil || v < 1 || strconv.FormatInt(v, 10) != version {
		return domain.TechnicalUpload{}, domain.TechnicalInvalid("version", "Use the current positive kit version")
	}
	return domain.TechnicalUpload{Purpose: purpose, Version: int32(v)}, nil
}
func (h technicalHandler) authority(w http.ResponseWriter, r *http.Request) {
	if !technicalNoQuery(w, r) {
		return
	}
	q, e := technicalUploadCommand(chi.URLParam(r, "purpose"), chi.URLParam(r, "version"))
	if technicalFail(w, r, e) {
		return
	}
	if !technicalFail(w, r, h.files.AuthorizeUpload(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(204)
	}
}
func (h technicalHandler) upload(w http.ResponseWriter, r *http.Request) {
	if !technicalNoQuery(w, r) {
		return
	}
	q, e := technicalUploadCommand(chi.URLParam(r, "purpose"), chi.URLParam(r, "version"))
	if technicalFail(w, r, e) {
		return
	}
	p, id := chi.URLParam(r, "project"), chi.URLParam(r, "package")
	if technicalFail(w, r, h.files.AuthorizeUpload(r.Context(), bearer(r), p, id, q)) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	reader, e := r.MultipartReader()
	if e != nil {
		technicalFail(w, r, domain.TechnicalInvalid("file", "Expected one file and one reason multipart field"))
		return
	}
	seen := map[string]bool{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			technicalFail(w, r, domain.TechnicalInvalid("file", "Invalid multipart evidence"))
			return
		}
		_, fields, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		name := fields["name"]
		if err != nil || seen[name] || (name != "file" && name != "reason") {
			_ = part.Close()
			technicalFail(w, r, domain.TechnicalInvalid("file", "Provide file and reason exactly once"))
			return
		}
		seen[name] = true
		limit := int64(8000)
		if name == "file" {
			limit = domain.TechnicalMaxFileBytes
			q.Name = fields["filename"]
			q.MediaType = part.Header.Get("Content-Type")
		} else if fields["filename"] != "" {
			_ = part.Close()
			technicalFail(w, r, domain.TechnicalInvalid("reason", "Reason must be a text field"))
			return
		}
		data, err := io.ReadAll(io.LimitReader(part, limit+1))
		_ = part.Close()
		if err != nil {
			if !technicalFail(w, r, err) {
				return
			}
			return
		}
		if int64(len(data)) > limit {
			write(w, 413, map[string]string{"detail": "Evidence field exceeds the permitted size"})
			return
		}
		if name == "file" {
			q.Data = data
		} else {
			q.Reason = string(data)
		}
	}
	if !seen["file"] || !seen["reason"] {
		technicalFail(w, r, domain.TechnicalInvalid("file", "Provide one actual file and its upload reason"))
		return
	}
	v, e := h.files.Upload(r.Context(), bearer(r), p, id, q)
	if !technicalFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_technical.evidence_uploaded").Str("file_id", v.ID).Msg("Technical evidence committed")
		write(w, 201, v)
	}
}
func (h technicalHandler) download(w http.ResponseWriter, r *http.Request) {
	if !technicalNoQuery(w, r) {
		return
	}
	v, data, e := h.files.Download(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), chi.URLParam(r, "file"))
	if technicalFail(w, r, e) {
		return
	}
	w.Header().Set("Content-Type", v.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": v.Name}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Encrypted-Storage", "true")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func TechnicalIsUploadPath(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	return len(p) == 10 && p[0] == "api" && p[1] == "v1" && p[2] == "projects" && p[3] != "" && p[4] == "purchase-workflow" && p[5] != "" && p[6] == "technical" && p[7] == "uploads" && p[8] != "" && p[9] != ""
}

// Guard precedes payload buffering in the API composition root.
func TechnicalUploadGuard(s *application.TechnicalEvidenceService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !TechnicalIsUploadPath(r.URL.Path) {
				technicalFail(w, r, domain.TechnicalMissing)
				return
			}
			if !technicalNoQuery(w, r) {
				return
			}
			p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			q, e := technicalUploadCommand(p[8], p[9])
			if technicalFail(w, r, e) {
				return
			}
			if technicalFail(w, r, s.AuthorizeUpload(r.Context(), bearer(r), p[3], p[5], q)) {
				return
			}
			controller := http.NewResponseController(w)
			_ = controller.SetReadDeadline(time.Now().Add(60 * time.Second))
			_ = controller.SetWriteDeadline(time.Now().Add(75 * time.Second))
			next.ServeHTTP(w, r)
		})
	}
}
