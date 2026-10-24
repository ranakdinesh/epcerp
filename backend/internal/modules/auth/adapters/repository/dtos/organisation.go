package dtos

import (
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func OptionalUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	return UUID(s)
}
func Department(d sqlc.AuthAuthDepartment) domain.Department {
	return domain.Department{ID: d.ID.String(), Code: d.Code, Name: d.Name, ManagerID: d.ManagerID.String(), IsActive: d.IsActive, Version: d.Version, CreatedAt: d.CreatedAt.Time, UpdatedAt: d.UpdatedAt.Time, CreatedByID: d.CreatedByID.String(), UpdatedByID: d.UpdatedByID.String()}
}
func Employee(e sqlc.OrganisationEmployeesRow) domain.Employee {
	return domain.Employee{ID: e.ID.String(), Username: e.Username, Email: e.Email, FirstName: e.FirstName, LastName: e.LastName, Mobile: e.Mobile, DepartmentID: e.DepartmentID.String(), ManagerID: e.ManagerID.String(), IsActive: e.IsActive, IsCompanyRoot: e.IsCompanyRoot, Version: e.Version}
}
