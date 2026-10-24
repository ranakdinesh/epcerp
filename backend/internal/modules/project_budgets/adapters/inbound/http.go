package inbound

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"io"
	"net/http"
	"strings"
)

type handler struct{ s *application.Service }

func Register(r chi.Router, s *application.Service) {
	h := handler{s}
	r.Get("/api/v1/projects/{project}/budget", h.read)
	r.Post("/api/v1/projects/{project}/budget", h.save)
}
func token(r *http.Request) string {
	p := strings.Fields(r.Header.Get("Authorization"))
	if len(p) == 2 && strings.EqualFold(p[0], "Bearer") {
		return p[1]
	}
	return ""
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
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
		if d.Field != "" {
			write(w, d.Status, map[string]any{"detail": []map[string]any{{"loc": append([]string{"body"}, strings.Split(d.Field, ".")...), "msg": d.Message, "type": "value_error"}}})
		} else {
			write(w, d.Status, map[string]string{"detail": d.Message})
		}
	} else {
		logging.Ctx(r.Context()).Error().Str("event", "project_budgets.operation.failed").Msg("Project budget operation failed")
		write(w, 500, map[string]string{"detail": "Internal server error"})
	}
	return true
}
func (h handler) read(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Workspace(r.Context(), token(r), chi.URLParam(r, "project"))
	if !fail(w, r, e) {
		write(w, 200, v)
	}
}
func (h handler) save(w http.ResponseWriter, r *http.Request) {
	v, e := decode(w, r)
	if fail(w, r, e) {
		return
	}
	result, e := h.s.Execute(r.Context(), token(r), chi.URLParam(r, "project"), v)
	if fail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "project_budgets.command.committed").Str("action", v.Action).Str("project_id", result.Project.ID).Int32("version", result.Record.Version).Msg("Project budget command committed")
	write(w, 200, result)
}

// Input types deliberately exclude server audit fields and retained source snapshots.
type headInput struct {
	ID     string  `json:"id"`
	Parent *string `json:"parent_id"`
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Amount Decimal `json:"amount"`
	Owner  string  `json:"owner_id"`
	Active bool    `json:"is_active"`
}
type allocationInput struct {
	ID        string   `json:"id"`
	BO        string   `json:"bo_id"`
	Line      string   `json:"line_id"`
	Head      string   `json:"head_id"`
	Amount    Decimal  `json:"amount"`
	Owner     string   `json:"owner_id"`
	Active    bool     `json:"is_active"`
	UnitPrice *Decimal `json:"unit_price"`
	Quantity  *Decimal `json:"pricing_quantity"`
}
type dataInput struct {
	Total       Decimal           `json:"total"`
	Currency    string            `json:"currency"`
	Owner       string            `json:"owner_id"`
	Notes       string            `json:"notes"`
	PricingMode string            `json:"pricing_mode"`
	Heads       []headInput       `json:"heads"`
	Allocations []allocationInput `json:"allocations"`
}
type priceInput struct {
	BO        string   `json:"bo_id"`
	Line      string   `json:"line_id"`
	Version   int32    `json:"bo_version"`
	Quantity  Decimal  `json:"quantity"`
	UnitPrice *Decimal `json:"unit_price"`
	Changed   bool     `json:"changed"`
}
type commandInput struct {
	Action  string       `json:"action"`
	Version int32        `json:"version"`
	Reason  string       `json:"reason"`
	Data    *dataInput   `json:"data"`
	Prices  []priceInput `json:"prices"`
}

