package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
)

type Designation = domain.Designation

// Directory must share the consuming Finance/Communication transaction and context.
// Nil means mandatory Director notice cannot currently be addressed.
type Directory interface {
	Current(context.Context) (*Designation, error)
}
