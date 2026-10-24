package dtos

import (
	"slices"
	"time"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type UserResponse struct {
	FirstName          string    `json:"first_name"`
	LastName           string    `json:"last_name"`
	Mobile             string    `json:"mobile"`
	MustChangePassword bool      `json:"must_change_password"`
	EmailStatus        string    `json:"email_status"`
	ID                 string    `json:"id"`
	Username           string    `json:"username"`
	Email              string    `json:"email"`
	Active             bool      `json:"is_active"`
	Superuser          bool      `json:"is_superuser"`
	Roles              []string  `json:"roles"`
	CreatedAt          time.Time `json:"created_at"`
}

func User(user domain.User) UserResponse {
	roles := []string{}
	for _, role := range user.Roles {
		roles = append(roles, role.Name)
	}
	slices.Sort(roles)
	return UserResponse{ID: user.ID, Username: user.Username, FirstName: user.FirstName, LastName: user.LastName, Mobile: user.Mobile, Email: user.Email, Active: user.IsActive, Superuser: user.IsSuperuser, Roles: roles, CreatedAt: user.CreatedAt, MustChangePassword: user.MustChangePassword, EmailStatus: user.EmailStatus}
}

type RoleResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Active      bool     `json:"is_active"`
	Permissions []string `json:"permissions"`
}

func Role(role domain.Role) RoleResponse {
	codes := append([]string{}, role.Permissions...)
	slices.Sort(codes)
	return RoleResponse{role.ID, role.Name, role.Description, role.IsActive, codes}
}

type PermissionResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

func Permissions(values []domain.Permission) []PermissionResponse {
	r := []PermissionResponse{}
	for _, v := range values {
		r = append(r, PermissionResponse{v.Code, v.Description, v.Group})
	}
	return r
}

type ActivityResponse struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	CurrentPage string    `json:"current_page"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

func Activities(values []domain.Activity) []ActivityResponse {
	r := []ActivityResponse{}
	for _, v := range values {
		r = append(r, ActivityResponse{v.UserID, v.Username, v.CurrentPage, v.LastSeenAt})
	}
	return r
}

type TokenResponse struct {
	MustChangePassword bool   `json:"must_change_password"`
	AccessToken        string `json:"access_token"`
	RefreshToken       string `json:"refresh_token"`
	TokenType          string `json:"token_type"`
	ExpiresIn          int    `json:"expires_in"`
	RefreshExpiresIn   int    `json:"refresh_expires_in"`
}

func Tokens(v domain.TokenPair) TokenResponse {
	return TokenResponse{AccessToken: v.AccessToken, RefreshToken: v.RefreshToken, TokenType: v.TokenType, ExpiresIn: v.ExpiresIn, RefreshExpiresIn: v.RefreshExpiresIn, MustChangePassword: v.MustChangePassword}
}
