package domain

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type IndentsError = Error

func IndentsInvalid(f, m string) error { return &IndentsError{422, f, m} }

var IndentsUnauthorized = &IndentsError{401, "", "Invalid or expired credentials"}
var IndentsForbidden = &IndentsError{403, "", "Purchase indent permission or project scope denied"}
var IndentsMissing = &IndentsError{404, "", "Purchase indent not found"}
var IndentsStale = &IndentsError{409, "", "Purchase indent or linked material requirement changed; reload the saved version"}

var indentsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func IndentsID(s string) (string, error) {
	s = strings.ToLower(s)
	if !indentsUuid.MatchString(s) {
		return s, IndentsInvalid("id", "Use a valid indent record identifier")
	}
	return s, nil
}

var indentsQuantity = regexp.MustCompile(`^[0-9]{1,13}(\.[0-9]{1,3})?$`)

func IndentsQuantity(s string) (int64, error) {
	if !indentsQuantity.MatchString(s) {
		return 0, IndentsInvalid("quantity", "Quantity supports 13 integer digits and at most 3 decimals")
	}
	parts := strings.Split(s, ".")
	digits := parts[0] + "000"
	if len(parts) == 2 {
		digits = parts[0] + parts[1] + strings.Repeat("0", 3-len(parts[1]))
	}
	v, e := strconv.ParseInt(digits, 10, 64)
	if e != nil || v <= 0 {
		return 0, IndentsInvalid("quantity", "Quantity must be positive")
	}
	return v, nil
}
func IndentsDate(s string) bool {
	v, e := time.Parse("2006-01-02", s)
	return e == nil && v.Year() > 0 && v.Year() <= 9999 && v.Format("2006-01-02") == s
}

