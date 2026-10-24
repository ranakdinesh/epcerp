package application

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type RoleService struct {
	unit   ports.UnitOfWork[ports.RoleRepository]
	tokens ports.Tokens
}

func NewRoles(unit ports.UnitOfWork[ports.RoleRepository], tokens ports.Tokens) *RoleService {
	return &RoleService{unit: unit, tokens: tokens}
}

func (s *RoleService) Roles(ctx context.Context, token string) ([]domain.Role, error) {
	var result []domain.Role
	err := authorized(ctx, s.unit, s.tokens, token, "admin.roles.read", false, func(ctx context.Context, repo ports.RoleRepository, actor domain.User) error {
		var err error
		result, err = repo.Roles(ctx)
		return err
	})
	return result, err
}

func (s *RoleService) Permissions(ctx context.Context, token string) ([]domain.Permission, error) {
	result := []domain.Permission{}
	err := authorized(ctx, s.unit, s.tokens, token, "admin.roles.read", false, func(ctx context.Context, repo ports.RoleRepository, actor domain.User) error {
		codes, err := repo.PermissionCodes(ctx)
		if err != nil {
			return err
		}
		for _, code := range codes {
			for _, definition := range Registry() {
				if code == definition.Code {
					result = append(result, definition)
				}
			}
		}
		return nil
	})
	return result, err
}

func validCodes(ctx context.Context, repo ports.RoleRepository, codes []string) error {
	persisted, err := repo.PermissionCodes(ctx)
	if err != nil {
		return err
	}
	for _, code := range codes {
		declared, present := false, false
		for _, p := range Registry() {
			if p.Code == code {
				declared = true
			}
		}
		for _, p := range persisted {
			if p == code {
				present = true
			}
		}
		if !declared || !present {
			return domain.ErrUnknownPermission
		}
	}
	return nil
}

func (s *RoleService) CreateRole(ctx context.Context, token string, request domain.RoleCreate) (*domain.Role, error) {
	var result *domain.Role
	err := authorized(ctx, s.unit, s.tokens, token, "admin.roles.write", true, func(ctx context.Context, repo ports.RoleRepository, actor domain.User) error {
		if err := validCodes(ctx, repo, request.PermissionCodes); err != nil {
			return err
		}
		var err error
		result, err = repo.CreateRole(ctx, s.tokens.ID(), request, actor.ID)
		return err
	})
	return result, err
}

func (s *RoleService) UpdateRole(ctx context.Context, token, roleID string, request domain.RoleUpdate) (*domain.Role, error) {
	var result *domain.Role
	err := authorized(ctx, s.unit, s.tokens, token, "admin.roles.write", true, func(ctx context.Context, repo ports.RoleRepository, actor domain.User) error {
		role, err := repo.Role(ctx, roleID)
		if err != nil {
			return err
		}
		if role == nil {
			return domain.ErrRoleMissing
		}
		if request.PermissionCodes != nil {
			if err = validCodes(ctx, repo, *request.PermissionCodes); err != nil {
				return err
			}
		}
		result, err = repo.UpdateRole(ctx, roleID, request, actor.ID)
		return err
	})
	return result, err
}

func (s *RoleService) FinancePresets(ctx context.Context, token string) ([]domain.Role, error) {
	result := []domain.Role{}
	err := authorized(ctx, s.unit, s.tokens, token, "admin.roles.write", true, func(ctx context.Context, repo ports.RoleRepository, actor domain.User) error {
		existing, err := repo.Roles(ctx)
		if err != nil {
			return err
		}
		for _, preset := range FinancePresets() {
			found := false
			for _, role := range existing {
				if role.Name == preset.Name {
					result = append(result, role)
					found = true
					break
				}
			}
			if found {
				continue
			}
			if err = validCodes(ctx, repo, preset.PermissionCodes); err != nil {
				return err
			}
			role, err := repo.CreateRole(ctx, s.tokens.ID(), preset, actor.ID)
			if err != nil {
				return err
			}
			result = append(result, *role)
		}
		return nil
	})
	return result, err
}
