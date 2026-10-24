package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type quotationsSourceRepository struct {
	ports.QuotationsRepository
	rows []domain.QuotationsQuote
	err  error
}

func (r quotationsSourceRepository) Revisions(context.Context, string, string) ([]domain.QuotationsQuote, error) {
	return r.rows, r.err
}

type quotationsSourceContext struct {
	ports.QuotationsContext
	value domain.QuotationsScope
	err   error
}

func (c quotationsSourceContext) Source(context.Context, string, string) (domain.QuotationsScope, error) {
	return c.value, c.err
}

type quotationsSourceGroups struct {
	rows []domain.QuotationsApprovalGroup
	err  error
}

func (g quotationsSourceGroups) Current(context.Context, string, string) ([]domain.QuotationsApprovalGroup, error) {
	return g.rows, g.err
}

func TestQuotationsApprovalSourceRetainsUnquotedGroupsAndUnavailableScope(t *testing.T) {
	project := "00000000-0000-0000-0000-000000000001"
	bo := "00000000-0000-0000-0000-000000000002"
	groups := []domain.QuotationsApprovalGroup{{ID: "unquoted", Name: "Needs vendor prices", ScopeRevision: 2}}
	out, e := QuotationsApprovalSource(context.Background(), quotationsSourceRepository{}, quotationsSourceContext{value: domain.QuotationsScope{Currency: "INR", Available: false}}, quotationsSourceGroups{rows: groups}, domain.QuotationsTimestamp{}, project, bo)
	if e != nil || out.Available || out.ProjectID != project || out.BOID != bo || out.Currency != "INR" || (len(out.Groups) != 1 || out.Groups[0] != groups[0]) || len(out.Quotes) != 0 {
		t.Fatal("An unquoted group or unavailable scope was hidden", e)
	}
}

func TestQuotationsApprovalSourceDoesNotReturnPartialEvidenceOnProviderFailure(t *testing.T) {
	project := "00000000-0000-0000-0000-000000000001"
	bo := "00000000-0000-0000-0000-000000000002"
	failure := errors.New("source unavailable")
	for _, stage := range []string{"scope", "groups", "revisions"} {
		t.Run(stage, func(t *testing.T) {
			r, c, g := quotationsSourceRepository{}, quotationsSourceContext{value: domain.QuotationsScope{Currency: "INR", Available: true}}, quotationsSourceGroups{rows: []domain.QuotationsApprovalGroup{{ID: "unquoted"}}}
			switch stage {
			case "scope":
				c.err = failure
			case "groups":
				g.err = failure
			case "revisions":
				r.err = failure
			}
			out, e := QuotationsApprovalSource(context.Background(), r, c, g, domain.QuotationsTimestamp{}, project, bo)
			if !errors.Is(e, failure) || (out.ProjectID != "" || out.BOID != "" || out.Currency != "" || out.Available || out.Groups != nil || out.Quotes != nil) {
				t.Fatal("Partial Finance source escaped a provider failure", e)
			}
		})
	}
}
