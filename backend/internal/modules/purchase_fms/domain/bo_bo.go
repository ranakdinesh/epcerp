package domain

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type BOError = Error

func BOInvalid(f, m string) error { return &BOError{422, f, m} }

var BOUnauthorized = &BOError{401, "", "Invalid or expired credentials"}
var BOForbidden = &BOError{403, "", "BO list permission or project scope denied"}
var BOMissing = &BOError{404, "", "BO list not found"}
var BOStale = &BOError{409, "", "BO list or linked material requirement changed; reload the saved version"}

var bOUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func BOID(s string) (string, error) {
	s = strings.ToLower(s)
	if !bOUuid.MatchString(s) {
		return s, BOInvalid("id", "Use a valid BO record identifier")
	}
	return s, nil
}

var bOQuantity = regexp.MustCompile(`^[0-9]{1,13}(\.[0-9]{1,3})?$`)

func BOQuantity(s string) (int64, error) {
	if !bOQuantity.MatchString(s) {
		return 0, BOInvalid("quantity", "Quantity supports 13 integer digits and at most 3 decimals")
	}
	parts := strings.Split(s, ".")
	digits := parts[0] + "000"
	if len(parts) == 2 {
		digits = parts[0] + parts[1] + strings.Repeat("0", 3-len(parts[1]))
	}
	v, e := strconv.ParseInt(digits, 10, 64)
	if e != nil || v <= 0 {
		return 0, BOInvalid("quantity", "Quantity must be positive")
	}
	return v, nil
}
func BODate(s string) bool {
	v, e := time.Parse("2006-01-02", s)
	return e == nil && v.Year() > 0 && v.Year() <= 9999 && v.Format("2006-01-02") == s
}

type BOSnapshot struct {
	ID             string `json:"id"`
	SKU            string `json:"sku"`
	Description    string `json:"description"`
	EngineeringKey string `json:"engineering_key"`
	UOM            string `json:"uom"`
	Manufacturer   string `json:"manufacturer"`
	DemandVersion  *int32 `json:"demand_version,omitempty"`
}
type BOLine struct {
	ID           string     `json:"id"`
	ItemID       *string    `json:"item_id"`
	Quantity     *string    `json:"quantity"`
	RequiredDate *string    `json:"required_date"`
	Reference    string     `json:"reference"`
	Notes        string     `json:"notes"`
	Snapshot     BOSnapshot `json:"item_snapshot"`
	DemandID     *string    `json:"demand_id"`
}
type BOInputLine struct {
	ID           string  `json:"id"`
	ItemID       *string `json:"item_id"`
	Quantity     *string `json:"quantity"`
	RequiredDate *string `json:"required_date"`
	Reference    string  `json:"reference"`
	Notes        string  `json:"notes"`
}
type BORecord struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	PreparerID  string     `json:"preparer_id"`
	Name        string     `json:"name"`
	Version     int32      `json:"version"`
	State       string     `json:"state"`
	Active      bool       `json:"is_active"`
	Lines       []BOLine   `json:"lines"`
	CompletedAt *time.Time `json:"completed_at"`
	CompletedBy *string    `json:"completed_by_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   *string    `json:"created_by_id"`
	UpdatedBy   *string    `json:"updated_by_id"`
}
type BOSave struct {
	Name    *string       `json:"name"`
	Version int32         `json:"version"`
	Reason  string        `json:"reason"`
	Lines   []BOInputLine `json:"lines"`
}
type BOChange struct {
	Version int32  `json:"version"`
	Reason  string `json:"reason"`
}
type BOEvent struct {
	ID        string    `json:"id"`
	SubjectID string    `json:"subject_id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Before    *BORecord `json:"before"`
	After     BORecord  `json:"after"`
	CreatedAt time.Time `json:"created_at"`
	ActorID   string    `json:"created_by_id"`
}
type BOListing struct {
	Lists       []BORecord `json:"lists"`
	CanWrite    bool       `json:"can_write"`
	CanComplete bool       `json:"can_complete"`
}

func BOReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 2000 {
		return s, BOInvalid("reason", "Provide a reason within 2000 characters")
	}
	return s, nil
}
func (s BOSave) Validate() (BOSave, error) {
	var e error
	s.Reason, e = BOReason(s.Reason)
	if e != nil {
		return s, e
	}
	if s.Version < 0 {
		return s, BOInvalid("version", "Use the current BO version")
	}
	if s.Name != nil {
		n := strings.TrimSpace(*s.Name)
		if n == "" || utf8.RuneCountInString(n) > 128 {
			return s, BOInvalid("name", "BO name must contain 1–128 characters")
		}
		s.Name = &n
	}
	if len(s.Lines) > 100 {
		return s, BOInvalid("lines", "Use at most 100 item rows")
	}
	seen := map[string]bool{}
	for i, line := range s.Lines {
		line.ID, e = BOID(line.ID)
		if e != nil {
			return s, BOInvalid("lines", "Each item row needs a valid identifier")
		}
		if seen[line.ID] {
			return s, BOInvalid("lines", "Use distinct item row identifiers")
		}
		seen[line.ID] = true
		if line.ItemID != nil {
			v, e := BOID(*line.ItemID)
			if e != nil {
				return s, BOInvalid("item_id", "Select a valid catalogue item")
			}
			line.ItemID = &v
		}
		if line.Quantity != nil {
			if _, e = BOQuantity(*line.Quantity); e != nil {
				return s, e
			}
		}
		if line.RequiredDate != nil && !BODate(*line.RequiredDate) {
			return s, BOInvalid("required_date", "Use a valid required-at-site date")
		}
		if utf8.RuneCountInString(line.Reference) > 500 || utf8.RuneCountInString(line.Notes) > 4000 {
			return s, BOInvalid("notes", "Item reference/notes exceed their permitted length")
		}
		s.Lines[i] = line
	}
	return s, nil
}
func BOValidateLine(line BOLine, complete bool) error {
	if complete && (line.ItemID == nil || line.Quantity == nil || line.RequiredDate == nil) {
		return BOInvalid("lines", "Every item needs a catalogue selection, quantity and required-at-site date")
	}
	if line.Quantity != nil {
		q, e := BOQuantity(*line.Quantity)
		if e != nil {
			return e
		}
		switch strings.ToLower(line.Snapshot.UOM) {
		case "nos", "each", "number", "numbers", "pcs", "piece", "pieces":
			if q%1000 != 0 {
				return BOInvalid("quantity", "Counted items require whole-number quantities")
			}
		}
	}
	return nil
}
func BOEditable(r BORecord, a Principal, version int32) error {
	if !r.Active {
		return BOInvalid("", "BO list is inactive")
	}
	if !a.Superuser && r.PreparerID != a.ID {
		return BOForbidden
	}
	if r.Version != version {
		return BOStale
	}
	if r.State != "draft" {
		return BOInvalid("", "Completed BO lists are immutable until explicitly reopened")
	}
	return nil
}
func BOCopy(r BORecord) BORecord { r.Lines = append([]BOLine{}, r.Lines...); return r }

type BOTimestamp = time.Time

// DemandLink is exact published source identity, not financial or actor authority.
type BODemandLink struct {
	BOID      string `json:"bo_id"`
	BOVersion int32  `json:"bo_version"`
	LineID    string `json:"line_id"`
	DemandID  string `json:"demand_id"`
	ItemID    string `json:"item_id"`
	Quantity  string `json:"quantity"`
}

type BOProject struct{ Number, Name string }
type BOPrint struct {
	Project     BOProject
	Records     []BORecord
	Comments    []string
	GeneratedAt BOTimestamp
}
type BOExpected struct {
	ID      string `json:"id"`
	Version int32  `json:"version"`
}
type BORegisterChange struct {
	Records []BOExpected `json:"records"`
	Reason  string       `json:"reason"`
}
