package repository

import (
	"context"
	"time"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type userRepository struct{ *Store }

func (r *userRepository) Users(ctx context.Context, filter domain.UserFilter) (domain.UserPage, error) {
	page := domain.UserPage{Items: []domain.User{}}
	count, err := r.queries.CountUsers(ctx, sqlc.CountUsersParams{Active: dtos.Boolean(filter.Active), Search: filter.Search, RoleName: filter.Role})
	if err != nil {
		return page, err
	}
	page.Total = count
	rows, err := r.queries.FilterUsers(ctx, sqlc.FilterUsersParams{Active: dtos.Boolean(filter.Active), Search: filter.Search, RoleName: filter.Role, Descending: filter.Descending, PageSize: int32(filter.PageSize), PageOffset: int64(filter.Page-1) * int64(filter.PageSize)})
	if err != nil {
		return page, err
	}
	for _, row := range rows {
		user, err := r.user(ctx, row)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, *user)
	}
	return page, nil
}

func (r *userRepository) CreateUser(ctx context.Context, userID string, request domain.UserCreate, hash, actor string) (*domain.User, error) {
	row, err := r.queries.InsertUser(ctx, sqlc.InsertUserParams{ID: dtos.UUID(userID), Username: request.Username, Email: request.Email, PasswordHash: hash, CreatedByID: dtos.UUID(actor), FirstName: request.FirstName, LastName: request.LastName, Mobile: request.Mobile, IsSuperuser: request.IsSuperuser})
	if unique(err) {
		return nil, domain.ErrUserConflict
	}
	if err != nil {
		return nil, err
	}
	if err = r.assignNames(ctx, userID, request.RoleNames); err != nil {
		return nil, err
	}
	return r.user(ctx, row)
}

func (r *userRepository) assignNames(ctx context.Context, userID string, names []string) error {
	for _, name := range names {
		role, err := r.queries.FindRole(ctx, name)
		if err != nil {
			return err
		}
		if err = r.AssignRole(ctx, userID, role.ID.String()); err != nil {
			return err
		}
	}
	return nil
}

func (r *userRepository) UpdateUser(ctx context.Context, userID string, request domain.UserUpdate, actor string) (*domain.User, error) {
	err := r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{ID: dtos.UUID(userID), Email: dtos.Nullable(request.Email), FirstName: dtos.Nullable(request.FirstName), LastName: dtos.Nullable(request.LastName), Mobile: dtos.Nullable(request.Mobile), ActorID: dtos.UUID(actor)})
	if unique(err) {
		return nil, domain.ErrEmailConflict
	}
	if err != nil {
		return nil, err
	}
	if request.RoleNames != nil {
		if err = r.queries.ClearUserRoles(ctx, dtos.UUID(userID)); err != nil {
			return nil, err
		}
		if err = r.assignNames(ctx, userID, *request.RoleNames); err != nil {
			return nil, err
		}
	}
	return r.User(ctx, userID, false)
}

func (r *userRepository) ActiveSuperusers(ctx context.Context) (int64, error) {
	return r.queries.CountActiveSuperusers(ctx)
}

func (r *userRepository) Superusers(ctx context.Context) (int64, error) {
	return r.queries.CountSuperusers(ctx)
}

func (r *userRepository) Online(ctx context.Context, since int64) ([]domain.Activity, error) {
	rows, err := r.queries.OnlineActivity(ctx, dtos.Stamp(time.Unix(since, 0)))
	if err != nil {
		return nil, err
	}
	result := []domain.Activity{}
	for _, row := range rows {
		result = append(result, dtos.Activity(row))
	}
	return result, nil
}

func (r *userRepository) Roles(ctx context.Context) ([]domain.Role, error) {
	return (&roleRepository{r.Store}).Roles(ctx)
}

func (r *userRepository) SetActive(ctx context.Context, id string, active bool, actor string) error {
	return r.queries.SetUserActive(ctx, sqlc.SetUserActiveParams{UserID: dtos.UUID(id), IsActive: active, ActorID: dtos.UUID(actor)})
}
func (r *userRepository) RecordStatusChange(ctx context.Context, event, id, actor, reason string) error {
	return r.queries.RecordUserStatusChange(ctx, sqlc.RecordUserStatusChangeParams{EventID: dtos.UUID(event), UserID: dtos.UUID(id), ActorID: dtos.UUID(actor), Reason: reason})
}

// Only Auth-owned data is touched here. Inventory is loaded later, so a missing
// manager or unavailable source module cannot hold up access revocation.
func (r *userRepository) QueueHandover(ctx context.Context, id, caseID, event, actor, reason string) error {
	rows, err := r.queries.QueueUserHandover(ctx, sqlc.QueueUserHandoverParams{UserID: dtos.UUID(id), CaseID: dtos.UUID(caseID), ActorID: dtos.UUID(actor), Reason: reason})
	if err != nil || rows == 0 {
		return err
	}
	return r.queries.RecordOffboardingChange(ctx, sqlc.RecordOffboardingChangeParams{EventID: dtos.UUID(event), CaseID: dtos.UUID(caseID), ActorID: dtos.UUID(actor), Action: "access_revoked", Reason: reason})
}
