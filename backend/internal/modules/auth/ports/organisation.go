package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type OrganisationRepository interface {
	AdministrationRepository
	WorkInventory(context.Context, string) ([]domain.Responsibility, error)
	WorkCurrent(context.Context, domain.Responsibility) ([]domain.Responsibility, error)
	Departments(context.Context) ([]domain.Department, error)
	Employees(context.Context) ([]domain.Employee, error)
	SaveDepartment(context.Context, string, domain.DepartmentInput, string) (domain.Department, error)
	SaveEmployee(context.Context, string, domain.EmployeeInput, string) (domain.Employee, error)
	RecordOrganisationChange(context.Context, domain.OrganisationChange) error
}
