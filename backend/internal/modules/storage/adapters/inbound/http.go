package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/application"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
	"io"
	"mime"
	"net/http"
	"strings"
)

type handler struct{ service *application.Service }

func Register(router chi.Router, service *application.Service) {
	h := handler{service}
	router.Post("/api/v1/documents/upload", h.upload)
	router.Get("/api/v1/documents", h.list)
	router.Get("/api/v1/documents/{document_id}", h.metadata)
	router.Get("/api/v1/documents/{document_id}/download", h.download)
	router.Delete("/api/v1/documents/{document_id}", h.deactivate)
}
func write(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	var denied *domain.Error
	if errors.As(err, &denied) {
		if denied.Code == 401 {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
		write(w, denied.Code, map[string]string{"detail": denied.Detail})
	} else {
		logging.Ctx(r.Context()).Error().Str("event", "documents.operation.failed").Msg("Document operation failed")
		write(w, 500, map[string]string{"detail": "Internal server error"})
	}
	return true
}
func invalid(w http.ResponseWriter, field, source, message string) {
	write(w, 422, map[string]any{"detail": []map[string]any{{"type": "value_error", "loc": []string{source, field}, "msg": message}}})
}
func token(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
func documentID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := domain.CanonicalID(chi.URLParam(r, "document_id"))
	if err != nil {
		invalid(w, "document_id", "path", "Input should be a valid UUID")
		return "", false
	}
	return id, true
}
func event(r *http.Request, kind string) {
	logging.Ctx(r.Context()).Info().Str("event", kind).Msg("Document lifecycle decision")
}

// Bounded in-memory multipart processing; do not spool plaintext evidence to disk.
func parseUpload(w http.ResponseWriter, r *http.Request) (domain.Upload, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaximumDocumentBytes+64*1024)
	reader, err := r.MultipartReader()
	if err != nil {
		invalid(w, "file", "body", "Multipart form required")
		return domain.Upload{}, false
	}
	fields := map[string]string{}
	seen := map[string]bool{}
	request := domain.Upload{}
	for count := 0; ; count++ {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			multipartFailure(w, err)
			return domain.Upload{}, false
		}
		name := part.FormName()
		if count >= 8 || seen[name] || (name != "file" && name != "entity_type" && name != "entity_id" && name != "doc_type" && name != "title") {
			_ = part.Close()
			invalid(w, name, "body", "Unknown or repeated multipart field")
			return domain.Upload{}, false
		}
		seen[name] = true
		maximum := int64(4096)
		if name == "file" {
			maximum = domain.MaximumDocumentBytes
		}
		data, err := io.ReadAll(io.LimitReader(part, maximum+1))
		_ = part.Close()
		if err != nil {
			multipartFailure(w, err)
			return domain.Upload{}, false
		}
		if int64(len(data)) > maximum {
			write(w, 413, map[string]string{"detail": "Request body is too large"})
			return domain.Upload{}, false
		}
		if name == "file" {
			_, parameters, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
			if err != nil {
				invalid(w, "file", "body", "File required")
				return domain.Upload{}, false
			}
			filename, present := parameters["filename"]
			if !present {
				invalid(w, "file", "body", "File required")
				return domain.Upload{}, false
			}
			request.Filename = filename
			if request.Filename == "" {
				request.Filename = "unnamed_document"
			}
			request.ContentType = part.Header.Get("Content-Type")
			if request.ContentType == "" {
				request.ContentType = "application/octet-stream"
			}
			request.Data = data
		} else {
			fields[name] = string(data)
		}
	}
	for _, name := range []string{"file", "entity_type", "entity_id", "doc_type"} {
		if !seen[name] || (name != "file" && fields[name] == "") {
			invalid(w, name, "body", "Field required")
			return domain.Upload{}, false
		}
	}
	id, err := domain.CanonicalID(fields["entity_id"])
	if err != nil {
		invalid(w, "entity_id", "body", "Input should be a valid UUID")
		return domain.Upload{}, false
	}
	request.EntityID = id
	request.EntityType = fields["entity_type"]
	request.DocType = fields["doc_type"]
	request.Title = fields["title"]
	if request.Title == "" {
		request.Title = request.Filename
	}
	return request, true
}
func multipartFailure(w http.ResponseWriter, err error) {
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		write(w, 413, map[string]string{"detail": "Request body is too large"})
	} else {
		invalid(w, "file", "body", "Invalid multipart body")
	}
}
func (h handler) upload(w http.ResponseWriter, r *http.Request) {
	request, ok := parseUpload(w, r)
	if !ok {
		return
	}
	doc, err := h.service.Upload(r.Context(), token(r), request)
	if fail(w, r, err) {
		return
	}
	event(r, "documents.upload.succeeded")
	write(w, 201, doc)
}
func (h handler) metadata(w http.ResponseWriter, r *http.Request) {
	id, ok := documentID(w, r)
	if !ok {
		return
	}
	doc, err := h.service.Metadata(r.Context(), token(r), id)
	if fail(w, r, err) {
		return
	}
	write(w, 200, doc)
}
func (h handler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	entityType := query.Get("entity_type")
	entityID := query.Get("entity_id")
	if _, ok := query["entity_type"]; !ok {
		invalid(w, "entity_type", "query", "Field required")
		return
	}
	if _, ok := query["entity_id"]; !ok {
		invalid(w, "entity_id", "query", "Field required")
		return
	}
	id, err := domain.CanonicalID(entityID)
	if err != nil {
		invalid(w, "entity_id", "query", "Input should be a valid UUID")
		return
	}
	docs, err := h.service.List(r.Context(), token(r), entityType, id)
	if fail(w, r, err) {
		return
	}
	if docs == nil {
		docs = []domain.Document{}
	}
	write(w, 200, map[string]any{"items": docs, "total": len(docs)})
}
func (h handler) download(w http.ResponseWriter, r *http.Request) {
	id, ok := documentID(w, r)
	if !ok {
		return
	}
	doc, data, err := h.service.Download(r.Context(), token(r), id)
	if fail(w, r, err) {
		return
	}
	// Metadata of legacy/imported filenames is not assumed to be header-safe.
	filename := domain.SanitizeFilename(doc.Filename)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("X-Document-SHA256", doc.SHA256)
	w.Header().Set("Content-Type", doc.ContentType)
	event(r, "documents.download.succeeded")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func (h handler) deactivate(w http.ResponseWriter, r *http.Request) {
	id, ok := documentID(w, r)
	if !ok {
		return
	}
	if fail(w, r, h.service.Deactivate(r.Context(), token(r), id)) {
		return
	}
	event(r, "documents.deactivate.succeeded")
	write(w, 200, map[string]string{"status": "deactivated", "id": id})
}
