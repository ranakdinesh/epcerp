package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

type RecordRepository interface {
	LockIdentity(context.Context) error
	ParticipantActive(context.Context, string, string) (bool, error)
	SaveParticipant(context.Context, string, string, string, domain.ParticipantGrant) (int64, error)
	GetNamed(context.Context, string) (domain.Record, error)
	CreateNamed(context.Context, domain.NamedInput, string, string, string, string) (domain.Record, error)
}
type EligibleUsers interface {
	Get(context.Context, string) (*domain.EligibleIdentity, error)
	List(context.Context) ([]domain.Candidate, error)
}
