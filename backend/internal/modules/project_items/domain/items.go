package domain

import (
	"regexp"
	"strconv"
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

var Unauthorized = &Error{401, "", "Invalid or expired credentials"}
var Forbidden = &Error{403, "", "Project material permission or purchasing scope denied"}
var Missing = &Error{404, "", "Project material requirement not found"}
var Stale = &Error{409, "", "Project requirement changed; reload before saving"}

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

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ID(s string) (string, error) {
	s = strings.ToLower(s)
	if !uuid.MatchString(s) {
		return s, Invalid("id", "Use a valid record identifier")
	}
	return s, nil
}

var quantity = regexp.MustCompile(`^[0-9]{1,13}(\.[0-9]{1,3})?$`)

// Quantities are exact thousandths, bounded to Python's 16 digits / 3 decimals.
func Quantity(s string) (int64, error) {
	if !quantity.MatchString(s) {
		return 0, Invalid("quantity", "Quantity supports 13 integer digits and at most 3 decimals")
	}
	parts := strings.Split(s, ".")
	digits := parts[0] + "000"
	if len(parts) == 2 {
		digits = parts[0] + parts[1] + strings.Repeat("0", 3-len(parts[1]))
	}
	v, e := strconv.ParseInt(digits, 10, 64)
	if e != nil || v <= 0 {
		return 0, Invalid("quantity", "Quantity must be positive")
	}
	return v, nil
}
func Date(s string) bool {
	v, e := time.Parse("2006-01-02", s)
	return e == nil && v.Year() > 0 && v.Year() <= 9999 && v.Format("2006-01-02") == s
}
func WholeUnit(u string) bool {
	switch strings.ToLower(strings.TrimSpace(u)) {
	case "nos", "no", "pcs", "pieces", "each":
		return true
	}
	return false
}

type Snapshot struct {
	ID                string `json:"id"`
	SKU               string `json:"sku"`
	Description       string `json:"description"`
	EngineeringKey    string `json:"engineering_key"`
	UOM               string `json:"uom"`
	Manufacturer      string `json:"manufacturer"`
	ManufacturerID    string `json:"manufacturer_id"`
	CategoryCode      string `json:"category_code"`
	FamilyCode        string `json:"family_code"`
	ConfigurationCode string `json:"configuration_code"`
}
type Allocation struct {
	Location string `json:"location"`
	Quantity string `json:"quantity"`
}
type Record struct {
	ID             string       `json:"id"`
	ProjectID      string       `json:"project_id"`
	ItemID         string       `json:"item_id"`
	EngineeringKey string       `json:"engineering_key"`
	Snapshot       Snapshot     `json:"item_snapshot"`
	Quantity       string       `json:"quantity"`
	RequiredDate   string       `json:"required_date"`
	Allocations    []Allocation `json:"allocations"`
	Alternatives   []string     `json:"alternatives"`
	Active         bool         `json:"is_active"`
	Version        int32        `json:"version"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	CreatedBy      *string      `json:"created_by_id"`
	UpdatedBy      *string      `json:"updated_by_id"`
}
type Save struct {
	ItemID       string       `json:"item_id"`
	Quantity     string       `json:"quantity"`
	RequiredDate string       `json:"required_date"`
	Allocations  []Allocation `json:"allocations"`
	Alternatives []string     `json:"alternatives"`
	Active       bool         `json:"is_active"`
	Version      int32        `json:"version"`
	Reason       string       `json:"reason"`
}
type Event struct {
	ID        string    `json:"id"`
	SubjectID string    `json:"subject_id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Before    *Record   `json:"before"`
	After     Record    `json:"after"`
	CreatedAt time.Time `json:"created_at"`
	ActorID   string    `json:"created_by_id"`
}

func (s Save) Validate() (Save, error) {
	var e error
	if s.ItemID, e = ID(s.ItemID); e != nil {
		return s, Invalid("item_id", "Select a catalogue item")
	}
	q, e := Quantity(s.Quantity)
	if e != nil {
		return s, e
	}
	if !Date(s.RequiredDate) {
		return s, Invalid("required_date", "Use a valid required-at-site date")
	}
	if s.Version < 1 {
		return s, Invalid("version", "Use the current material requirement version")
	}
	s.Reason = strings.TrimSpace(s.Reason)
	if s.Reason == "" || utf8.RuneCountInString(s.Reason) > 2000 {
		return s, Invalid("reason", "Provide a reason within 2000 characters")
	}
	if len(s.Allocations) > 100 || len(s.Alternatives) > 100 {
		return s, Invalid("allocations", "Use at most 100 allocations and alternatives")
	}
	total := int64(0)
	locations := map[string]bool{}
	for i, a := range s.Allocations {
		a.Location = strings.TrimSpace(a.Location)
		if a.Location == "" || utf8.RuneCountInString(a.Location) > 255 {
			return s, Invalid("allocations", "Enter a location within 255 characters")
		}
		key := strings.ToLower(a.Location)
		if locations[key] {
			return s, Invalid("allocations", "Each location must occur once")
		}
		locations[key] = true
		amount, e := Quantity(a.Quantity)
		if e != nil {
			return s, Invalid("allocations", "Allocation quantities must be positive with at most 3 decimals")
		}
		total += amount
		s.Allocations[i] = a
	}
	if len(s.Allocations) > 0 && total != q {
		return s, Invalid("allocations", "Location allocations must total the project item quantity")
	}
	seen := map[string]bool{s.ItemID: true}
	for i, id := range s.Alternatives {
		v, e := ID(id)
		if e != nil {
			return s, Invalid("alternatives", "Select valid alternative items")
		}
		if seen[v] {
			return s, Invalid("alternatives", "Duplicate or primary item in alternatives")
		}
		seen[v] = true
		s.Alternatives[i] = v
	}
	if s.Allocations == nil {
		s.Allocations = []Allocation{}
	}
	if s.Alternatives == nil {
		s.Alternatives = []string{}
	}
	return s, nil
}
func (s Save) ValidateUnit(unit string) error {
	if !WholeUnit(unit) {
		return nil
	}
	values := []string{s.Quantity}
	for _, a := range s.Allocations {
		values = append(values, a.Quantity)
	}
	for _, v := range values {
		q, e := Quantity(v)
		if e != nil {
			return e
		}
		if q%1000 != 0 {
			return Invalid("quantity", unit+" quantities must be whole numbers")
		}
	}
	return nil
}
func CommitmentsGuard(quantity string, active bool, committed string) error {
	if committed == "" || committed == "0" || committed == "0.000" {
		return nil
	}
	c, e := Quantity(committed)
	if e != nil {
		return e
	}
	q, e := Quantity(quantity)
	if e != nil {
		return e
	}
	if !active || q < c {
		return Invalid("quantity", "Requirement cannot be reduced below released purchasing quantity or deactivated")
	}
	return nil
}

// CatalogueChoice is a project-scoped, live eligible item selection; issued SKU
// and configuration evidence remain owned by the catalogue capability.
type CatalogueChoice struct {
	ID             string `json:"id"`
	SKU            string `json:"sku"`
	Description    string `json:"description"`
	EngineeringKey string `json:"engineering_key"`
	UOM            string `json:"uom"`
	Manufacturer   string `json:"manufacturer"`
	Category       string `json:"category"`
	Configuration  string `json:"configuration"`
	Model          string `json:"model"`
}
