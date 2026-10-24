package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type workflowProgressReader struct {
	c database.Connection
	p WorkflowProviders
}

func NewWorkflowProgressReader(c database.Connection, p WorkflowProviders) contracts.WorkflowProgressReader {
	return &workflowProgressReader{c, p}
}
func (r *workflowProgressReader) Page(ctx context.Context, project string, demands []string, page, size int32) (out domain.WorkflowProgressPage, e error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return out, errors.New("Progress package reads require the consuming transaction")
	}
	if e = domain.WorkflowID(project); e != nil {
		return out, e
	}
	if len(demands) > 200 || page < 1 || page > 100000 || size < 1 || size > 100 {
		return out, domain.WorkflowInvalid("query", "Use bounded demand/page scope")
	}
	out.Rows = []domain.WorkflowProgressPackage{}
	if len(demands) == 0 {
		return out, nil
	}
	seen := map[string]bool{}
	ids := []pgtype.UUID{}
	for _, id := range demands {
		if e = domain.WorkflowID(id); e != nil {
			return domain.WorkflowProgressPage{}, e
		}
		if seen[id] {
			return domain.WorkflowProgressPage{}, domain.WorkflowInvalid("demands", "Use distinct demand identifiers")
		}
		seen[id] = true
		ids = append(ids, dtos.UUID(id))
	}
	// Serialize source inspection with owner writes before counting/paging records.
	parent, e := r.p.Projects.Get(ctx, project, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.WorkflowMissing
	}
	q := NewStore(r.c)
	out.Total, e = q.WorkflowCountProgressPackages(ctx, sqlc.WorkflowCountProgressPackagesParams{ProjectID: dtos.UUID(project), DemandIds: ids})
	if e != nil {
		return domain.WorkflowProgressPage{}, e
	}
	rows, e := q.WorkflowPageProgressPackages(ctx, sqlc.WorkflowPageProgressPackagesParams{ProjectID: dtos.UUID(project), DemandIds: ids, PageSize: size, RecordOffset: (page - 1) * size})
	if e != nil {
		return domain.WorkflowProgressPage{}, e
	}
	for _, row := range rows {
		p, e := dtos.WorkflowRecord(row)
		if e != nil {
			return domain.WorkflowProgressPage{}, e
		}
		source, e := (&workflowSourceContext{r.p}).Source(ctx, project, p.IndentID, p.DemandID)
		if e != nil {
			return domain.WorkflowProgressPage{}, e
		}
		out.Rows = append(out.Rows, domain.WorkflowProgressPackage{Package: p, Source: source})
	}
	return out, nil
}
