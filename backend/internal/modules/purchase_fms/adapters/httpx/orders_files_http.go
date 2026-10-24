package httpx

import (
	"github.com/go-chi/chi/v5"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func ordersUploadVersion(s string) (int32, error) {
	v, e := strconv.ParseInt(s, 10, 32)
	if e != nil || v < 1 || strconv.FormatInt(v, 10) != s {
		return 0, domain.OrdersInvalid("version", "Use the current PO version")
	}
	return int32(v), nil
}
func (h ordersHandler) upload(w http.ResponseWriter, r *http.Request) {
	if !ordersNoQuery(w, r) {
		return
	}
	v, e := ordersUploadVersion(chi.URLParam(r, "version"))
	if ordersFail(w, r, e) {
		return
	}
	p, id := chi.URLParam(r, "project"), chi.URLParam(r, "package")
	if ordersFail(w, r, h.s.AuthorizeClientUpload(r.Context(), bearer(r), p, id, v)) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	reader, e := r.MultipartReader()
	if e != nil {
		ordersFail(w, r, domain.OrdersInvalid("file", "Upload one signed PDF, PNG or JPEG"))
		return
	}
	part, e := reader.NextPart()
	if e != nil {
		ordersFail(w, r, domain.OrdersInvalid("file", "Upload one signed file"))
		return
	}
	_, fields, e := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if e != nil || fields["name"] != "file" {
		ordersFail(w, r, domain.OrdersInvalid("file", "Use the file upload field"))
		return
	}
	data, e := io.ReadAll(io.LimitReader(part, domain.TechnicalMaxFileBytes+1))
	_ = part.Close()
	if ordersFail(w, r, e) {
		return
	}
	if len(data) > domain.TechnicalMaxFileBytes {
		write(w, 413, map[string]string{"detail": "Signed file must be at most 10 MiB"})
		return
	}
	if _, e = reader.NextPart(); e != io.EOF {
		ordersFail(w, r, domain.OrdersInvalid("file", "Upload exactly one signed file"))
		return
	}
	f, e := h.s.UploadClientFile(r.Context(), bearer(r), p, id, v, fields["filename"], part.Header.Get("Content-Type"), data)
	if !ordersFail(w, r, e) {
		write(w, 201, f)
	}
}
func (h ordersHandler) file(w http.ResponseWriter, r *http.Request) {
	if !ordersNoQuery(w, r) {
		return
	}
	f, data, e := h.s.ClientFile(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), chi.URLParam(r, "file"))
	if ordersFail(w, r, e) {
		return
	}
	w.Header().Set("Content-Type", f.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": f.Name}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func OrdersIsUploadPath(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	return len(p) == 9 && p[0] == "api" && p[1] == "v1" && p[2] == "projects" && p[4] == "purchase-workflow" && p[6] == "purchase-orders" && p[7] == "uploads"
}
func OrdersUploadGuard(s *application.OrdersService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ordersNoQuery(w, r) {
				return
			}
			if !OrdersIsUploadPath(r.URL.Path) {
				ordersFail(w, r, domain.OrdersMissing)
				return
			}
			p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			v, e := ordersUploadVersion(p[8])
			if ordersFail(w, r, e) {
				return
			}
			if ordersFail(w, r, s.AuthorizeClientUpload(r.Context(), bearer(r), p[3], p[5], v)) {
				return
			}
			c := http.NewResponseController(w)
			_ = c.SetReadDeadline(time.Now().Add(60 * time.Second))
			_ = c.SetWriteDeadline(time.Now().Add(75 * time.Second))
			next.ServeHTTP(w, r)
		})
	}
}

func (h ordersHandler) uploadAuthority(w http.ResponseWriter, r *http.Request) {
	if !ordersNoQuery(w, r) {
		return
	}
	v, e := ordersUploadVersion(chi.URLParam(r, "version"))
	if ordersFail(w, r, e) {
		return
	}
	if !ordersFail(w, r, h.s.AuthorizeClientUpload(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), v)) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(204)
	}
}
