package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type EmailSender interface {
	Ready() bool
	Send(context.Context, string, string, string) error
}
type TemporaryPasswords interface{ Generate() (string, error) }
type CredentialRepository interface {
	AdministrationRepository
	Superusers(context.Context) (int64, error)
	Roles(context.Context) ([]domain.Role, error)
	CreateUser(context.Context, string, domain.UserCreate, string, string) (*domain.User, error)
	RevokeUserSessions(context.Context, string) error
	SetTemporaryPassword(context.Context, string, string, string) error
	QueueCredentialEmail(context.Context, domain.CredentialEmail, int64, string) error
	RecordCredentialChange(context.Context, string, string, string, string, string) error
}
type EmailDeliveryRepository interface {
	Claim(context.Context, string) (domain.EmailJob, error)
	Finish(context.Context, string, string, bool) error
}
