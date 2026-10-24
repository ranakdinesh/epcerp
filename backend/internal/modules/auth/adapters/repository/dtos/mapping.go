package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func User(row sqlc.AuthUser) *domain.User {
	user := &domain.User{MustChangePassword: row.MustChangePassword, CredentialVersion: row.CredentialVersion, ID: row.ID.String(), Username: row.Username, FirstName: row.FirstName, LastName: row.LastName, Mobile: row.Mobile, Email: row.Email, PasswordHash: row.PasswordHash, IsActive: row.IsActive, IsSuperuser: row.IsSuperuser, CreatedAt: row.CreatedAt.Time, Roles: []domain.Role{}}
	if row.TemporaryPasswordExpiresAt.Valid {
		expiry := row.TemporaryPasswordExpiresAt.Time
		user.TemporaryExpiresAt = &expiry
	}
	return user
}
func UserRole(row sqlc.ListUserRolesRow) (domain.Role, error) {
	codes := []string{}
	err := json.Unmarshal(row.PermissionCodes, &codes)
	return domain.Role{ID: row.ID.String(), Name: row.Name, Description: row.Description, IsActive: row.IsActive, Permissions: codes}, err
}
func Role(row sqlc.AuthRole, codes []string) *domain.Role {
	return &domain.Role{ID: row.ID.String(), Name: row.Name, Description: row.Description, IsActive: row.IsActive, Permissions: codes}
}
func Refresh(row sqlc.AuthRefreshToken) *domain.Refresh {
	v := &domain.Refresh{ID: row.ID.String(), UserID: row.UserID.String(), Hash: row.TokenHash, FamilyID: row.FamilyID.String(), ExpiresAt: row.ExpiresAt.Time}
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		v.RevokedAt = &t
	}
	return v
}
func Activity(row sqlc.OnlineActivityRow) domain.Activity {
	return domain.Activity{UserID: row.UserID.String(), Username: row.Username, CurrentPage: row.CurrentPage, LastSeenAt: row.LastSeenAt.Time}
}

func Assignment(id, name string, active, superuser bool, raw []byte) (domain.AssignmentCandidate, error) {
	v := domain.AssignmentCandidate{ID: id, Name: name, Active: active, Superuser: superuser}
	err := json.Unmarshal(raw, &v.Permissions)
	return v, err
}
func Bundle(id, name string, active bool, permissions []string) contracts.Bundle {
	return contracts.Bundle{ID: id, Name: name, Active: active, Permissions: permissions}
}
