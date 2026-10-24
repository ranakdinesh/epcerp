package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

type DraftRepository interface {
	Pending(context.Context) ([]domain.DocumentDraft, error)
	Authorize(context.Context, string, bool) error
	Create(context.Context, string, string, string) (domain.DocumentDraft, error)
	Get(context.Context, string, string) (domain.DocumentDraft, error)
	List(context.Context, string, domain.DraftQuery) ([]domain.DocumentDraft, int64, error)
	Attach(context.Context, domain.DocumentDraft, string, string) error
	Populate(context.Context, *domain.DocumentDraft) error
	Publish(context.Context, domain.DocumentDraft) error
	Artifact(context.Context, domain.DocumentDraft, string, string) ([]byte, error)
	Retry(context.Context, domain.DocumentDraft, string, int32) error
}
type DraftUnit interface {
	Within(context.Context, func(context.Context, DraftRepository) error) error
}
