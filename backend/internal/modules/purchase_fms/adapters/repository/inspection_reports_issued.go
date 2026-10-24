package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type inspectionReportsIssuedReader struct {
	c database.Connection
	p InspectionReportsProviders
}

func NewInspectionReportsIssuedReader(c database.Connection, p InspectionReportsProviders) contracts.InspectionReportsIssuedReader {
	return &inspectionReportsIssuedReader{c, p}
}
func (r *inspectionReportsIssuedReader) Issued(ctx context.Context, project, pkg string) (*contracts.InspectionReportsIssued, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Material report clearance requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.InspectionReportsID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&inspectionReportsRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewInspectionReportsContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.InspectionReportsIssuedFact(record, source)
}
