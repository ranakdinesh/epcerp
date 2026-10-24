package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
)

type Repository interface {
	Record(context.Context, domain.Decision) error
}
type Authority interface {
	ApproveBudget(context.Context, domain.Actor, string) error
}
type Notices interface {
	Director(context.Context, string) error
}
type Values interface {
	ID() string
	Now() domain.Timestamp
}

type QuotationRepository interface {
	RecordQuotation(context.Context, domain.QuotationDecision) error
}
type QuotationAuthority interface {
	ReviewQuotation(context.Context, domain.Actor, string) error
}

type AdvanceRepository interface {
	RecordAdvance(context.Context, domain.AdvanceDecision) error
}
type AdvanceAuthority interface {
	ReviewAdvance(context.Context, domain.Actor, string) error
}
