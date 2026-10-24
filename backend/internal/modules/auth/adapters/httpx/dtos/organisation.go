package dtos

import (
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"io"
	"regexp"
	"strings"
	"time"
)

var organisationID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type DepartmentResponse struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	ManagerID string    `json:"manager_id"`
	IsActive  bool      `json:"is_active"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func Department(d domain.Department) DepartmentResponse {
	return DepartmentResponse{d.ID, d.Code, d.Name, d.ManagerID, d.IsActive, d.Version, d.CreatedAt, d.UpdatedAt}
}

type EmployeeResponse struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Mobile        string `json:"mobile"`
	DepartmentID  string `json:"department_id"`
	ManagerID     string `json:"manager_id"`
	IsActive      bool   `json:"is_active"`
	IsCompanyRoot bool   `json:"is_company_root"`
	Version       int64  `json:"version"`
}

func Employee(e domain.Employee) EmployeeResponse {
	return EmployeeResponse{e.ID, e.Username, e.Email, e.FirstName, e.LastName, e.Mobile, e.DepartmentID, e.ManagerID, e.IsActive, e.IsCompanyRoot, e.Version}
}
func ParseDepartment(reader io.Reader) (domain.DepartmentInput, error) {
	var p struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		ManagerID string `json:"manager_id"`
		Active    *bool  `json:"is_active"`
		Version   *int64 `json:"version"`
		Reason    string `json:"reason"`
	}
	_, e := Decode(reader, &p, "code", "name", "manager_id", "is_active", "version", "reason")
	if e != nil {
		return domain.DepartmentInput{}, e
	}
	if p.Active == nil || p.Version == nil {
		return domain.DepartmentInput{}, Invalid("version", "Version and active status are required", "missing")
	}
	if p.ManagerID != "" && !organisationID.MatchString(p.ManagerID) {
		return domain.DepartmentInput{}, Invalid("manager_id", "Invalid employee identifier", "value_error")
	}
	in := domain.DepartmentInput{Code: strings.ToUpper(strings.TrimSpace(p.Code)), Name: strings.TrimSpace(p.Name), ManagerID: strings.ToLower(p.ManagerID), IsActive: *p.Active, Version: *p.Version, Reason: strings.TrimSpace(p.Reason)}
	return in, in.Validate()
}
func ParseEmployee(reader io.Reader) (domain.EmployeeInput, error) {
	var p struct {
		First        string `json:"first_name"`
		Last         string `json:"last_name"`
		Mobile       string `json:"mobile"`
		DepartmentID string `json:"department_id"`
		ManagerID    string `json:"manager_id"`
		Root         *bool  `json:"is_company_root"`
		Version      *int64 `json:"version"`
		Reason       string `json:"reason"`
	}
	object, e := Decode(reader, &p, "first_name", "last_name", "mobile", "department_id", "manager_id", "is_company_root", "version", "reason")
	if e != nil {
		return domain.EmployeeInput{}, e
	}
	if p.Root == nil || p.Version == nil {
		return domain.EmployeeInput{}, Invalid("version", "Version and root status are required", "missing")
	}
	if !organisationID.MatchString(p.DepartmentID) || (p.ManagerID != "" && !organisationID.MatchString(p.ManagerID)) {
		return domain.EmployeeInput{}, Invalid("department_id", "Valid department and manager identifiers are required", "value_error")
	}
	first, e := Text(object, "first_name", &p.First, 1, 100, true, true)
	if e != nil {
		return domain.EmployeeInput{}, e
	}
	last, e := Text(object, "last_name", &p.Last, 1, 100, true, true)
	if e != nil {
		return domain.EmployeeInput{}, e
	}
	mobile, e := Text(object, "mobile", &p.Mobile, 8, 24, true, true)
	if e != nil {
		return domain.EmployeeInput{}, e
	}
	if !domain.ValidEmployeeContact(first, last, mobile) {
		return domain.EmployeeInput{}, Invalid("mobile", "Use 8 to 15 digits, optionally with a country code and spaces", "value_error")
	}
	if !domain.ValidResetReason(strings.TrimSpace(p.Reason)) {
		return domain.EmployeeInput{}, Invalid("reason", "Use 10 to 1000 characters", "value_error")
	}
	in := domain.EmployeeInput{FirstName: first, LastName: last, Mobile: mobile, DepartmentID: strings.ToLower(p.DepartmentID), ManagerID: strings.ToLower(p.ManagerID), IsCompanyRoot: *p.Root, Version: *p.Version, Reason: strings.TrimSpace(p.Reason)}
	return in, nil
}

type WorkResponse struct {
	Source              string   `json:"source"`
	Kind                string   `json:"kind"`
	ID                  string   `json:"id"`
	OwnerID             string   `json:"owner_id"`
	Title               string   `json:"title"`
	State               string   `json:"state"`
	Link                string   `json:"link"`
	DueDate             string   `json:"due_date"`
	Version             int64    `json:"version"`
	RequiredPermissions []string `json:"required_permissions"`
}

func Work(v domain.Responsibility) WorkResponse {
	return WorkResponse{v.Source, v.Kind, v.ID, v.OwnerID, v.Title, v.State, v.Link, v.DueDate, v.Version, v.RequiredPermissions}
}
