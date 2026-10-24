package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type OffboardingRepository interface {
	OrganisationRepository
	OffboardingHistory(context.Context, string) ([]domain.OffboardingChange, error)
	Cases(context.Context, string, bool) ([]domain.OffboardingCase, error)
	Case(context.Context, string) (*domain.OffboardingCase, error)
	SaveCase(context.Context, domain.OffboardingCase, string) (domain.OffboardingCase, error)
	RecordOffboarding(context.Context, domain.OffboardingCase, string, string, string, string) error
	ActiveSuperusers(context.Context) (int64, error)
	Deactivate(context.Context, string, string) error
	RevokeUserSessions(context.Context, string) error
}
