package httpx

import (
	"encoding/json"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"io"
	"mime"
	"net/http"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type vendorDocumentHandler struct {
	s *masterApplication.VendorDocumentService
}

func vendorDocumentFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "supplier_documents.operation.failed")
}

func vendorDocumentDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	d.UseNumber()
	e := masterDTO.DecodeMasterJSON(d, v)
	if e == nil {
		var extra any
		if masterDTO.DecodeMasterJSON(d, &extra) != io.EOF {
			e = errors.New("trailing input")
		}
	}
	if e != nil {
		var max *http.MaxBytesError
		if errors.As(e, &max) {
			writeJSON(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			writeJSON(w, 422, map[string]string{"detail": "Invalid import payload"})
		}
		return false
	}
	return true
}
func RegisterVendorDocumentRoutes(r chi.Router, s *masterApplication.VendorDocumentService) {
	h := vendorDocumentHandler{s}
	r.Get("/api/v1/suppliers/{supplier_id}/documents", h.list)
	r.Post("/api/v1/suppliers/{supplier_id}/documents", h.upload)
	r.Post("/api/v1/suppliers/{supplier_id}/documents/reconcile-approvals", h.reconcileApprovals)
	r.Get("/api/v1/suppliers/{supplier_id}/documents/{document_id}/preview", h.preview)
	r.Post("/api/v1/suppliers/{supplier_id}/documents/{document_id}/archive", h.archive)
	r.Post("/api/v1/suppliers/{supplier_id}/documents/legacy-import", h.importRecord)
	r.Get("/api/v1/suppliers/{supplier_id}/documents/{document_id}/download", h.download)
}
func vendorDocumentSupplier(w http.ResponseWriter, r *http.Request) (string, bool) {
	s := chi.URLParam(r, "supplier_id")
	if !masterDomain.VendorDocumentValidID(s) {
		writeJSON(w, 422, map[string]string{"detail": "Invalid supplier identity"})
		return "", false
	}
	return s, true
}
func (h vendorDocumentHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := vendorDocumentSupplier(w, r)
	if !ok {
		return
	}
	v, e := h.s.List(r.Context(), bearerToken(r), id)
	if !vendorDocumentFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h vendorDocumentHandler) importRecord(w http.ResponseWriter, r *http.Request) {
	id, ok := vendorDocumentSupplier(w, r)
	if !ok {
		return
	}
	var in masterDomain.VendorDocumentInput
	if !vendorDocumentDecode(w, r, &in) {
		return
	}
	v, e := h.s.Import(r.Context(), bearerToken(r), id, in)
	if vendorDocumentFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "supplier_documents.legacy_imported").Str("document_id", v.ID).Msg("Supplier document imported")
	writeJSON(w, 200, v)
}
func (h vendorDocumentHandler) download(w http.ResponseWriter, r *http.Request) {
	supplier, ok := vendorDocumentSupplier(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "document_id")
	if !masterDomain.VendorDocumentValidID(id) {
		writeJSON(w, 422, map[string]string{"detail": "Invalid document identity"})
		return
	}
	v, data, e := h.s.Download(r.Context(), bearerToken(r), supplier, id)
	if vendorDocumentFail(w, r, e) {
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": v.Filename}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Encrypted-Storage", "true")
	_, _ = w.Write(data)
}

func (h vendorDocumentHandler) preview(w http.ResponseWriter, r *http.Request) {
	s, ok := vendorDocumentSupplier(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "document_id")
	if !masterDomain.VendorDocumentValidID(id) {
		writeJSON(w, 422, map[string]string{"detail": "Invalid document identity"})
		return
	}
	v, data, e := h.s.Preview(r.Context(), bearerToken(r), s, id)
	if vendorDocumentFail(w, r, e) {
		return
	}
	w.Header().Set("Content-Type", v.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": v.Filename}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; frame-ancestors 'self'")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	_, _ = w.Write(data)
}
func (h vendorDocumentHandler) upload(w http.ResponseWriter, r *http.Request) {
	s, ok := vendorDocumentSupplier(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, masterDomain.VendorDocumentMaximumBytes+(64<<10))
	reader, e := r.MultipartReader()
	if e != nil {
		writeJSON(w, 422, map[string]string{"detail": "Multipart upload is required"})
		return
	}
	var in masterDomain.VendorDocumentInput
	var reason string
	seen := map[string]bool{}
	for {
		part, e := reader.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			writeJSON(w, 422, map[string]string{"detail": "Invalid multipart upload"})
			return
		}
		name := part.FormName()
		if seen[name] || (name != "file" && name != "doc_type" && name != "title" && name != "reason" && name != "replaces_document_id") {
			writeJSON(w, 422, map[string]string{"detail": "Unknown or duplicate upload field"})
			return
		}
		seen[name] = true
		limit := int64(2000)
		if name == "file" {
			limit = masterDomain.VendorDocumentMaximumBytes
		}
		data, e := io.ReadAll(io.LimitReader(part, limit+1))
		_ = part.Close()
		if e != nil || int64(len(data)) > limit {
			writeJSON(w, 413, map[string]string{"detail": "Upload exceeds permitted size"})
			return
		}
		switch name {
		case "file":
			in.Content = data
			in.Filename = part.FileName()
		case "doc_type":
			in.DocType = string(data)
		case "title":
			in.Title = string(data)
		case "reason":
			reason = string(data)
		case "replaces_document_id":
			in.ReplacesID = string(data)
		}
	}
	v, e := h.s.Upload(r.Context(), bearerToken(r), s, in, reason)
	if vendorDocumentFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "supplier_documents.uploaded").Str("document_id", v.ID).Msg("Document uploaded")
	writeJSON(w, 201, v)
}
func (h vendorDocumentHandler) archive(w http.ResponseWriter, r *http.Request) {
	s, ok := vendorDocumentSupplier(w, r)
	if !ok {
		return
	}
	var in masterDTO.VendorDocumentArchiveRequest

	if !vendorDocumentDecode(w, r, &in) {
		return
	}
	v, e := h.s.Archive(r.Context(), bearerToken(r), s, chi.URLParam(r, "document_id"), in.Version, in.Reason)
	if vendorDocumentFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "supplier_documents.archived").Str("document_id", v.ID).Msg("Document archived")
	writeJSON(w, 200, v)
}

func VendorDocumentIsUploadPath(path string) bool {
	parts := strings.Split(path, "/")
	return len(parts) == 6 && parts[1] == "api" && parts[2] == "v1" && parts[3] == "suppliers" && masterDomain.VendorDocumentValidID(parts[4]) && parts[5] == "documents"
}

func (h vendorDocumentHandler) reconcileApprovals(w http.ResponseWriter, r *http.Request) {
	supplier, ok := vendorDocumentSupplier(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var input struct{}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&input); e != nil {
		writeJSON(w, 422, map[string]string{"detail": "Send an empty JSON object"})
		return
	}
	var trailing any
	if e := decoder.Decode(&trailing); e != io.EOF {
		writeJSON(w, 422, map[string]string{"detail": "Send one empty JSON object"})
		return
	}
	result, e := h.s.ReconcileApprovals(r.Context(), strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), supplier)
	if vendorDocumentFail(w, r, e) {
		return
	}
	writeJSON(w, 200, masterDTO.DocumentApprovalReconciliationDTO{Processed: result.Processed, Unreadable: result.Unreadable, Replaced: result.Replaced})
}
