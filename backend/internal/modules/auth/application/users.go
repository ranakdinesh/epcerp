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
	mail      ports.EmailSender
	publicURL string
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
	raw, recipient := "", ""
	err := authorized(ctx, s.unit, s.tokens, token, "admin.users.write", true, func(ctx context.Context, repo ports.UserRepository, actor domain.User) error {
		user, err := repo.User(ctx, userID, true)
		if err != nil {
			return err
		}
		if user == nil {
			return domain.ErrUserMissing
		}
		if request.ExpectedVersion != nil && *request.ExpectedVersion != user.EditVersion {
			return &domain.Error{Kind: domain.Conflict, Detail: "This user changed while you were editing. Reload and review the saved details."}
		}
		if user.IsSuperuser && !actor.IsSuperuser {
			return domain.ErrForbidden
		}
		if request.RoleNames != nil {
			names := domain.NormalizedRoles(*request.RoleNames)
			if !actor.IsSuperuser {
				for _, name := range names {
					if name == "superadmin" {
						return domain.ErrForbidden
					}
				}
			}
			request.RoleNames = &names
			newNames := []string{}
			for _, name := range names {
				existed := false
				for _, old := range user.Roles {
					if old.Name == name {
						existed = true
						break
					}
				}
				if !existed {
					newNames = append(newNames, name)
				}
			}
			if err = rolesValid(ctx, repo, newNames, domain.ErrInactiveRole); err != nil {
				return err
			}
		}
		if request.Email != nil && *request.Email != user.Email {
			if s.mail == nil || !s.mail.Ready() {
				return &domain.Error{Kind: domain.Unavailable, Detail: "Email verification delivery is not configured."}
			}
			recipient = *request.Email
			raw, err = stageEmail(ctx, repo, s.tokens, *user, recipient, actor.ID)
			if err != nil {
				return err
			}
			request.Email = nil
		}
		result, err = repo.UpdateUser(ctx, userID, request, actor.ID)
		return err
	})
	if err == nil && raw != "" {
		err = sendVerification(ctx, s.mail, s.publicURL, recipient, raw)
	}
	return result, err
}

// Deactivate keeps the existing endpoint safe for older clients.
func (s *UserService) Deactivate(ctx context.Context, token, userID string) (*domain.User, error) {
	return s.SetActive(ctx, token, userID, false, "Administrator revoked account access.")
}

func (s *UserService) SetActive(ctx context.Context, token, userID string, active bool, reason string) (*domain.User, error) {
	if !domain.ValidResetReason(reason) {
		return nil, &domain.Error{Kind: domain.Invalid, Detail: "Provide a status change reason of 10 to 1000 characters."}
	}
	var result *domain.User
	err := authorized(ctx, s.unit, s.tokens, token, "admin.users.write", true, func(ctx context.Context, repo ports.UserRepository, actor domain.User) error {
		if !active && actor.ID == userID {
			return domain.ErrSelfDeactivation
		}
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
		if user.IsActive == active {
			result = user
			return nil
		}
		if !active && user.IsSuperuser {
			count, err := repo.ActiveSuperusers(ctx)
			if err != nil {
				return err
			}
			if count <= 1 {
				return domain.ErrLastSuperadmin
			}
		}
		// Both transitions advance the credential version. Reactivation cannot
		// resurrect an access token or refresh session issued before revocation.
		if err = repo.SetActive(ctx, userID, active, actor.ID); err != nil {
			return err
		}
		if err = repo.RevokeUserSessions(ctx, userID); err != nil {
			return err
		}
		if err = repo.RecordStatusChange(ctx, s.tokens.ID(), userID, actor.ID, reason); err != nil {
			return err
		}
		if !active {
			if err = repo.QueueHandover(ctx, userID, s.tokens.ID(), s.tokens.ID(), actor.ID, reason); err != nil {
				return err
			}
		}
		result, err = repo.User(ctx, userID, false)
		return err
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
