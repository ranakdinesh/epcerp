package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type qualityVerifiedReader struct {
	c database.Connection
	p QualityProviders
}

func NewQualityVerifiedReader(c database.Connection, p QualityProviders) contracts.QualityVerifiedReader {
	return &qualityVerifiedReader{c, p}
}
func (r *qualityVerifiedReader) Verified(ctx context.Context, project, pkg string) (*contracts.QualityVerified, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Quality verification requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.QualityID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&qualityRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewQualityContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.QualityVerifiedFact(record, source, domain.QualityToday(Values{}.Now()))
}
