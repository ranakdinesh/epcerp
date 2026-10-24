package outbound

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

type ownershipUnit struct {
	manager *database.Manager
	users   func(database.Connection) auth.AssignmentDirectory
	journal func(database.Connection) events.Writer
}

func NewOwnershipUnit(b database.Beginner, u func(database.Connection) auth.AssignmentDirectory, j func(database.Connection) events.Writer) ports.OwnershipUnit {
	return &ownershipUnit{database.NewManager(b), u, j}
}
func (u *ownershipUnit) Within(ctx context.Context, f func(context.Context, ports.OwnershipRepository, ports.OwnershipUsers) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &ownershipRepository{&repository{q: db.New(c), e: u.journal(c)}}, &ownershipUsers{u.users(c)})
	})
}

type ownershipUsers struct{ users auth.AssignmentDirectory }

func (u *ownershipUsers) Get(ctx context.Context, id string) (*domain.Principal, error) {
	p, e := u.users.Get(ctx, id)
	if e != nil || p == nil {
		return nil, e
	}
	return &domain.Principal{ID: p.ID, Active: p.Active, Superuser: p.Superuser, Permissions: p.Permissions}, nil
}
func (u *ownershipUsers) List(ctx context.Context) ([]domain.OwnershipCandidate, error) {
	rows, e := u.users.List(ctx)
	out := []domain.OwnershipCandidate{}
	for _, p := range rows {
		if p.Active {
			out = append(out, domain.OwnershipCandidate{ID: p.ID, Name: p.Name})
		}
	}
	return out, e
}

type ownershipRepository struct{ *repository }

func (r *ownershipRepository) SaveOwner(ctx context.Context, id string, in domain.OwnershipInput, actor, event string) (domain.Intake, error) {
	prior, e := r.Get(ctx, id)
	if e != nil {
		return prior, e
	}
	row, e := r.q.HandoverTenderOwner(ctx, db.HandoverTenderOwnerParams{ID: uuid(id), OwnerID: uuid(in.OwnerID), UpdatedByID: uuid(actor), Version: in.Version})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Intake{}, &domain.Error{Status: 409, Detail: "Tender ownership changed; refresh before handing over"}
	}
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" {
		return domain.Intake{}, &domain.Error{Status: 409, Detail: "The recipient already owns an intake of the same source; reconcile it before handover"}
	}
	if e != nil {
		return domain.Intake{}, e
	}
	if e = r.q.RecordTenderOwnerChange(ctx, db.RecordTenderOwnerChangeParams{ID: uuid(event), IntakeID: uuid(id), PreviousOwnerID: uuid(prior.OwnerID), OwnerID: uuid(in.OwnerID), Version: row.Version, Reason: in.Reason, CreatedByID: uuid(actor), CreatedAt: pgtype.Timestamptz{Time: row.UpdatedAt.Time, Valid: true}}); e != nil {
		return domain.Intake{}, e
	}
	if e = r.e.Record(ctx, events.Event{ID: event, CorrelationID: event, Module: "tender_fms", EntityType: "tender_intake", EntityID: id, EntityVersion: int64(row.Version), Action: "owner_handover", ActorID: actor, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: row.UpdatedAt.Time}, nil); e != nil {
		return domain.Intake{}, e
	}
	return record(row), nil
}
