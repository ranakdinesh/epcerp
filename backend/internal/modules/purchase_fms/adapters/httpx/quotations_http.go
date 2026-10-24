package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx/dtos"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type quotationsHandler struct {
	s *application.QuotationsService
}

const quotationsBase = "/api/v1/projects/{project}/purchase-bo/{bo}/group-quotations"

func quotationsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "group_quotations.operation.failed", "Quotation operation failed")
}

func quotationsParams(r *http.Request) (string, string) {
	return chi.URLParam(r, "project"), chi.URLParam(r, "bo")
}
func (h quotationsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	p, b := quotationsParams(r)
	section := r.URL.Query().Get("section")
	if section == "" {
		section = "receipt"
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), p, b, section)
	if !quotationsFail(w, r, e) {
		write(w, 200, v)
	}
}
func quotationsDecode(w http.ResponseWriter, r *http.Request) (domain.QuotationsCommand, error) {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 512<<10))
	if e != nil {
		return domain.QuotationsCommand{}, e
	}
	if e = dtos.QuotationsNonNull(raw); e != nil {
		return domain.QuotationsCommand{}, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var req dtos.QuotationsCommandRequest
	if e = d.Decode(&req); e != nil {
		return domain.QuotationsCommand{}, domain.QuotationsInvalid("", "Invalid quotation command")
	}
	if e = d.Decode(&struct{}{}); e != io.EOF {
		return domain.QuotationsCommand{}, domain.QuotationsInvalid("", "Expected one quotation command")
	}
	return req.Command()
}
func (h quotationsHandler) execute(w http.ResponseWriter, r *http.Request) {
	cmd, e := quotationsDecode(w, r)
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
			return
		}
		quotationsFail(w, r, e)
		return
	}
	p, b := quotationsParams(r)
	v, e := h.s.Execute(r.Context(), bearer(r), p, b, cmd)
	if !quotationsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "group_quotations.quote_saved").Str("bo_id", b).Msg("Quotation revision committed")
		write(w, 200, v)
	}
}
func (h quotationsHandler) upload(w http.ResponseWriter, r *http.Request) {
	p, b := quotationsParams(r)
	document := chi.URLParam(r, "document")
	if quotationsFail(w, r, h.s.AuthorizeUpload(r.Context(), bearer(r), p, b, document)) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	reader, e := r.MultipartReader()
	if e != nil {
		quotationsFail(w, r, domain.QuotationsInvalid("file", "Expected one multipart quotation file"))
		return
	}
	part, e := reader.NextPart()
	if e != nil || part.FormName() != "file" || part.FileName() == "" {
		quotationsFail(w, r, domain.QuotationsInvalid("file", "Expected one multipart quotation file"))
		return
	}
	// Read raw filename from Content-Disposition: multipart.FileName silently strips
	// caller path components, which must instead be rejected by the inspector.
	_, fields, e := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if e != nil {
		quotationsFail(w, r, domain.QuotationsInvalid("file", "Invalid file disposition"))
		return
	}
	data, e := io.ReadAll(io.LimitReader(part, (10<<20)+1))
	_ = part.Close()
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			quotationsFail(w, r, domain.QuotationsInvalid("file", "Unable to read quotation evidence"))
		}
		return
	}
	if len(data) > 10<<20 {
		write(w, 413, map[string]string{"detail": "Quotation evidence exceeds 10 MiB"})
		return
	}
	if _, e = reader.NextPart(); e != io.EOF {
		quotationsFail(w, r, domain.QuotationsInvalid("file", "Upload exactly one quotation file"))
		return
	}
	contentType := part.Header.Get("Content-Type")
	v, e := h.s.Upload(r.Context(), bearer(r), p, b, document, fields["filename"], contentType, data)
	if !quotationsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "group_quotations.evidence_uploaded").Str("attachment_id", v.ID).Msg("Quotation evidence committed")
		write(w, 201, v)
	}
}
func (h quotationsHandler) download(w http.ResponseWriter, r *http.Request) {
	p, b := quotationsParams(r)
	v, data, e := h.s.Download(r.Context(), bearer(r), p, b, chi.URLParam(r, "document"), chi.URLParam(r, "file"))
	if quotationsFail(w, r, e) {
		return
	}
	disposition := "attachment"
	if r.URL.Query().Get("inline") == "true" && (v.ContentType == "application/pdf" || strings.HasPrefix(v.ContentType, "image/")) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", v.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": v.Filename}))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}

// UploadGuard must wrap the buffering payload limiter at the composition root.
func QuotationsUploadGuard(s *application.QuotationsService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			if !QuotationsIsUploadPath(r.URL.Path) {
				quotationsFail(w, r, domain.QuotationsMissing)
				return
			}
			if quotationsFail(w, r, s.AuthorizeUpload(r.Context(), bearer(r), parts[3], parts[5], parts[7])) {
				return
			}
			controller := http.NewResponseController(w)
			_ = controller.SetReadDeadline(time.Now().Add(60 * time.Second))
			_ = controller.SetWriteDeadline(time.Now().Add(75 * time.Second))
			next.ServeHTTP(w, r)
		})
	}
}
func QuotationsIsUploadPath(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	return len(p) == 9 && p[0] == "api" && p[1] == "v1" && p[2] == "projects" && p[4] == "purchase-bo" && p[6] == "group-quotations" && p[8] == "attachments" && p[3] != "" && p[5] != "" && p[7] != ""
}
func (h quotationsHandler) boChoices(w http.ResponseWriter, r *http.Request) {
	section := r.URL.Query().Get("section")
	if section == "" {
		section = "receipt"
	}
	v, e := h.s.BOChoices(r.Context(), bearer(r), chi.URLParam(r, "project"), section)
	if !quotationsFail(w, r, e) {
		write(w, 200, v)
	}
}
