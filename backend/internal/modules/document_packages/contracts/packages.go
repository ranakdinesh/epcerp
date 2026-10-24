package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
)

type Receipt = domain.Receipt
type Member = domain.Member
type Error = domain.Error

// Consumer is bound to the caller's exact shared transaction. Consumption and
// source publication must commit atomically with the named Tender command.
type Consumer interface {
	Consume(context.Context, string, string, int64, string) (Receipt, error)
	MemberIDs(context.Context, Receipt) ([]string, error)
	Member(context.Context, Receipt, string) (Member, error)
}
