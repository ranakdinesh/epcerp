package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type inspectionsProgressReader struct{ c database.Connection }

func NewInspectionsProgressReader(c database.Connection) contracts.InspectionsProgressReader {
	return &inspectionsProgressReader{c}
}
func (r *inspectionsProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.InspectionsProgressSource, upstreamCurrent bool) (domain.InspectionsProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.InspectionsProgress{}, errors.New("Inspection progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.InspectionsID(id); e != nil {
			return domain.InspectionsProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.InspectionsProgress{}, domain.InspectionsInvalid("source", "Inspection reporting source must match project and package")
	}
	record, err := (&inspectionsRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.InspectionsProgress{}, err
	}
	return domain.InspectionsProgressFact(record, expected, upstreamCurrent), nil
}
