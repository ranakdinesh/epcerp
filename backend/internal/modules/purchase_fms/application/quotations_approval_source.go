package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

// ApprovalSource projects immutable negotiated revisions using current source
// eligibility. It does not resolve a user or start another transaction: the
// consuming Finance use case owns authorization and the shared transaction.
func QuotationsApprovalSource(ctx context.Context, r ports.QuotationsRepository, c ports.QuotationsContext, groups ports.QuotationsApprovalGroups, now domain.QuotationsTimestamp, project, bo string) (domain.QuotationsApprovalSource, error) {
	var out domain.QuotationsApprovalSource
	var e error
	if project, e = domain.QuotationsID(project); e != nil {
		return out, e
	}
	if bo, e = domain.QuotationsID(bo); e != nil {
		return out, e
	}
	source, e := c.Source(ctx, project, bo)
	if e != nil {
		return out, e
	}
	current, e := groups.Current(ctx, project, bo)
	if e != nil {
		return out, e
	}
	revisions, e := r.Revisions(ctx, project, bo)
	if e != nil {
		return out, e
	}
	return domain.QuotationsApprovalSource{ProjectID: project, BOID: bo, Currency: source.Currency, Available: source.Available, Groups: current, Quotes: domain.QuotationsCompare(revisions, source.Documents, domain.QuotationsToday(now), source.Currency)}, nil
}
