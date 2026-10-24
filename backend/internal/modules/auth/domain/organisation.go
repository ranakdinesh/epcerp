package domain

import (
	"strings"
	"time"
	"unicode"
)

type Department struct {
	ID, Code, Name, ManagerID string
	IsActive                  bool
	Version                   int64
	CreatedAt, UpdatedAt      time.Time
	CreatedByID, UpdatedByID  string
}
type DepartmentInput struct {
	Code, Name, ManagerID, Reason string
	IsActive                      bool
	Version                       int64
}
type Employee struct {
	ID, Username, Email, FirstName, LastName, Mobile string
	DepartmentID, ManagerID                          string
	IsActive, IsCompanyRoot                          bool
	Version                                          int64
}
type EmployeeInput struct {
	FirstName, LastName, Mobile, DepartmentID, ManagerID, Reason string
	IsCompanyRoot                                                bool
	Version                                                      int64
}
type OrganisationChange struct {
	ID, EntityID, EntityType, ActorID, Reason string
	Version                                   int64
	At                                        time.Time
}

var ErrOrganisationConflict = &Error{Conflict, "Organisation record changed. Refresh and review before saving."}

func ValidEmployeeContact(first, last, mobile string) bool {
	if len([]rune(strings.TrimSpace(first))) < 1 || len([]rune(first)) > 100 || len([]rune(strings.TrimSpace(last))) < 1 || len([]rune(last)) > 100 {
		return false
	}
	digits := 0
	for i, c := range mobile {
		if c >= '0' && c <= '9' {
			digits++
			continue
		}
		if c == '+' && i == 0 {
			continue
		}
		if c != ' ' && c != '-' && c != '(' && c != ')' {
			return false
		}
	}
	if digits < 8 || digits > 15 || len(mobile) > 24 {
		return false
	}
	for _, c := range first + last {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
func (d DepartmentInput) Validate() error {
	if len(d.Code) < 2 || len(d.Code) > 32 || len([]rune(strings.TrimSpace(d.Name))) < 2 || len([]rune(d.Name)) > 150 || !ValidResetReason(d.Reason) || d.Version < 0 {
		return &Error{Invalid, "Provide department code, name, version and a reason of 10 to 1000 characters."}
	}
	for _, c := range d.Code {
		if !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '-' && c != '_' {
			return &Error{Invalid, "Department code must use uppercase letters, numbers, hyphens or underscores."}
		}
	}
	return nil
}
func (e EmployeeInput) Validate(id string) error {
	if !ValidEmployeeContact(e.FirstName, e.LastName, e.Mobile) || !ValidResetReason(e.Reason) || e.DepartmentID == "" || e.Version < 0 {
		return &Error{Invalid, "First name, last name, valid mobile, department, version and change reason are required."}
	}
	if e.ManagerID == id || (e.IsCompanyRoot && e.ManagerID != "") || (!e.IsCompanyRoot && e.ManagerID == "") {
		return &Error{Invalid, "Select one reporting manager, or explicitly mark the employee as a company root."}
	}
	return nil
}

// ReportsTo walks live edges with a visited set, including defence against a
// malformed imported graph. An employee never counts as their own report.
func ReportsTo(employees []Employee, employeeID, managerID string) bool {
	if employeeID == managerID {
		return false
	}
	byID := map[string]Employee{}
	for _, e := range employees {
		byID[e.ID] = e
	}
	seen := map[string]bool{}
	for employeeID != "" && !seen[employeeID] {
		seen[employeeID] = true
		e, ok := byID[employeeID]
		if !ok || !e.IsActive {
			return false
		}
		if e.ManagerID == managerID {
			return true
		}
		employeeID = e.ManagerID
	}
	return false
}
