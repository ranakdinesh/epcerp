package domain

import (
	"slices"
	"strings"
)

type WorkflowPageFilter struct {
	Page                    int32
	Size                    int32
	Search, Sort, Direction string
}

func (f WorkflowPageFilter) Validate() (WorkflowPageFilter, error) {
	if f.Page == 0 {
		f.Page = 1
	}
	if f.Size == 0 {
		f.Size = 20
	}
	if f.Sort == "" {
		f.Sort = "code"
	}
	if f.Direction == "" {
		f.Direction = "asc"
	}
	if f.Page < 1 || f.Page > 100000 || !slices.Contains([]int32{20, 50, 100}, f.Size) {
		return f, WorkflowInvalid("page", "Use a valid page and 20/50/100 rows")
	}
	if !slices.Contains([]string{"code", "description", "updated"}, f.Sort) || !slices.Contains([]string{"asc", "desc"}, f.Direction) {
		return f, WorkflowInvalid("sort", "Use a supported sort field and direction")
	}
	var e error
	f.Search, e = WorkflowText("search", f.Search, 120, false)
	return f, e
}
func (f WorkflowPageFilter) Pattern() string {
	s := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.Search)
	return "%" + s + "%"
}

type WorkflowSummary struct {
	Supplier       string            `json:"supplier"`
	LOIState       string            `json:"loi_state"`
	TechnicalState string            `json:"technical_state"`
	OrderState     string            `json:"order_state"`
	ClientPOState  string            `json:"client_po_state"`
	ID             string            `json:"id"`
	Code           string            `json:"code"`
	Description    string            `json:"description"`
	Quantity       string            `json:"quantity"`
	UOM            string            `json:"uom"`
	Version        int32             `json:"version"`
	UpdatedAt      WorkflowTimestamp `json:"updated_at"`
}
