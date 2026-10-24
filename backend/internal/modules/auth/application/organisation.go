package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type OrganisationService struct {
	unit   ports.UnitOfWork[ports.OrganisationRepository]
	values ports.Tokens
}

func NewOrganisation(u ports.UnitOfWork[ports.OrganisationRepository], v ports.Tokens) *OrganisationService {
	return &OrganisationService{u, v}
}
func (s *OrganisationService) Departments(ctx context.Context, token string) ([]domain.Department, error) {
	result := []domain.Department{}
	e := authorized(ctx, s.unit, s.values, token, "admin.organisation.read", false, func(ctx context.Context, r ports.OrganisationRepository, a domain.User) error {
		var e error
		result, e = r.Departments(ctx)
		return e
	})
	return result, e
}
func (s *OrganisationService) Employees(ctx context.Context, token string) ([]domain.Employee, error) {
	result := []domain.Employee{}
	e := authorized(ctx, s.unit, s.values, token, "admin.organisation.read", false, func(ctx context.Context, r ports.OrganisationRepository, a domain.User) error {
		var e error
		result, e = r.Employees(ctx)
		return e
	})
	return result, e
}

// Team returns only live descendants. Organisation permission does not bypass
// reporting scope; administrators use the separate organisation directory.
func (s *OrganisationService) Team(ctx context.Context, token string) ([]domain.Employee, error) {
	result := []domain.Employee{}
	e := authorized(ctx, s.unit, s.values, token, "account.team.read", false, func(ctx context.Context, r ports.OrganisationRepository, a domain.User) error {
		all, e := r.Employees(ctx)
		if e != nil {
			return e
		}
		for _, v := range all {
			if domain.ReportsTo(all, v.ID, a.ID) {
				result = append(result, v)
			}
		}
		return nil
	})
	return result, e
}
func (s *OrganisationService) SaveDepartment(ctx context.Context, token, id string, in domain.DepartmentInput) (domain.Department, error) {
	var result domain.Department
	if e := in.Validate(); e != nil {
		return result, e
	}
	e := authorized(ctx, s.unit, s.values, token, "admin.organisation.write", true, func(ctx context.Context, r ports.OrganisationRepository, a domain.User) error {
		departments, e := r.Departments(ctx)
		if e != nil {
			return e
		}
		found := false
		for _, d := range departments {
			if d.ID == id {
				found = true
				if d.Version != in.Version {
					return domain.ErrOrganisationConflict
				}
			} else if d.Code == in.Code {
				return &domain.Error{Kind: domain.Conflict, Detail: "Department code is already in use."}
			}
		}
		if in.Version > 0 && !found {
			return domain.ErrOrganisationConflict
		}
		if in.Version == 0 {
			if id != "" {
				return domain.ErrOrganisationConflict
			}
			id = s.values.ID()
		}
		if in.ManagerID != "" {
			manager, e := r.User(ctx, in.ManagerID, false)
			if e != nil {
				return e
			}
			if manager == nil || !manager.IsActive {
				return &domain.Error{Kind: domain.Invalid, Detail: "Department manager must be an active employee."}
			}
		}
		if !in.IsActive {
			all, e := r.Employees(ctx)
			if e != nil {
				return e
			}
			for _, v := range all {
				if v.IsActive && v.DepartmentID == id {
					return &domain.Error{Kind: domain.Conflict, Detail: "Move active employees before deactivating this department."}
				}
			}
			if in.ManagerID != "" {
				return &domain.Error{Kind: domain.Invalid, Detail: "Remove the department manager before deactivation."}
			}
		}
		result, e = r.SaveDepartment(ctx, id, in, a.ID)
		if e != nil {
			return e
		}
		return r.RecordOrganisationChange(ctx, domain.OrganisationChange{ID: s.values.ID(), EntityID: id, EntityType: "department", ActorID: a.ID, Reason: in.Reason, Version: result.Version, At: domain.At(s.values.Now())})
	})
	return result, e
}
func (s *OrganisationService) SaveEmployee(ctx context.Context, token, id string, in domain.EmployeeInput) (domain.Employee, error) {
	var result domain.Employee
	if e := in.Validate(id); e != nil {
		return result, e
	}
	e := authorized(ctx, s.unit, s.values, token, "admin.organisation.write", true, func(ctx context.Context, r ports.OrganisationRepository, a domain.User) error {
		user, e := r.User(ctx, id, true)
		if e != nil {
			return e
		}
		if user == nil {
			return domain.ErrUserMissing
		}
		if !user.IsActive {
			return &domain.Error{Kind: domain.Invalid, Detail: "Inactive employee profiles cannot be reassigned."}
		}
		all, e := r.Employees(ctx)
		if e != nil {
			return e
		}
		for _, v := range all {
			if v.ID == id && v.Version != in.Version {
				return domain.ErrOrganisationConflict
			}
		}
		departments, e := r.Departments(ctx)
		if e != nil {
			return e
		}
		valid := false
		for _, d := range departments {
			if d.ID == in.DepartmentID && d.IsActive {
				valid = true
			}
		}
		if !valid {
			return &domain.Error{Kind: domain.Invalid, Detail: "Select an active department."}
		}
		if in.ManagerID != "" {
			valid = false
			for _, v := range all {
				if v.ID == in.ManagerID && v.IsActive && v.Version > 0 {
					valid = true
				}
			}
			if !valid {
				return &domain.Error{Kind: domain.Invalid, Detail: "Set up the active reporting manager's organisation profile first."}
			}
			if domain.ReportsTo(all, in.ManagerID, id) {
				return &domain.Error{Kind: domain.Invalid, Detail: "This reporting line would create a circular hierarchy."}
			}
		}
		result, e = r.SaveEmployee(ctx, id, in, a.ID)
		if e != nil {
			return e
		}
		return r.RecordOrganisationChange(ctx, domain.OrganisationChange{ID: s.values.ID(), EntityID: id, EntityType: "employee", ActorID: a.ID, Reason: in.Reason, Version: result.Version, At: domain.At(s.values.Now())})
	})
	return result, e
}

func (s *OrganisationService) TeamWork(ctx context.Context, token, userID string) ([]domain.Responsibility, error) {
	out := []domain.Responsibility{}
	e := authorized(ctx, s.unit, s.values, token, "account.team.read", false, func(ctx context.Context, r ports.OrganisationRepository, a domain.User) error {
		all, e := r.Employees(ctx)
		if e != nil {
			return e
		}
		if !domain.ReportsTo(all, userID, a.ID) {
			return domain.ErrForbidden
		}
		out, e = r.WorkInventory(ctx, userID)
		return e
	})
	return out, e
}