func decimalPointer(v *Decimal) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}
func (v commandInput) command() domain.Command {
	c := domain.Command{Action: v.Action, Version: v.Version, Reason: v.Reason, Prices: []domain.Price{}}
	if v.Data != nil {
		d := v.Data
		c.Data = &domain.Data{Total: string(d.Total), Currency: d.Currency, OwnerID: d.Owner, Notes: d.Notes, PricingMode: d.PricingMode, Heads: []domain.Head{}, Allocations: []domain.Allocation{}}
		for _, h := range d.Heads {
			c.Data.Heads = append(c.Data.Heads, domain.Head{ID: h.ID, ParentID: h.Parent, Name: h.Name, Kind: h.Kind, Amount: string(h.Amount), OwnerID: h.Owner, Active: h.Active})
		}
		for _, a := range d.Allocations {
			c.Data.Allocations = append(c.Data.Allocations, domain.Allocation{ID: a.ID, BOID: a.BO, LineID: a.Line, HeadID: a.Head, Amount: string(a.Amount), OwnerID: a.Owner, Active: a.Active, UnitPrice: decimalPointer(a.UnitPrice), PricingQuantity: decimalPointer(a.Quantity)})
		}
	}
	for _, p := range v.Prices {
		c.Prices = append(c.Prices, domain.Price{BOID: p.BO, LineID: p.Line, BOVersion: p.Version, Quantity: string(p.Quantity), UnitPrice: decimalPointer(p.UnitPrice), Changed: p.Changed})
	}
	return c
}
func object(raw json.RawMessage, path string, required []string, nullable ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, domain.Invalid(path, "Expected one JSON object")
	}
	field := func(k string) string {
		if path == "" {
			return k
		}
		return path + "." + k
	}
	for _, k := range required {
		if _, ok := fields[k]; !ok {
			return nil, domain.Invalid(field(k), "This field is required")
		}
	}
	for k, v := range fields {
		if strings.TrimSpace(string(v)) == "null" {
			allowed := false
			for _, n := range nullable {
				if n == k {
					allowed = true
				}
			}
			if !allowed {
				return nil, domain.Invalid(field(k), "This field must not be null")
			}
		}
	}
	return fields, nil
}
func children(raw json.RawMessage, path string, max int, required []string, nullable ...string) error {
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil || rows == nil {
		return domain.Invalid(path, "Expected an array")
	}
	if len(rows) > max {
		return domain.Invalid(path, "Too many records")
	}
	for _, row := range rows {
		if _, e := object(row, path, required, nullable...); e != nil {
			return e
		}
	}
	return nil
}
func decode(w http.ResponseWriter, r *http.Request) (domain.Command, error) {
	var v commandInput
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 512<<10))
	if e != nil {
		var size *http.MaxBytesError
		if errors.As(e, &size) {
			return domain.Command{}, &domain.Error{Status: 413, Message: "Request body too large"}
		}
		return domain.Command{}, domain.Invalid("", "Unable to read input")
	}
	f, e := object(raw, "", []string{"action", "version", "reason"}, "data")
	if e != nil {
		return domain.Command{}, e
	}
	if d, ok := f["data"]; ok && strings.TrimSpace(string(d)) != "null" {
		fields, e := object(d, "data", []string{"total", "currency", "owner_id", "heads", "allocations"})
		if e != nil {
			return domain.Command{}, e
		}
		if e = children(fields["heads"], "data.heads", 100, []string{"id", "name", "kind", "amount", "owner_id", "is_active"}, "parent_id"); e != nil {
			return domain.Command{}, e
		}
		if e = children(fields["allocations"], "data.allocations", 1000, []string{"id", "bo_id", "line_id", "head_id", "amount", "owner_id", "is_active"}, "unit_price", "pricing_quantity"); e != nil {
			return domain.Command{}, e
		}
	}
	if p, ok := f["prices"]; ok {
		if e = children(p, "prices", 1000, []string{"bo_id", "line_id", "bo_version", "quantity", "changed"}, "unit_price"); e != nil {
			return domain.Command{}, e
		}
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if e = d.Decode(&v); e != nil {
		return domain.Command{}, domain.Invalid("", "Invalid budget input")
	}
	// Financial commands refer to the saved source version; browser data cannot override it.
	if v.Action != "save" && v.Action != "price_items" && v.Action != "approve" && v.Action != "revise" {
		return domain.Command{}, domain.Invalid("action", "Choose save, price_items, approve or revise")
	}
	if v.Action == "approve" || v.Action == "revise" {
		if _, ok := f["data"]; ok {
			return domain.Command{}, domain.Invalid("data", "Financial commands use saved budget data")
		}
		if _, ok := f["prices"]; ok {
			return domain.Command{}, domain.Invalid("prices", "Financial commands use saved item budgets")
		}
	}
	if v.Action == "save" && len(v.Prices) > 0 {
		return domain.Command{}, domain.Invalid("prices", "Use price_items to update unit budgets")
	}
	if v.Action == "price_items" && v.Data != nil {
		return domain.Command{}, domain.Invalid("data", "Use save to update budget heads")
	}
	return v.command().Validate()
}
