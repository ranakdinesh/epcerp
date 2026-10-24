package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
)

type Notice = domain.Notice

// Writer is transaction-bound. The caller owns Finance authority and decision truth.
// No recipient is accepted: first delivery resolves the effective Director.
type Writer interface {
	DirectorDecision(context.Context, string) (Notice, error)
}

// Error publishes safe notice delivery failures to consuming capabilities.
type Error = domain.Error

type AddressedWriter interface {
	QuotationNotice(context.Context, string, string, string, string) error
}

type SupplierReviewWriter interface {
	SupplierReviewNotice(context.Context, string, string, string, string) error
}

// TenderWriter resolves the exact addressed recipient from a recorded source action.
type TenderWriter interface {
	TenderAssignment(context.Context, string) (string, error)
}
