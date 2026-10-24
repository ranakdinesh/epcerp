package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type inspectionsClearedReader struct {
	c database.Connection
	p InspectionsProviders
}

func NewInspectionsClearedReader(c database.Connection, p InspectionsProviders) contracts.InspectionsClearedReader {
	return &inspectionsClearedReader{c, p}
}
func (r *inspectionsClearedReader) Cleared(ctx context.Context, project, pkg string) (*contracts.InspectionsCleared, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Material inspection clearance requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.InspectionsID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&inspectionsRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewInspectionsContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.InspectionsClearedFact(record, source)
}