type IndentsTimestamp = time.Time
type IndentsLine struct {
	DemandID string `json:"demand_id"`
	Quantity string `json:"quantity"`
	Budget   string `json:"budget"`
	FullKit  string `json:"full_kit"`
}
type IndentsItemSnapshot struct {
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
type IndentsPolicyEvidence struct {
	ID      string `json:"id"`
	Version int32  `json:"version"`
}
type IndentsPolicy struct {
	Status  string                  `json:"status"`
	Reasons []string                `json:"reasons"`
	Rules   []IndentsPolicyEvidence `json:"rules"`
}
type IndentsAllocation struct {
	Location string `json:"location"`
	Quantity string `json:"quantity"`
}
type IndentsHold struct {
	ID         string    `json:"id"`
	Reason     string    `json:"reason"`
	DoerID     string    `json:"doer_id"`
	ResolverID string    `json:"resolver_id"`
	StartedAt  time.Time `json:"started_at"`
}
type IndentsHolds struct {
	Holds        []IndentsHold `json:"holds"`
	HoldMinutes  float64       `json:"hold_minutes"`
	FinishImpact string        `json:"finish_impact"`
}
type IndentsSource struct {
	Active        bool                `json:"active"`
	Quantity      string              `json:"quantity"`
	DemandVersion int32               `json:"demand_version"`
	Item          IndentsItemSnapshot `json:"item"`
	RequiredDate  string              `json:"required_date"`
	Allocations   []IndentsAllocation `json:"allocations"`
	ClientID      string              `json:"client_id"`
	Policy        IndentsPolicy       `json:"policy"`
	Holds         IndentsHolds        `json:"holds"`
}
type IndentsRelease struct {
	IndentsLine
	At       time.Time     `json:"at"`
	ActorID  string        `json:"actor_id"`
	Evidence string        `json:"evidence"`
	Source   IndentsSource `json:"source"`
}
type IndentsReview struct {
	ActorID        string           `json:"actor_id"`
	At             time.Time        `json:"at"`
	Evidence       string           `json:"evidence"`
	DemandVersions map[string]int32 `json:"demand_versions"`
}
type IndentsInput struct {
	Currency   string        `json:"currency"`
	DoerID     string        `json:"doer_id"`
	ResolverID string        `json:"resolver_id"`
	Lines      []IndentsLine `json:"lines"`
	Documents  string        `json:"documents"`
}
type IndentsData struct {
	IndentsInput
	Number   string           `json:"number"`
	State    string           `json:"state"`
	Released []IndentsRelease `json:"released"`
	Review   *IndentsReview   `json:"review"`
}
type IndentsSave struct {
	Data    IndentsInput `json:"data"`
	Version int32        `json:"version"`
	Reason  string       `json:"reason"`
}
type IndentsAction struct {
	Version  int32    `json:"version"`
	Reason   string   `json:"reason"`
	Evidence string   `json:"evidence"`
	Action   string   `json:"action"`
	LineIDs  []string `json:"line_ids"`
}
type IndentsRecord struct {
	ID        string      `json:"id"`
	ProjectID string      `json:"project_id"`
	Data      IndentsData `json:"data"`
	Version   int32       `json:"version"`
	Active    bool        `json:"is_active"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	CreatedBy *string     `json:"created_by_id"`
	UpdatedBy *string     `json:"updated_by_id"`
}
type IndentsEvent struct {
	ID        string         `json:"id"`
	SubjectID string         `json:"subject_id"`
	Action    string         `json:"action"`
	Reason    string         `json:"reason"`
	Before    *IndentsRecord `json:"before"`
	After     IndentsRecord  `json:"after"`
	CreatedAt time.Time      `json:"created_at"`
	ActorID   string         `json:"created_by_id"`
}

func IndentsText(field, s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 2000 {
		return s, IndentsInvalid(field, "Provide a value within 2000 characters")
	}
	return s, nil
}

var indentsCurrency = regexp.MustCompile(`^[A-Z]{3}$`)
var indentsMoney = regexp.MustCompile(`^[0-9]{1,16}(\.[0-9]{1,2})?$`)

func IndentsMoney(s string) bool { return indentsMoney.MatchString(s) }
func (s IndentsSave) Validate() (IndentsSave, error) {
	var e error
	s.Reason, e = IndentsText("reason", s.Reason)
	if e != nil {
		return s, e
	}
	if s.Version < 1 {
		return s, IndentsInvalid("version", "Use the saved version")
	}
	d := s.Data
	if !indentsCurrency.MatchString(d.Currency) {
		return s, IndentsInvalid("currency", "Use a three-letter currency code")
	}
	d.DoerID, e = IndentsID(d.DoerID)
	if e != nil {
		return s, IndentsInvalid("doer_id", "Select an active doer")
	}
	d.ResolverID, e = IndentsID(d.ResolverID)
	if e != nil {
		return s, IndentsInvalid("resolver_id", "Select an active resolver")
	}
	if d.DoerID == d.ResolverID {
		return s, IndentsInvalid("resolver_id", "Procurement doer and problem resolver must be different people")
	}
	d.Documents, e = IndentsText("documents", d.Documents)
	if e != nil {
		return s, e
	}
	if len(d.Lines) < 1 || len(d.Lines) > 200 {
		return s, IndentsInvalid("lines", "Add 1–200 requirements")
	}
	seen := map[string]bool{}
	for i, line := range d.Lines {
		line.DemandID, e = IndentsID(line.DemandID)
		if e != nil {
			return s, IndentsInvalid("demand_id", "Select a valid requirement")
		}
		if seen[line.DemandID] {
			return s, IndentsInvalid("demand_id", "Duplicate requirement in indent")
		}
		seen[line.DemandID] = true
		if _, e = IndentsQuantity(line.Quantity); e != nil {
			return s, e
		}
		if !IndentsMoney(line.Budget) {
			return s, IndentsInvalid("budget", "Budget must be nonnegative with up to 16 integer digits and 2 decimals")
		}
		line.FullKit, e = IndentsText("full_kit", line.FullKit)
		if e != nil {
			return s, e
		}
		d.Lines[i] = line
	}
	s.Data = d
	return s, nil
}
func (a IndentsAction) Validate() (IndentsAction, error) {
	var e error
	a.Reason, e = IndentsText("reason", a.Reason)
	if e != nil {
		return a, e
	}
	a.Evidence, e = IndentsText("evidence", a.Evidence)
	if e != nil {
		return a, e
	}
	if a.Version < 1 {
		return a, IndentsInvalid("version", "Use the saved indent version")
	}
	switch a.Action {
	case "submit", "return", "review", "release", "withdraw":
	default:
		return a, IndentsInvalid("action", "Unknown indent action")
	}
	if len(a.LineIDs) > 200 {
		return a, IndentsInvalid("line_ids", "Select at most 200 requirements")
	}
	seen := map[string]bool{}
	for i, id := range a.LineIDs {
		id, e = IndentsID(id)
		if e != nil {
			return a, e
		}
		if seen[id] {
			return a, IndentsInvalid("line_ids", "Select distinct lines for release")
		}
		seen[id] = true
		a.LineIDs[i] = id
	}
	return a, nil
}
func IndentsReleaseGuard(line IndentsLine, source IndentsSource, committed int64) error {
	if !source.Active {
		return IndentsInvalid("demand_id", "Project requirement is inactive")
	}
	if source.Policy.Status != "eligible" {
		return IndentsInvalid("demand_id", "Client make policy blocks release: "+strings.Join(source.Policy.Reasons, "; "))
	}
	if len(source.Holds.Holds) > 0 {
		return IndentsInvalid("demand_id", "Client purchase hold blocks release")
	}
	q, e := IndentsQuantity(line.Quantity)
	if e != nil {
		return e
	}
	total, e := IndentsQuantity(source.Quantity)
	if e != nil {
		return e
	}
	if committed > total || q > total-committed {
		return IndentsInvalid("quantity", "Quantity exceeds remaining project requirement")
	}
	return nil
}
func IndentsCopy(r IndentsRecord) IndentsRecord {
	r.Data.Lines = append([]IndentsLine{}, r.Data.Lines...)
	r.Data.Released = append([]IndentsRelease{}, r.Data.Released...)
	return r
}

func IndentsNumber(id string) string {
	return "PI-" + strings.ToUpper(strings.ReplaceAll(id, "-", "")[:12])
}
