package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type inspectionReportsProgressReader struct{ c database.Connection }

func NewInspectionReportsProgressReader(c database.Connection) contracts.InspectionReportsProgressReader {
	return &inspectionReportsProgressReader{c}
}
func (r *inspectionReportsProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.InspectionReportsProgressSource, upstreamCurrent bool) (domain.InspectionReportsProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.InspectionReportsProgress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.InspectionReportsID(id); e != nil {
			return domain.InspectionReportsProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.InspectionReportsProgress{}, domain.InspectionReportsInvalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&inspectionReportsRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.InspectionReportsProgress{}, err
	}
	return domain.InspectionReportsProgressFact(record, expected, upstreamCurrent), nil
}
