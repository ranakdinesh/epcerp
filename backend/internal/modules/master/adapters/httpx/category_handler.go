package httpx

import (
	"encoding/json"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"io"
	"net/http"
	"strconv"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type categoryHandler struct {
	s *masterApplication.CategoryService
}

func RegisterCategoryRoutes(r chi.Router, s *masterApplication.CategoryService) {
	h := categoryHandler{s}
	r.Get("/api/v1/categories", h.list)
	r.Get("/api/v1/categories/browse", h.browse)
	r.Get("/api/v1/categories/detail/{kind}/{identity}", h.detail)
	r.Post("/api/v1/categories/import", h.importRows)
	r.Post("/api/v1/categories/categories", h.create("category"))
	r.Post("/api/v1/categories/subcategories", h.create("subcategory"))
	r.Post("/api/v1/categories/units", h.create("unit"))
	r.Patch("/api/v1/categories/categories/{identity}", h.edit("category"))
	r.Patch("/api/v1/categories/subcategories/{identity}", h.edit("subcategory"))
	r.Patch("/api/v1/categories/units/{identity}", h.edit("unit"))
}

func categoryFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "categories.operation.failed")
}

func (h categoryHandler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), bearerToken(r))
	if !categoryFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h categoryHandler) importRows(w http.ResponseWriter, r *http.Request) {
	var body masterDTO.CategoryImportRowsRequest

	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if e := masterDTO.DecodeMasterJSON(d, &body); e != nil {
		var limit *http.MaxBytesError
		if errors.As(e, &limit) {
			writeJSON(w, 413, map[string]string{"detail": "Import exceeds 1 MiB"})
		} else {
			writeJSON(w, 422, map[string]string{"detail": "Invalid import JSON or unknown field"})
		}
		return
	}
	var extra any
	if masterDTO.DecodeMasterJSON(d, &extra) != io.EOF {
		writeJSON(w, 422, map[string]string{"detail": "One JSON object is required"})
		return
	}
	rows := make([]masterDomain.CategorySource, 0, len(body.Records))
	for _, row := range body.Records {
		if row.Active == nil {
			writeJSON(w, 422, map[string]string{"detail": "Source is_active is required"})
			return
		}
		row.CategorySourceDTO.Active = *row.Active
		rows = append(rows, row.CategorySourceDTO.Domain())
	}
	v, e := h.s.Import(r.Context(), bearerToken(r), rows)
	if categoryFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "categories.import.committed").Int("categories", v.Categories).Int("subcategories", v.Subcategories).Int("units", v.Units).Msg("Catalogue import committed")
	writeJSON(w, 200, v)
}

func (h categoryHandler) create(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in masterDomain.CategoryCreate
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		d.DisallowUnknownFields()
		if e := masterDTO.DecodeMasterJSON(d, &in); e != nil {
			var limit *http.MaxBytesError
			if errors.As(e, &limit) {
				writeJSON(w, 413, map[string]string{"detail": "Request exceeds 16 KiB"})
			} else {
				writeJSON(w, 422, map[string]string{"detail": "Invalid JSON or unknown field"})
			}
			return
		}
		var extra any
		if masterDTO.DecodeMasterJSON(d, &extra) != io.EOF {
			writeJSON(w, 422, map[string]string{"detail": "One JSON object is required"})
			return
		}
		if kind == "subcategory" && in.UnitID != "" {
			writeJSON(w, 422, map[string]string{"detail": "Subcategories inherit the category unit"})
			return
		}
		v, e := h.s.Create(r.Context(), bearerToken(r), kind, in)
		if categoryFail(w, r, e) {
			return
		}
		logging.Ctx(r.Context()).Info().Str("event", "categories.create.committed").Str("record_id", v.ID).Str("kind", kind).Msg("Catalogue record created")
		writeJSON(w, 201, v)
	}
}

func (h categoryHandler) edit(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in masterDomain.CategoryEdit
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		d.DisallowUnknownFields()
		if e := masterDTO.DecodeMasterJSON(d, &in); e != nil {
			var limit *http.MaxBytesError
			if errors.As(e, &limit) {
				writeJSON(w, 413, map[string]string{"detail": "Request exceeds 16 KiB"})
			} else {
				writeJSON(w, 422, map[string]string{"detail": "Invalid JSON or unknown field"})
			}
			return
		}
		var extra any
		if masterDTO.DecodeMasterJSON(d, &extra) != io.EOF {
			writeJSON(w, 422, map[string]string{"detail": "One JSON object is required"})
			return
		}
		v, e := h.s.Edit(r.Context(), bearerToken(r), kind, chi.URLParam(r, "identity"), in)
		if categoryFail(w, r, e) {
			return
		}
		logging.Ctx(r.Context()).Info().Str("event", "categories.update.committed").Str("record_id", v.ID).Int32("version", v.Version).Msg("Catalogue correction committed")
		writeJSON(w, 200, v)
	}
}

func (h categoryHandler) browse(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	allowed := map[string]bool{"kind": true, "category_id": true, "search": true, "status": true, "sort": true, "direction": true, "page": true, "size": true}
	for k, values := range v {
		if !allowed[k] || len(values) != 1 {
			writeJSON(w, 422, map[string]string{"detail": "Unknown or repeated query parameter"})
			return
		}
	}
	integer := func(key string, fallback int32) (int32, error) {
		if v.Get(key) == "" {
			return fallback, nil
		}
		n, e := strconv.ParseInt(v.Get(key), 10, 32)
		return int32(n), e
	}
	page, e := integer("page", 1)
	if e != nil {
		writeJSON(w, 422, map[string]string{"detail": "Invalid page"})
		return
	}
	size, e := integer("size", 20)
	if e != nil {
		writeJSON(w, 422, map[string]string{"detail": "Invalid size"})
		return
	}
	q := masterDomain.CategoryBrowse{Kind: v.Get("kind"), CategoryID: v.Get("category_id"), Search: v.Get("search"), Status: v.Get("status"), Sort: v.Get("sort"), Direction: v.Get("direction"), Page: page, Size: size}
	if q.Kind == "" {
		q.Kind = "category"
	}
	if q.Sort == "" {
		q.Sort = "name"
	}
	if q.Direction == "" {
		q.Direction = "asc"
	}
	out, e := h.s.Browse(r.Context(), bearerToken(r), q)
	if !categoryFail(w, r, e) {
		writeJSON(w, 200, out)
	}
}
func (h categoryHandler) detail(w http.ResponseWriter, r *http.Request) {
	out, e := h.s.Detail(r.Context(), bearerToken(r), chi.URLParam(r, "kind"), chi.URLParam(r, "identity"))
	if !categoryFail(w, r, e) {
		writeJSON(w, 200, out)
	}
}
