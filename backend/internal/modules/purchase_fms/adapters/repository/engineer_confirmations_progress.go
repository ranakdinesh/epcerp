package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type engineerConfirmationsProgressReader struct{ c database.Connection }

func NewEngineerConfirmationsProgressReader(c database.Connection) contracts.EngineerConfirmationsProgressReader {
	return &engineerConfirmationsProgressReader{c}
}
func (r *engineerConfirmationsProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.EngineerConfirmationsProgressSource, upstreamCurrent bool) (domain.EngineerConfirmationsProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.EngineerConfirmationsProgress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.EngineerConfirmationsID(id); e != nil {
			return domain.EngineerConfirmationsProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.EngineerConfirmationsProgress{}, domain.EngineerConfirmationsInvalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&engineerConfirmationsRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.EngineerConfirmationsProgress{}, err
	}
	return domain.EngineerConfirmationsProgressFact(record, expected, upstreamCurrent), nil
}
