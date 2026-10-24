package domain

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type Error struct {
	Status         int
	Field, Message string
}
type Timestamp = time.Time

func (e *Error) Error() string  { return e.Message }
func Invalid(f, m string) error { return &Error{422, f, m} }

var Unauthorized = &Error{401, "", "Invalid or expired credentials"}
var Forbidden = &Error{403, "", "Project responsibility access denied"}
var Missing = &Error{404, "", "Project or responsibility not found"}
var Stale = &Error{409, "", "Responsibility changed; reload before saving"}
var Duplicate = &Error{409, "", "An active profile for this user, role and responsibility already exists"}
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ID(s string) (string, error) {
	s = strings.ToLower(s)
	if !uuid.MatchString(s) {
		return "", Invalid("id", "Choose a valid record identifier")
	}
	return s, nil
}

type Actor struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}

func (a Actor) Can(p string) bool {
	return a.Active && a.ID != "" && (a.Superuser || slices.Contains(a.Permissions, p))
}

type User struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Active      bool     `json:"is_active"`
	Superuser   bool     `json:"-"`
	Permissions []string `json:"-"`
	BundleIDs   []string `json:"bundle_ids"`
}

func (u User) Can(p string) bool { return Actor{u.ID, u.Active, u.Superuser, u.Permissions}.Can(p) }

type Bundle struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Active      bool     `json:"is_active"`
	Permissions []string `json:"permissions"`
}
type Project struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	ManagerID              string `json:"project_manager_id"`
	ProcurementExecutiveID string `json:"procurement_executive_id"`
	Active                 bool   `json:"is_active"`
	Lifecycle              string `json:"lifecycle"`
}
type Profile struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	UserID         string    `json:"user_id"`
	UserName       string    `json:"user_name"`
	BundleID       string    `json:"bundle_id"`
	BundleName     string    `json:"bundle_name"`
	Responsibility string    `json:"responsibility"`
	Permissions    []string  `json:"permission_codes"`
	Active         bool      `json:"is_active"`
	Version        int32     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	CreatedBy      *string   `json:"created_by_id"`
	UpdatedBy      *string   `json:"updated_by_id"`
}
type Input struct {
	UserID         string   `json:"user_id"`
	BundleID       string   `json:"bundle_id"`
	Responsibility string   `json:"responsibility"`
	Permissions    []string `json:"permission_codes"`
	Active         bool     `json:"is_active"`
	Version        int32    `json:"version"`
	Reason         string   `json:"reason"`
}
type Event struct {
	ID        string    `json:"id"`
	ProfileID string    `json:"profile_id"`
	ProjectID string    `json:"project_id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Before    *Profile  `json:"before"`
	After     Profile   `json:"after"`
	ActorID   string    `json:"created_by_id"`
	At        time.Time `json:"created_at"`
}
type Owner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Access struct {
	Project  Project
	CanWrite bool
}
type Options struct {
	Users   []User   `json:"users"`
	Bundles []Bundle `json:"bundles"`
}
type Workspace struct {
	Project  Project   `json:"project"`
	Profiles []Profile `json:"profiles"`
	CanWrite bool      `json:"can_write"`
}

// Financial decision permissions require explicit scoped authority, even for a PM.
func FinancialDecision(p string) bool {
	return p == "accounts.purchase_acceptance.write" || p == "accounts.purchase_advances.write" || p == "accounts.purchase_advances.disburse" || p == "projects.budgets.approve" || p == "accounts.quotation_approvals.write" || p == "projects.group_awards.approve" || p == "projects.purchase_sourcing.approve"
}
func ScopedCode(p string) bool {
	if strings.HasPrefix(p, "accounts.purchase_acceptance.") || strings.HasPrefix(p, "accounts.quotation_approvals.") || strings.HasPrefix(p, "accounts.purchase_advances.") {
		return true
	}
	if !strings.HasPrefix(p, "projects.") {
		return false
	}
	for _, prefix := range []string{"projects.portfolio.", "projects.purchase_portfolio.", "projects.master.", "projects.manager_assignments.", "projects.purchase_assignments.", "projects.processes.bypass"} {
		if strings.HasPrefix(p, prefix) {
			return false
		}
	}
	return p != "projects.team.write"
}
func Manage(a Actor, p Project) bool {
	return a.Can("projects.team.write") && (a.Superuser || a.Can("projects.portfolio.write") || a.ID == p.ManagerID)
}
func Portfolio(a Actor, write bool) bool {
	if a.Superuser {
		return true
	}
	if write {
		return a.Can("projects.portfolio.write")
	}
	return a.Can("projects.portfolio.read") || a.Can("projects.portfolio.write")
}
func Effective(a Actor, u User, b Bundle, member bool, p Profile, code string) bool {
	return a.ID == u.ID && a.Can(code) && u.Can(code) && u.Active && b.Active && member && p.Active && p.UserID == u.ID && p.BundleID == b.ID && ScopedCode(code) && slices.Contains(p.Permissions, code) && slices.Contains(b.Permissions, code)
}
func Open(p Project) bool { return p.Active && p.Lifecycle != "closed" && p.Lifecycle != "on_hold" }
func (i Input) Validate() (Input, error) {
	var e error
	if i.UserID, e = ID(i.UserID); e != nil {
		return i, e
	}
	if i.BundleID, e = ID(i.BundleID); e != nil {
		return i, e
	}
	i.Responsibility = strings.TrimSpace(i.Responsibility)
	i.Reason = strings.TrimSpace(i.Reason)
	name := strings.ToLower(i.Responsibility)
	if i.Responsibility == "" || utf8.RuneCountInString(i.Responsibility) > 80 || name == "project manager" || name == "pm" || name == "procurement executive" || name == "purchase manager" {
		return i, Invalid("responsibility", "Choose a specialist responsibility; manage PM and Procurement Executive in their owning workspaces")
	}
	if i.Reason == "" || utf8.RuneCountInString(i.Reason) > 2000 {
		return i, Invalid("reason", "Record a reason of at most 2000 characters")
	}
	if i.Version < 0 || len(i.Permissions) == 0 || len(i.Permissions) > 100 {
		return i, Invalid("permission_codes", "Provide a bounded permission profile and current version")
	}
	i.Permissions = append([]string{}, i.Permissions...)
	slices.Sort(i.Permissions)
	for n, code := range i.Permissions {
		if !ScopedCode(code) || (n > 0 && i.Permissions[n-1] == code) {
			return i, Invalid("permission_codes", "Use distinct project-scoped permission codes")
		}
	}
	return i, nil
}
func ValidateEligibility(i Input, u *User, b *Bundle, member bool) error {
	if u == nil || !u.Active {
		return Invalid("user_id", "Choose an active account")
	}
	if b == nil || !b.Active || !member {
		return Invalid("bundle_id", "Choose an active role currently assigned to this account")
	}
	for _, code := range i.Permissions {
		if !u.Can(code) || !slices.Contains(b.Permissions, code) {
			return Invalid("permission_codes", "Profile permissions must remain within this account's assigned role")
		}
	}
	return nil
}
func Copy(p Profile) Profile { p.Permissions = append([]string{}, p.Permissions...); return p }
