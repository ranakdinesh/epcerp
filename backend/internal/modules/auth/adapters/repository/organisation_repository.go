package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type organisationRepository struct{ *Store }

func (r *organisationRepository) Departments(ctx context.Context) ([]domain.Department, error) {
	rows, e := r.queries.OrganisationDepartments(ctx)
	result := []domain.Department{}
	for _, v := range rows {
		result = append(result, dtos.Department(v))
	}
	return result, e
}
func (r *organisationRepository) Employees(ctx context.Context) ([]domain.Employee, error) {
	rows, e := r.queries.OrganisationEmployees(ctx)
	result := []domain.Employee{}
	for _, v := range rows {
		result = append(result, dtos.Employee(v))
	}
	return result, e
}
func (r *organisationRepository) SaveDepartment(ctx context.Context, id string, in domain.DepartmentInput, actor string) (domain.Department, error) {
	row, e := r.queries.SaveOrganisationDepartment(ctx, sqlc.SaveOrganisationDepartmentParams{ID: dtos.UUID(id), Code: in.Code, Name: in.Name, ManagerID: dtos.OptionalUUID(in.ManagerID), IsActive: in.IsActive, ActorID: dtos.UUID(actor), ExpectedVersion: in.Version})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Department{}, domain.ErrOrganisationConflict
	}
	if unique(e) {
		return domain.Department{}, &domain.Error{Kind: domain.Conflict, Detail: "Department code is already in use."}
	}
	return dtos.Department(row), e
}
func (r *organisationRepository) SaveEmployee(ctx context.Context, id string, in domain.EmployeeInput, actor string) (domain.Employee, error) {
	_, e := r.queries.SaveOrganisationEmployee(ctx, sqlc.SaveOrganisationEmployeeParams{UserID: dtos.UUID(id), DepartmentID: dtos.UUID(in.DepartmentID), ManagerID: dtos.OptionalUUID(in.ManagerID), IsCompanyRoot: in.IsCompanyRoot, ActorID: dtos.UUID(actor), ExpectedVersion: in.Version})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Employee{}, domain.ErrOrganisationConflict
	}
	if e != nil {
		return domain.Employee{}, e
	}
	if e = r.queries.SaveEmployeeContact(ctx, sqlc.SaveEmployeeContactParams{ID: dtos.UUID(id), FirstName: in.FirstName, LastName: in.LastName, Mobile: in.Mobile, UpdatedByID: dtos.UUID(actor)}); e != nil {
		return domain.Employee{}, e
	}
	all, e := r.Employees(ctx)
	if e != nil {
		return domain.Employee{}, e
	}
	for _, v := range all {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.Employee{}, domain.ErrUserMissing
}
func (r *organisationRepository) RecordOrganisationChange(ctx context.Context, c domain.OrganisationChange) error {
	var e error
	if c.EntityType == "department" {
		e = r.queries.RecordOrganisationDepartmentChange(ctx, sqlc.RecordOrganisationDepartmentChangeParams{EventID: dtos.UUID(c.ID), EntityID: dtos.UUID(c.EntityID), ActorID: dtos.UUID(c.ActorID), Reason: c.Reason})
	} else {
		e = r.queries.RecordOrganisationEmployeeChange(ctx, sqlc.RecordOrganisationEmployeeChangeParams{EventID: dtos.UUID(c.ID), EntityID: dtos.UUID(c.EntityID), ActorID: dtos.UUID(c.ActorID), Reason: c.Reason})
	}
	if e != nil {
		return e
	}
	if r.events == nil {
		return errors.New("organisation event writer required")
	}
	return r.events.Record(ctx, events.Event{ID: c.ID, CorrelationID: c.ID, Module: "auth", EntityType: c.EntityType, EntityID: c.EntityID, EntityVersion: c.Version, Action: "organisation_updated", ActorID: c.ActorID, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: c.At}, nil)
}
