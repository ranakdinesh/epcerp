package application

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type BootstrapService struct {
	unit      ports.UnitOfWork[ports.BootstrapRepository]
	passwords ports.Passwords
	tokens    ports.Tokens
}

func NewBootstrap(u ports.UnitOfWork[ports.BootstrapRepository], p ports.Passwords, t ports.Tokens) *BootstrapService {
	return &BootstrapService{u, p, t}
}

// Bootstrap reconciles declared permissions and the recovery administrator at startup.
// Existing administrator-managed credentials and roles are preserved.
func (s *BootstrapService) Bootstrap(ctx context.Context, username, email, password string) (bool, error) {
	hash, err := s.passwords.Hash(ctx, password)
	if err != nil {
		return false, err
	}
	created := false
	err = s.unit.Within(ctx, func(ctx context.Context, repo ports.BootstrapRepository) error {
		if err := repo.Lock(ctx, "auth.administration"); err != nil {
			return err
		}
		for _, permission := range Registry() {
			if err := repo.ReconcilePermission(ctx, s.tokens.ID(), permission); err != nil {
				return err
			}
		}
		role, err := repo.EnsureBootstrapRole(ctx, s.tokens.ID())
		if err != nil {
			return err
		}
		for _, permission := range Registry() {
			if err := repo.GrantRolePermission(ctx, role, permission.Code); err != nil {
				return err
			}
		}
		user, made, err := repo.BootstrapUser(ctx, s.tokens.ID(), username, email, hash)
		if err != nil {
			return err
		}
		created = made
		return repo.AssignRole(ctx, user, role)
	})
	return created, err
}
