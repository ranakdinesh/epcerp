package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

type TSCRepository interface {
	CloudBudget(context.Context, string) (int64, error)
	ReserveCloud(context.Context, domain.TSCCloudCall) (domain.TSCCloudCall, bool, error)
	CompleteCloud(context.Context, domain.TSCCloudCall, domain.TSCAnswer, string) error
	Authorize(context.Context, string, string) error
	Snapshot(context.Context, domain.DocumentDraft) (domain.TSCInput, error)
	FindOperation(context.Context, string, string, string) (*domain.TSCJob, error)
	Create(context.Context, domain.DocumentDraft, string, string, string, string, domain.TSCInput) (domain.TSCJob, error)
	Get(context.Context, string, string) (domain.TSCJob, error)
	List(context.Context, string) ([]domain.TSCJob, error)
	Edit(context.Context, domain.TSCJob, int64, domain.TSCAnswer, string, string) (domain.TSCJob, error)
	Claim(context.Context) (*domain.TSCJob, error)
	Renew(context.Context, domain.TSCJob) error
	Finish(context.Context, domain.TSCJob, domain.TSCResult, string) error
	ExportRequested(context.Context, domain.TSCJob, string) error
}
type TSCUnit interface {
	Within(context.Context, func(context.Context, TSCRepository, DraftRepository) error) error
}
type TSCProvider interface {
	Model() string
	Propose(context.Context, domain.TSCQuestion, []domain.TSCPassage) (domain.TSCAnswer, error)
}
type TSCExporter interface {
	Export(domain.TSCJob) ([]byte, error)
}
