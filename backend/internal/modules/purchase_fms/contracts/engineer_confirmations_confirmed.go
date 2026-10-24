package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type EngineerConfirmationsConfirmed = domain.EngineerConfirmationsRecord
type EngineerConfirmationsError = domain.EngineerConfirmationsError

// Current engineer confirmation evidence; consumer authorizes its accounting workflow.
type EngineerConfirmationsConfirmedReader interface {
	Confirmed(context.Context, string, string) (*EngineerConfirmationsConfirmed, error)
}
