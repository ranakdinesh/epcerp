package repository

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func (c *indentsIndentContext) WorkspaceSource(ctx context.Context, project string, allCandidates bool, records []domain.IndentsRecord) (domain.IndentsWorkspaceSource, error) {
	out := domain.IndentsWorkspaceSource{Lines: []domain.IndentsRequirement{}, Members: []domain.IndentsCandidate{}}
	p, e := c.p.Projects.Get(ctx, project, false)
	if e != nil {
		return out, e
	}
	if p == nil {
		return out, domain.IndentsMissing
	}
	out.Currency = p.Currency
	rows, e := c.p.Demands.List(ctx, project)
	if e != nil {
		return out, e
	}
	for _, row := range rows {
		source, e := c.Line(ctx, project, row.ID)
		if e != nil {
			return out, e
		}
		used, left, e := domain.IndentsRemaining(source.Quantity, records, row.ID)
		if e != nil {
			return out, e
		}
		out.Lines = append(out.Lines, domain.IndentsRequirement{IndentsSource: source, ID: row.ID, ReleasedQuantity: used, RemainingQuantity: left})
	}
	if allCandidates {
		users, e := c.p.Users.List(ctx)
		if e != nil {
			return out, e
		}
		for _, user := range users {
			if user.Active {
				out.Members = append(out.Members, domain.IndentsCandidate{ID: user.ID, Label: user.Name})
			}
		}
	} else {
		seen := map[string]bool{}
		for _, record := range records {
			for _, id := range []string{record.Data.DoerID, record.Data.ResolverID} {
				if seen[id] {
					continue
				}
				seen[id] = true
				user, e := c.p.Users.Get(ctx, id)
				if e != nil {
					return out, e
				}
				if user != nil {
					out.Members = append(out.Members, domain.IndentsCandidate{ID: user.ID, Label: user.Name})
				}
			}
		}
	}
	return out, nil
}
