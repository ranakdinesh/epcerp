package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type Error struct {
	Status         int
	Field, Message string
}

func (e *Error) Error() string  { return e.Message }
func Invalid(f, m string) error { return &Error{422, f, m} }

var Forbidden = &Error{403, "", "Budget permission or project scope denied"}
var Unauthorized = &Error{401, "", "Invalid or expired credentials"}
var Missing = &Error{404, "", "Project budget not found"}
var Stale = &Error{409, "", "Budget, purchase quantity or BO version changed; reload before saving"}
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ID(s string) (string, error) {
	s = strings.ToLower(s)
	if !uuid.MatchString(s) {
		return s, Invalid("id", "Use a valid budget record identifier")
	}
	return s, nil
}

type Timestamp = time.Time
type Actor struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}

func (a Actor) Can(code string) bool {
	if !a.Active || a.ID == "" {
		return false
	}
	if a.Superuser {
		return true
	}
	for _, p := range a.Permissions {
		if p == code {
			return true
		}
	}
	return false
}

type Audit struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *string   `json:"created_by_id"`
	UpdatedBy *string   `json:"updated_by_id"`
}
type Head struct {
	Audit
	ID       string  `json:"id"`
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Amount   string  `json:"amount"`
	OwnerID  string  `json:"owner_id"`
	Active   bool    `json:"is_active"`
}
type Item struct {
	BOID         string       `json:"bo_id"`
	LineID       string       `json:"line_id"`
	BOVersion    int32        `json:"bo_version"`
	BOName       string       `json:"bo_name"`
	Quantity     string       `json:"quantity"`
	SKU          string       `json:"sku"`
	Description  string       `json:"description"`
	UOM          string       `json:"uom"`
	Manufacturer string       `json:"manufacturer"`
	ItemID       string       `json:"item_id"`
	Snapshot     ItemSnapshot `json:"item_snapshot"`
}

type ItemSnapshot struct {
	SKU          string `json:"sku"`
	Description  string `json:"description"`
	UOM          string `json:"uom"`
	Manufacturer string `json:"manufacturer"`
}

func (i Item) Key() string { return i.BOID + ":" + i.LineID }

type Allocation struct {
	Audit
	ID              string  `json:"id"`
	BOID            string  `json:"bo_id"`
	LineID          string  `json:"line_id"`
	HeadID          string  `json:"head_id"`
	Amount          string  `json:"amount"`
	OwnerID         string  `json:"owner_id"`
	Active          bool    `json:"is_active"`
	UnitPrice       *string `json:"unit_price"`
	PricingQuantity *string `json:"pricing_quantity"`
	Source          *Item   `json:"source"`
}

func (a Allocation) Key() string { return a.BOID + ":" + a.LineID }

type Data struct {
	Total       string       `json:"total"`
	Currency    string       `json:"currency"`
	OwnerID     string       `json:"owner_id"`
	Notes       string       `json:"notes"`
	PricingMode string       `json:"pricing_mode,omitempty"`
	Heads       []Head       `json:"heads"`
	Allocations []Allocation `json:"allocations"`
}
type Approved struct {
	Revision   int32     `json:"revision"`
	Data       Data      `json:"data"`
	ApprovedAt time.Time `json:"approved_at"`
	ApprovedBy string    `json:"approved_by_id"`
	DecisionID string    `json:"decision_id"`
}
type Budget struct {
	Audit
	ProjectID string    `json:"project_id"`
	Version   int32     `json:"version"`
	Revision  int32     `json:"revision"`
	State     string    `json:"state"`
	Data      Data      `json:"data"`
	Approved  *Approved `json:"approved"`
	Active    bool      `json:"is_active"`
}
type Price struct {
	BOID      string  `json:"bo_id"`
	LineID    string  `json:"line_id"`
	BOVersion int32   `json:"bo_version"`
	Quantity  string  `json:"quantity"`
	UnitPrice *string `json:"unit_price"`
	Changed   bool    `json:"changed"`
}

func (p Price) Key() string { return p.BOID + ":" + p.LineID }

type Command struct {
	Action  string  `json:"action"`
	Version int32   `json:"version"`
	Reason  string  `json:"reason"`
	Data    *Data   `json:"data"`
	Prices  []Price `json:"prices"`
}
type Owner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Project struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Currency      string  `json:"currency"`
	ContractValue *string `json:"contract_value"`
	Available     bool    `json:"available"`
}
type Scope struct {
	Project Project
	Owners  []Owner
	Items   []Item
}
type Capabilities struct{ CanWrite, CanApprove bool }
type HeadReport struct {
	Head
	Unallocated string `json:"unallocated"`
}
type Report struct {
	Total           string       `json:"total"`
	AllocatedHeads  string       `json:"allocated_heads"`
	PurchaseTotal   string       `json:"purchase_total"`
	Unallocated     string       `json:"unallocated"`
	ItemAllocations string       `json:"item_allocations"`
	Heads           []HeadReport `json:"heads"`
}
type Event struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Before    *Budget   `json:"before"`
	After     Budget    `json:"after"`
	ActorID   string    `json:"created_by_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Workspace struct {
	Record         *Budget `json:"record"`
	Project        Project `json:"project"`
	Owners         []Owner `json:"owners"`
	Items          []Item  `json:"items"`
	CanWrite       bool    `json:"can_write"`
	CanApprove     bool    `json:"can_approve"`
	Report         *Report `json:"report"`
	ApprovedReport *Report `json:"approved_report"`
	History        []Event `json:"history"`
}

func (c Command) Validate() (Command, error) {
	if c.Version < 0 {
		return c, Invalid("version", "Use the current budget version")
	}
	if c.Action != "save" && c.Action != "price_items" && c.Action != "approve" && c.Action != "revise" {
		return c, Invalid("action", "Choose save, price items, approve or revise")
	}
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Reason == "" || utf8.RuneCountInString(c.Reason) > 2000 {
		return c, Invalid("reason", "Record a budget change reason of at most 2000 characters")
	}
	if c.Action == "save" && c.Data == nil {
		return c, Invalid("data", "Provide budget data")
	}
	if len(c.Prices) > 1000 {
		return c, Invalid("prices", "At most 1000 purchase item prices per budget")
	}
	c.Prices = append([]Price{}, c.Prices...)
	seen := map[string]bool{}
	for n := range c.Prices {
		p := &c.Prices[n]
		var e error
		if p.BOID, e = ID(p.BOID); e != nil {
			return c, e
		}
		if p.LineID, e = ID(p.LineID); e != nil {
			return c, e
		}
		if p.BOVersion < 1 {
			return c, Invalid("bo_version", "Use the current BO version")
		}
		if _, e = Qty(p.Quantity); e != nil {
			return c, e
		}
		if p.UnitPrice != nil {
			if _, e = Money(*p.UnitPrice); e != nil {
				return c, e
			}
		}
		if seen[p.Key()] {
			return c, Invalid("prices", "Provide each purchase item once")
		}
		seen[p.Key()] = true
	}
	return c, nil
}
func CopyData(d Data) Data {
	d.Heads = append([]Head{}, d.Heads...)
	d.Allocations = append([]Allocation{}, d.Allocations...)
	return d
}
func Copy(b Budget) Budget {
	b.Data = CopyData(b.Data)
	if b.Approved != nil {
		v := *b.Approved
		v.Data = CopyData(v.Data)
		b.Approved = &v
	}
	return b
}
