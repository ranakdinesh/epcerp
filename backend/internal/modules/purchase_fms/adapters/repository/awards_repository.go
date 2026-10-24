package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type AwardsProviders struct {
	Indents   IndentsProviders
	Workflow  WorkflowProviders
	Sourcing  SourcingProviders
	Approvals QuotationApprovalsProviders
}
type awardsUnitOfWork struct {
	m *database.Manager
	p func(database.Connection) AwardsProviders
	j func(database.Connection) events.Writer
}

func NewAwardsUnitOfWork(b database.Beginner, p func(database.Connection) AwardsProviders, j func(database.Connection) events.Writer) *awardsUnitOfWork {
	return &awardsUnitOfWork{database.NewManager(b), p, j}
}
func (u *awardsUnitOfWork) WithinAwards(ctx context.Context, f func(context.Context, ports.AwardsSession) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		p := u.p(c)
		q := NewStore(c)
		j := u.j(c)
		return f(ctx, ports.AwardsSession{Documents: &loiDocumentsRepository{q, j}, DocumentContext: &awardsContext{c, p}, Awards: &awardsRepository{q, j}, Context: &awardsContext{c, p}, Indents: &indentsRepository{q, j}, IndentContext: &indentsIndentContext{p.Indents}, Workflow: &workflowRepository{q, j}, WorkContext: &workflowSourceContext{p.Workflow}, Sourcing: &sourcingRepository{q, j}, SourceContext: NewSourcingContext(p.Sourcing)})
	})
}

type awardsContext struct {
	c database.Connection
	p AwardsProviders
}

func (c *awardsContext) Authorize(ctx context.Context, a domain.Principal, p string, write bool) error {
	code := "projects.purchase_sourcing.read"
	if write {
		code = "projects.purchase_awards.write"
	}
	user, e := c.p.Indents.Users.Get(ctx, a.ID)
	if e != nil {
		return e
	}
	if user == nil || !user.Active || !user.Allows(code) || !a.Can(code) {
		return domain.WorkflowForbidden
	}
	return (&indentsIndentContext{c.p.Indents}).Scope(ctx, a, p, code, write)
}
func (c *awardsContext) Candidates(ctx context.Context, p string) ([]domain.AwardCandidate, error) {
	bos, e := NewBOReader(c.c).List(ctx, p)
	if e != nil {
		return nil, e
	}
	approved := NewQuotationApprovalsApprovedReader(c.c, c.p.Approvals).(contracts.QuotationGroupApprovedReader)
	out := []domain.AwardCandidate{}
	for _, bo := range bos {
		if !bo.Active || bo.State != "completed" {
			continue
		}
		rows, e := approved.ApprovedGroups(ctx, p, bo.ID)
		if e != nil {
			var d *domain.Error
			if errors.As(e, &d) && (d.Status == 422 || d.Status == 403) {
				continue
			}
			return nil, e
		}
		prices, e := approved.CommercialGroups(ctx, p, bo.ID)
		if e != nil {
			return nil, e
		}
		quotes, e := c.p.Approvals.Quotes.Source(ctx, p, bo.ID)
		if e != nil {
			return nil, e
		}
		for _, a := range rows {
			candidate := domain.AwardCandidate{BOID: bo.ID, Attempt: a, Lines: []domain.AwardLine{}}
			for _, l := range a.Lines {
				line := domain.AwardLine{Approval: l}
				for _, b := range bo.Lines {
					if b.ID == l.LineID && b.DemandID != nil && b.ItemID != nil && *b.ItemID == l.Item.ID {
						line.DemandID = *b.DemandID
					}
				}
				if line.DemandID == "" {
					return nil, domain.WorkflowInvalid("source", "Approved item has no current published requirement")
				}
				found := false
				for _, price := range prices {
					if price.AttemptID == a.ID && price.LineID == l.LineID && price.ItemID == l.Item.ID {
						line.Price = price.Price
						found = true
					}
				}
				if !found {
					return nil, domain.WorkflowInvalid("price", "Current approved item pricing is unavailable")
				}
				for _, q := range quotes.Quotes {
					if q.DocumentID == a.DocumentID && q.Revision == a.Revision && q.VendorID == a.VendorID {
						line.DeliveryDays = int(q.DeliveryDays)
						line.PaymentTerms = q.PaymentTerms
						for _, ql := range q.Lines {
							if ql.LineID == l.LineID {
								line.Warranty = ql.CoverageType
								if ql.WarrantyMonths != nil {
									line.Warranty = fmt.Sprintf("%s · %d months", line.Warranty, *ql.WarrantyMonths)
								}
								if ql.Notes != "" {
									line.Warranty += " · " + ql.Notes
								}
							}
						}
					}
				}
				candidate.Lines = append(candidate.Lines, line)
			}
			out = append(out, candidate)
		}
	}
	return out, nil
}

type awardsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *awardsRepository) Lock(ctx context.Context, p string) error { return r.q.AwardsLock(ctx, p) }
func (r *awardsRepository) Get(ctx context.Context, p, attempt string) (*domain.AwardHandoff, error) {
	v, e := r.q.AwardsGet(ctx, sqlc.AwardsGetParams{ProjectID: dtos.UUID(p), AttemptID: dtos.UUID(attempt)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out := domain.AwardHandoff{ID: v.ID.String(), ProjectID: v.ProjectID.String(), BOID: v.BoID.String(), AttemptID: v.AttemptID.String(), CreatedBy: v.CreatedByID.String(), CreatedAt: v.CreatedAt.Time}
	if e = json.Unmarshal(v.Packages, &out.Packages); e != nil {
		return nil, e
	}
	return &out, nil
}
func (r *awardsRepository) Save(ctx context.Context, v domain.AwardHandoff, reason string) error {
	packages, e := json.Marshal(v.Packages)
	if e != nil {
		return e
	}
	if e = r.q.AwardsInsert(ctx, sqlc.AwardsInsertParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), BoID: dtos.UUID(v.BOID), AttemptID: dtos.UUID(v.AttemptID), Packages: packages, Reason: reason, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "purchase_awards", EntityType: "award_handoff", EntityID: v.ID, EntityVersion: 1, Action: "created", ActorID: v.CreatedBy, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
