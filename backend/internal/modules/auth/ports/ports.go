package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/identity"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type IdentityReader interface {
	User(context.Context, string, bool) (*domain.User, error)
	AccessRevoked(context.Context, string) (bool, error)
}

type AdministrationRepository interface {
	AccessRevoked(context.Context, string) (bool, error)
	SetActor(context.Context, string) error
	Lock(context.Context, string) error
	User(context.Context, string, bool) (*domain.User, error)
}

type SessionRepository interface {
	TrackAccess(context.Context, string, string, string, int64) error
	Sessions(context.Context, string, string) ([]domain.Session, error)
	RevokeSession(context.Context, string, string) error
	RevokeAllSessions(context.Context, string) error
	LoginBlocked(context.Context, string) (bool, error)
	FailLogin(context.Context, string) error
	ClearLoginFailures(context.Context, string) error
	AccessRevoked(context.Context, string) (bool, error)
	SetActor(context.Context, string) error
	Lock(context.Context, string) error
	User(context.Context, string, bool) (*domain.User, error)
	FindUser(context.Context, string) (*domain.User, error)
	CreateRefresh(context.Context, domain.Refresh) error
	Refresh(context.Context, string, bool) (*domain.Refresh, error)
	RevokeRefresh(context.Context, string) error
	RevokeFamily(context.Context, string) error
	RecordActivity(context.Context, string, string) error
}

type RoleDirectory interface {
	Roles(context.Context) ([]domain.Role, error)
}

type UserRepository interface {
	EmailChangeRepository
	AccessRevoked(context.Context, string) (bool, error)
	SetActor(context.Context, string) error
	Lock(context.Context, string) error
	User(context.Context, string, bool) (*domain.User, error)
	Users(context.Context, domain.UserFilter) (domain.UserPage, error)
	Roles(context.Context) ([]domain.Role, error)
	CreateUser(context.Context, string, domain.UserCreate, string, string) (*domain.User, error)
	UpdateUser(context.Context, string, domain.UserUpdate, string) (*domain.User, error)
	SetPassword(context.Context, string, string, string) error
	SetActive(context.Context, string, bool, string) error
	RecordStatusChange(context.Context, string, string, string, string) error
	QueueHandover(context.Context, string, string, string, string, string) error
	ActiveSuperusers(context.Context) (int64, error)
	RevokeUserSessions(context.Context, string) error
	Online(context.Context, int64) ([]domain.Activity, error)
}

type RoleRepository interface {
	AdministrationHistory(context.Context, string) ([]domain.AdministrationChange, error)
	AccessRevoked(context.Context, string) (bool, error)
	SetActor(context.Context, string) error
	Lock(context.Context, string) error
	User(context.Context, string, bool) (*domain.User, error)
	Roles(context.Context) ([]domain.Role, error)
	Role(context.Context, string) (*domain.Role, error)
	PermissionCodes(context.Context) ([]string, error)
	CreateRole(context.Context, string, domain.RoleCreate, string) (*domain.Role, error)
	UpdateRole(context.Context, string, domain.RoleUpdate, string) (*domain.Role, error)
}

type AccountRepository interface {
	EmailChangeRepository
	AccessRevoked(context.Context, string) (bool, error)
	SetActor(context.Context, string) error
	Lock(context.Context, string) error
	UsernameAvailable(context.Context, string, string) (bool, error)
	SaveUsername(context.Context, string, string, string, string) error
	User(context.Context, string, bool) (*domain.User, error)
	UpdateUser(context.Context, string, domain.UserUpdate, string) (*domain.User, error)
	SetPassword(context.Context, string, string, string) error
	RevokeUserSessions(context.Context, string) error
	RecordAccountChange(context.Context, string, string, string) error
}

type BootstrapRepository interface {
	Lock(context.Context, string) error
	ReconcilePermission(context.Context, string, domain.Permission) error
	EnsureBootstrapRole(context.Context, string) (string, error)
	GrantRolePermission(context.Context, string, string) error
	BootstrapUser(context.Context, string, string, string, string) (string, bool, error)
	AssignRole(context.Context, string, string) error
}

// UnitOfWork supplies a capability-specific repository bound to one transaction.
type UnitOfWork[R any] interface {
	Within(context.Context, func(context.Context, R) error) error
}

type Passwords interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) bool
	VerifyAbsent(context.Context, string)
}
type Tokens interface {
	Issue(string, int64, int64) (string, int, error)
	Subject(string) (string, error)
	Version(string) (int64, error)
	Refresh() (string, string)
	HashRefresh(string) string
	ID() string
	Now() int64
}

type AssignmentReader interface {
	Assignment(context.Context, string) (*domain.AssignmentCandidate, error)
	Assignments(context.Context) ([]domain.AssignmentCandidate, error)
}

// Profile shares Auth's identity and transaction infrastructure.
type ProfileRepository interface {
	IdentityReader
	Lock(context.Context, string) error
	SaveContact(context.Context, string, domain.PersonalContact, string, string) error
	Get(context.Context, string) (*domain.Profile, error)
	Save(context.Context, string, *string, string, string, bool) (domain.Profile, error)
	Record(context.Context, domain.ProfileChange) error
}
type ProfileIdentity interface {
	Resolve(context.Context, string) (identity.Principal, error)
}
type ImageProcessor interface{ Canonical([]byte) ([]byte, error) }
type ImageStorage interface {
	Store(context.Context, string, []byte) (string, error)
	Retrieve(context.Context, string, string) ([]byte, error)
}
type ProfileValues interface{ ID() string }

type EmailChangeRepository interface {
	User(context.Context, string, bool) (*domain.User, error)
	SetActor(context.Context, string) error
	CancelEmailChanges(context.Context, string) error
	CancelCredentialForUser(context.Context, string) error
	SaveEmailChange(context.Context, domain.EmailChange) error
	EmailChange(context.Context, string) (*domain.EmailChange, error)
}
