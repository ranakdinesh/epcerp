package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrUnauthorized = errors.New("invalid or expired credentials")
var ErrForbidden = errors.New("project access denied")
var ErrMissing = errors.New("project not found")
var ErrConflict = errors.New("duplicate project or assignment conflict")
var ErrStale = errors.New("project changed; refresh before saving")

type Invalid struct{ Field, Message string }

func (e *Invalid) Error() string { return e.Message }

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ID(value string) (string, error) {
	v := strings.ToLower(value)
	if !uuidPattern.MatchString(v) {
		return "", &Invalid{"id", "Select a valid record"}
	}
	return v, nil
}
func Text(field, value string, max int) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" || utf8.RuneCountInString(v) > max {
		return "", &Invalid{field, "Enter a nonempty value within the permitted length"}
	}
	return v, nil
}

type Principal struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}

func (p Principal) Allows(code string) bool {
	if !p.Active {
		return false
	}
	if p.Superuser {
		return true
	}
	for _, v := range p.Permissions {
		if v == code {
			return true
		}
	}
	return false
}
func (p Principal) Portfolio() bool {
	return p.Allows("projects.portfolio.read") || p.Allows("projects.portfolio.write") || p.Allows("projects.manager_assignments.write")
}

type Candidate struct {
	ID, Name          string
	Active, Superuser bool
	Permissions       []string
}

func (c Candidate) Eligible() bool {
	return (Principal{ID: c.ID, Active: c.Active, Superuser: c.Superuser, Permissions: c.Permissions}).Allows("projects.master.read") && (Principal{ID: c.ID, Active: c.Active, Superuser: c.Superuser, Permissions: c.Permissions}).Allows("projects.master.write")
}

type Client struct {
	ID, Name, Code string
	Active         bool
}
type Project struct {
	ProcurementExecutiveID string
	Capture

	ID, Number, Name, NameKey, ClientID, ManagerID string
	Version                                        int32
	Active                                         bool
	Lifecycle                                      string
	CreatedAt, UpdatedAt                           time.Time
	CreatedBy, UpdatedBy                           string
}
type Assignment struct {
	ID, ProjectID, BeforeManagerID, ManagerID, Reason, ActorID string
	Version                                                    int32
	OccurredAt                                                 time.Time
}
type Create struct {
	Name, ClientID, ManagerID, ProcurementExecutiveID string
	Capture
}

func (c Create) Validate() (Create, error) {
	v, err := Text("name", c.Name, 255)
	if err != nil {
		return c, err
	}
	c.Name = v
	if utf8.RuneCountInString(v) < 3 {
		return c, &Invalid{"name", "Enter at least three characters"}
	}
	if c.Capture, err = c.Capture.Validate(c.Name); err != nil {
		return c, err
	}
	if c.ClientID, err = ID(c.ClientID); err != nil {
		return c, &Invalid{"client_id", "Select an active client"}
	}
	if c.ManagerID, err = ID(c.ManagerID); err != nil {
		return c, &Invalid{"project_manager_id", "Select an eligible active manager"}
	}
	if c.ProcurementExecutiveID, err = ID(c.ProcurementExecutiveID); err != nil {
		return c, &Invalid{"procurement_executive_id", "Select an eligible Procurement Executive"}
	}
	return c, nil
}

type Transfer struct {
	ManagerID, Reason string
	ExpectedVersion   int32
}

func (t Transfer) Validate() (Transfer, error) {
	var err error
	if t.ManagerID, err = ID(t.ManagerID); err != nil {
		return t, &Invalid{"project_manager_id", "Select an eligible active manager"}
	}
	if t.Reason, err = Text("reason", t.Reason, 2000); err != nil {
		return t, err
	}
	if t.ExpectedVersion < 1 {
		return t, &Invalid{"expected_version", "A current project version is required"}
	}
	return t, nil
}

type Filter struct {
	Page, PageSize  int32
	Search          string
	Sort, Direction string
}

func (f Filter) Validate() (Filter, error) {
	if f.Page < 1 || f.Page > 100000 || f.PageSize < 1 || f.PageSize > 100 || utf8.RuneCountInString(f.Search) > 255 {
		return f, &Invalid{"query", "Invalid pagination or search"}
	}
	f.Search = strings.ToLower(strings.TrimSpace(f.Search))
	return f, nil
}

type Page struct {
	Items          []Project
	Total          int64
	Page, PageSize int32
}
type Options struct {
	Clients     []Client
	Managers    []Candidate
	Procurement []Candidate
}

func CanRead(p Principal, project Project) bool {
	return p.Allows("projects.master.read") && (p.Portfolio() || p.ID == project.ManagerID)
}

// NameKey defines native Go duplicate-name semantics for project registration.
func NameKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
