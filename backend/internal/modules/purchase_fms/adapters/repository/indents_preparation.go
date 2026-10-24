package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

type indentsPreparationReader struct {
	c database.Connection
	p IndentsProviders
}

func NewIndentsPreparationReader(c database.Connection, p IndentsProviders) contracts.IndentsPreparationReader {
	return &indentsPreparationReader{c, p}
}
func (r *indentsPreparationReader) Line(ctx context.Context, p, id string) (contracts.IndentsPreparationSource, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return contracts.IndentsPreparationSource{}, errors.New("Preparation source requires the consuming transaction")
	}
	return (&indentsIndentContext{r.p}).Line(ctx, p, id)
}
