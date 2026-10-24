package application

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type UserService struct {
	unit      ports.UnitOfWork[ports.UserRepository]
	tokens    ports.Tokens
	passwords ports.Passwords
	policy    domain.Policy
}

func NewUsers(unit ports.UnitOfWork[ports.UserRepository], passwords ports.Passwords, tokens ports.Tokens, policy domain.Policy) *UserService {
	return &UserService{unit: unit, tokens: tokens, passwords: passwords, policy: policy}
}

func (s *UserService) Users(ctx context.Context, token string, filter domain.UserFilter) (domain.UserPage, error) {
	var result domain.UserPage
	err := authorized(ctx, s.unit, s.tokens, token, "admin.users.read", false, func(ctx context.Context, repo ports.UserRepository, actor domain.User) error {
		var err error
		result, err = repo.Users(ctx, filter)
		return err
	})
	return result, err
}

func (s *UserService) User(ctx context.Context, token, userID string) (*domain.User, error) {
	var result *domain.User
	err := authorized(ctx, s.unit, s.tokens, token, "admin.users.read", false, func(ctx context.Context, repo ports.UserRepository, actor domain.User) error {
		var err error
		result, err = repo.User(ctx, userID, false)
		if err != nil {
			return err
		}
		if result == nil {
			return domain.ErrUserMissing
		}
		return nil
	})
	return result, err
}

func rolesValid(ctx context.Context, repo ports.RoleDirectory, names []string, inactiveError error) error {
	roles, err := repo.Roles(ctx)
	if err != nil {
		return err
	}
	for _, name := range names {
		found := false
		for _, role := range roles {
			if name == role.Name {
				found = true
				if !role.IsActive {
					return inactiveError
				}
			}
		}
		if !found {
			return domain.ErrUnknownRole
		}
	}
	return nil
}

func (s *UserService) UpdateUser(ctx context.Context, token, userID string, request domain.UserUpdate) (*domain.User, error) {
	var result *domain.User
	err := authorized(ctx, s.unit, s.tokens, token, "admin.users.write", true, func(ctx context.Context, repo ports.UserRepository, actor domain.User) error {
		user, err := repo.User(ctx, userID, true)
		if err != nil {
			return err
		}
		if user == nil {
			return domain.ErrUserMissing
		}
		if user.IsSuperuser && !actor.IsSuperuser {
			return domain.ErrForbidden
		}
		if request.RoleNames != nil {
			names := domain.NormalizedRoles(*request.RoleNames)
			request.RoleNames = &names
			if err = rolesValid(ctx, repo, names, domain.ErrEmailConflict); err != nil {
				return err
			}
		}
		result, err = repo.UpdateUser(ctx, userID, request, actor.ID)
		return err
	})
	return result, err
}

func (s *UserService) Deactivate(ctx context.Context, token, userID string) (*domain.User, error) {
	var result *domain.User
	err := authorized(ctx, s.unit, s.tokens, token, "admin.users.write", true, func(ctx context.Context, repo ports.UserRepository, actor domain.User) error {
		if actor.ID == userID {
			return domain.ErrSelfDeactivation
		}
		user, err := repo.User(ctx, userID, true)
		if err != nil {
			return err
		}
		if user == nil {
			return domain.ErrUserMissing
		}
		if user.IsActive && user.IsSuperuser {
			count, err := repo.ActiveSuperusers(ctx)
			if err != nil {
				return err
			}
			if count <= 1 {
				return domain.ErrLastSuperadmin
			}
		}
		return domain.ErrOffboardingRequired
	})
	return result, err
}

func (s *UserService) Online(ctx context.Context, token string) ([]domain.Activity, error) {
	var result []domain.Activity
	err := authorized(ctx, s.unit, s.tokens, token, "admin.users.read", false, func(ctx context.Context, repo ports.UserRepository, actor domain.User) error {
		var err error
		result, err = repo.Online(ctx, s.tokens.Now()-int64(s.policy.PresenceTimeout.Seconds()))
		return err
	})
	return result, err
}
