package ports

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type RecoveryRepository interface {
	User(context.Context, string, bool) (*domain.User, error)
	FindEmail(context.Context, string) (*domain.User, error)
	RecoveryUserID(context.Context, string) (string, error)
	ValidRecovery(context.Context, string) (bool, error)
	RecentlyRequested(context.Context, string) (bool, error)
	CreateRecovery(context.Context, string, string, int64) error
	ConsumeRecoveries(context.Context, string) error
	SetPassword(context.Context, string, string, string) error
	RevokeUserSessions(context.Context, string) error
	RecordAccountChange(context.Context, string, string, string) error
}

type RecoveryMail interface {
	Ready() bool
	SendRecovery(context.Context, string, string) error
}
