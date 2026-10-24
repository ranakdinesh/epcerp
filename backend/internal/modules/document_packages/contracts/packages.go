package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
)

type Receipt = domain.Receipt
type Disposition = domain.Disposition
type Member = domain.Member
type Error = domain.Error

// Consumer is bound to the caller's exact shared transaction. Consumption and
// source publication must commit atomically with the named Tender command.
type Consumer interface {
	Consume(context.Context, string, string, int64, string) (Receipt, error)
	MemberIDs(context.Context, Receipt) ([]string, error)
	Member(context.Context, Receipt, string) (Member, error)
}

// DraftConsumer retains uploads before verification, then publishes ready members
// atomically with source links and reader jobs in the caller's shared transaction.
type DraftConsumer interface {
	ReadyDrafts(context.Context) ([]Receipt, error)
	Dispositions(context.Context, Receipt) ([]Disposition, error)
	MemberInfo(context.Context, Receipt, string) (Member, error)
	Retain(context.Context, string, string, string) error
	Receipts(context.Context, string, string) ([]Receipt, error)
	Publish(context.Context, string, string, string) (Receipt, error)
	MemberIDs(context.Context, Receipt) ([]string, error)
	Member(context.Context, Receipt, string) (Member, error)
}
