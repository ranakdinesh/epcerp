package repository

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func (r *organisationRepository) ownWork(ctx context.Context, owner, kind, id string) ([]domain.Responsibility, error) {
	out := []domain.Responsibility{}
	all, e := r.Employees(ctx)
	if e != nil {
		return nil, e
	}
	for _, v := range all {
		if v.IsActive && v.ManagerID != "" && (owner == "" || v.ManagerID == owner) && (kind == "" || kind == "reporting_manager") && (id == "" || v.ID == id) {
			out = append(out, domain.Responsibility{Source: "auth", Kind: "reporting_manager", ID: v.ID, OwnerID: v.ManagerID, Title: v.Username + " · reporting manager", State: "active", Link: "/dashboard?screen=organisation", Version: v.Version, RequiredPermissions: []string{}})
		}
	}
	deps, e := r.Departments(ctx)
	if e != nil {
		return nil, e
	}
	for _, v := range deps {
		if v.IsActive && v.ManagerID != "" && (owner == "" || v.ManagerID == owner) && (kind == "" || kind == "department_manager") && (id == "" || v.ID == id) {
			out = append(out, domain.Responsibility{Source: "auth", Kind: "department_manager", ID: v.ID, OwnerID: v.ManagerID, Title: v.Name + " · department manager", State: "active", Link: "/dashboard?screen=organisation", Version: v.Version, RequiredPermissions: []string{}})
		}
	}
	return out, nil
}
func (r *organisationRepository) WorkInventory(ctx context.Context, user string) ([]domain.Responsibility, error) {
	out, e := r.ownWork(ctx, user, "", "")
	if e != nil {
		return nil, e
	}
	for _, source := range []string{"projects", "purchase_fms", "master", "project_access", "project_budgets", "organisation_governance", "tender_fms"} {
		reader := r.work[source]
		if reader == nil {
			return nil, &domain.Error{Kind: domain.Unavailable, Detail: "Responsibility inventory is not fully configured."}
		}
		rows, e := reader.Inventory(ctx, user)
		if e != nil {
			return nil, e
		}
		out = append(out, rows...)
	}
	return out, nil
}
func (r *organisationRepository) WorkCurrent(ctx context.Context, ref domain.Responsibility) ([]domain.Responsibility, error) {
	if ref.Source == "auth" {
		return r.ownWork(ctx, "", ref.Kind, ref.ID)
	}
	reader := r.work[ref.Source]
	if reader == nil {
		return nil, &domain.Error{Kind: domain.Unavailable, Detail: "Responsibility source is unavailable."}
	}
	return reader.Current(ctx, ref.Kind, ref.ID)
}
