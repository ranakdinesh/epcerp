package domain

import (
	"slices"
	"strings"
	"time"
)

type User struct {
	ID, Username, Email, PasswordHash string
	FirstName, LastName, Mobile       string
	IsActive, IsSuperuser             bool
	MustChangePassword                bool
	TemporaryExpiresAt                *time.Time
	CredentialVersion                 int64
	EmailStatus                       string
	CreatedAt                         time.Time
	Roles                             []Role
}
type Role struct {
	ID, Name, Description string
	IsActive              bool
	Permissions           []string
}
type Permission struct {
	Code        string
	Description string
	Group       string
}
type Refresh struct {
	ID, UserID, Hash, FamilyID string
	ExpiresAt                  time.Time
	RevokedAt                  *time.Time
}
type Activity struct {
	UserID      string
	Username    string
	CurrentPage string
	LastSeenAt  time.Time
}
type TokenPair struct {
	MustChangePassword bool
	AccessToken        string
	RefreshToken       string
	TokenType          string
	ExpiresIn          int
	RefreshExpiresIn   int
}
type UserFilter struct {
	Page, PageSize int
	Search, Role   string
	Active         *bool
	Descending     bool
}
type UserPage struct {
	Items []User
	Total int64
}
type UserCreate struct {
	IsSuperuser                 bool
	Username, Email             string
	FirstName, LastName, Mobile string
	RoleNames                   []string
}
type UserUpdate struct {
	Email     *string
	RoleNames *[]string
}
type RoleCreate struct {
	Name, Description string
	PermissionCodes   []string
}
type RoleUpdate struct {
	Description     *string
	PermissionCodes *[]string
	Active          *bool
}

// Policy contains only lifetime values, never infrastructure settings or JWT keys.
type Policy struct{ SessionLifetime, PresenceTimeout time.Duration }

type ErrorKind string

const (
	Unauthorized ErrorKind = "unauthorized"
	Forbidden    ErrorKind = "forbidden"
	Missing      ErrorKind = "missing"
	Conflict     ErrorKind = "conflict"
	Invalid      ErrorKind = "invalid"
	Unavailable  ErrorKind = "unavailable"
)

type Error struct {
	Kind   ErrorKind
	Detail string
}

func (e *Error) Error() string { return e.Detail }

var (
	ErrUnauthorized      = &Error{Unauthorized, "Invalid or expired credentials"}
	ErrForbidden         = &Error{Forbidden, "Insufficient permission"}
	ErrUserMissing       = &Error{Missing, "User not found"}
	ErrRoleMissing       = &Error{Missing, "Role not found"}
	ErrUnknownRole       = &Error{Invalid, "Unknown role"}
	ErrUnknownPermission = &Error{Invalid, "Unknown permission"}
	ErrUserConflict      = &Error{Conflict, "Username or email is already in use"}
	ErrEmailConflict     = &Error{Conflict, "Email is already in use"}
	ErrRoleConflict      = &Error{Conflict, "Role name is already in use"}
	ErrSelfDeactivation  = &Error{Invalid, "You cannot deactivate yourself"}
	ErrLastSuperadmin    = &Error{Invalid, "At least one active superadmin must remain"}
	ErrSuperadminLimit   = &Error{Conflict, "The system can have at most two superadmin accounts, including inactive accounts"}
)

func EffectivePermissions(user User) []string {
	if user.MustChangePassword {
		return []string{}
	}
	codes := map[string]bool{}
	if user.IsActive {
		for _, code := range []string{"account.profile.read", "account.profile.write", "account.security.write", "account.team.read", "account.offboarding.read", "account.offboarding.write"} {
			codes[code] = true
		}
	}
	for _, role := range user.Roles {
		if role.IsActive {
			for _, code := range role.Permissions {
				codes[code] = true
			}
		}
	}
	if user.IsSuperuser {
		codes["system:admin"] = true
	}
	result := make([]string, 0, len(codes))
	for code := range codes {
		result = append(result, code)
	}
	slices.Sort(result)
	return result
}
func HasPermission(user User, code string) bool {
	return user.IsActive && !user.MustChangePassword && (user.IsSuperuser || slices.Contains(EffectivePermissions(user), code))
}

func At(unix int64) time.Time { return time.Unix(unix, 0).UTC() }
func NormalizedRoles(names []string) []string {
	set := map[string]bool{}
	for _, name := range names {
		set[strings.ToLower(strings.TrimSpace(name))] = true
	}
	result := []string{}
	for name := range set {
		result = append(result, name)
	}
	slices.Sort(result)
	return result
}

// AssignmentCandidate is the limited identity view published to assignment consumers.
type AssignmentCandidate struct {
	Ready             bool // Permanent credentials established; no pending forced password change.
	ID, Name          string
	Active, Superuser bool
	Permissions       []string
}

func (c AssignmentCandidate) Allows(permission string) bool {
	if !c.Active {
		return false
	}
	if c.Superuser {
		return true
	}
	for _, code := range c.Permissions {
		if code == permission {
			return true
		}
	}
	return false
}
