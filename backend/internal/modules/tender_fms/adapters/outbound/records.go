package outbound

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/identity"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

type recordsRepository struct {
	c       database.Connection
	q       *db.Queries
	journal events.Writer
}

func NewRecordsRepository(c database.Connection, j events.Writer) ports.RecordRepository {
	return &recordsRepository{c, db.New(c), j}
}
func (r *recordsRepository) LockIdentity(ctx context.Context) error {
	if !database.IsTransactionConnection(ctx, r.c) {
		return errors.New("Tender records require caller transaction")
	}
	return r.q.LockTenderIdentity(ctx)
}
func namedRecord(v db.TenderFmsIntake) domain.Record {
	return domain.Record{ID: v.ID.String(), Name: v.Title, SalesCreatorID: v.SalesCreatorID.String(), ManagerID: v.OwnerID.String(), ManagerAssignmentID: v.ManagerAssignmentID.String(), BusinessVersion: v.BusinessVersion, ContentRevision: v.ContentRevision, AssignmentVersion: v.AssignmentVersion, Active: v.IsActive, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: actor(v.CreatedByID), UpdatedBy: actor(v.UpdatedByID)}
}
func (r *recordsRepository) GetNamed(ctx context.Context, id string) (domain.Record, error) {
	v, e := r.q.LockNamedTender(ctx, uuid(id))
	if errors.Is(e, pgx.ErrNoRows) || (e == nil && v.RecordKind != "named") {
		return domain.Record{}, domain.Missing
	}
	if e != nil {
		return domain.Record{}, e
	}
	return namedRecord(v), nil
}
func (r *recordsRepository) CreateNamed(ctx context.Context, in domain.NamedInput, actorID, id, assignment, event string) (domain.Record, error) {
	var out domain.Record
	if r.journal == nil {
		return out, errors.New("Tender business journal required")
	}
	v, e := r.q.InsertNamedTender(ctx, db.InsertNamedTenderParams{ID: uuid(id), Title: in.Name, ManagerID: uuid(in.ManagerID), ActorID: uuid(actorID), AssignmentID: uuid(assignment)})
	if e != nil {
		return out, e
	}
	if e = r.q.InsertNamedManagerAssignment(ctx, db.InsertNamedManagerAssignmentParams{ID: uuid(assignment), IntakeID: uuid(id), ManagerID: uuid(in.ManagerID), ActorID: uuid(actorID)}); e != nil {
		return out, e
	}
	if e = r.q.InsertNamedRecordEvent(ctx, db.InsertNamedRecordEventParams{ID: uuid(event), IntakeID: uuid(id), ActorID: uuid(actorID)}); e != nil {
		return out, e
	}
	if e = r.journal.Record(ctx, events.Event{ID: event, CorrelationID: event, Module: "tender_fms", EntityType: "tender_record", EntityID: id, EntityVersion: 1, Action: "named_record_created", ActorID: actorID, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.CreatedAt.Time}, nil); e != nil {
		return out, e
	}
	return namedRecord(v), nil
}

// Eligibility is trusted administrator-reviewed deployment configuration of stable
// Auth bundle IDs. Empty config produces no candidates; names never confer authority.
type eligibleUsers struct {
	users    auth.AssignmentDirectory
	bundles  auth.BundleDirectory
	approved []string
}

func NewEligibleUsers(u auth.AssignmentDirectory, b auth.BundleDirectory, ids []string) ports.EligibleUsers {
	return &eligibleUsers{u, b, append([]string{}, ids...)}
}
func (u *eligibleUsers) Get(ctx context.Context, id string) (*domain.EligibleIdentity, error) {
	v, e := u.users.Get(ctx, id)
	if e != nil || v == nil {
		return nil, e
	}
	out := &domain.EligibleIdentity{Principal: identity.Principal{ID: v.ID, Active: v.Active, Superuser: v.Superuser, Permissions: v.Permissions}, Ready: v.Ready, DisplayName: v.Name}
	for _, bundle := range u.approved {
		b, e := u.bundles.Get(ctx, bundle)
		if e != nil {
			return nil, e
		}
		if b == nil || !b.Active {
			continue
		}
		member, e := u.bundles.Member(ctx, id, bundle)
		if e != nil {
			return nil, e
		}
		if member {
			out.Designated = true
			break
		}
	}
	return out, nil
}
func (u *eligibleUsers) List(ctx context.Context) ([]domain.Candidate, error) {
	if len(u.approved) == 0 {
		return []domain.Candidate{}, nil
	}
	rows, e := u.users.List(ctx)
	if e != nil {
		return nil, e
	}
	if len(rows) > 10000 {
		return nil, errors.New("Tender candidate directory exceeds reviewed limit")
	}
	out := []domain.Candidate{}
	for _, v := range rows {
		if v.Active && v.Ready {
			out = append(out, domain.Candidate{ID: v.ID, DisplayName: v.Name})
		}
	}
	return out, nil
}

func (r *recordsRepository) ParticipantActive(ctx context.Context, id, user string) (bool, error) {
	return r.q.NamedParticipantActive(ctx, db.NamedParticipantActiveParams{IntakeID: uuid(id), UserID: uuid(user)})
}
func (r *recordsRepository) SaveParticipant(ctx context.Context, id, actor, event string, in domain.ParticipantGrant) (int64, error) {
	if r.journal == nil {
		return 0, errors.New("Tender business journal required")
	}
	// Source row lock serializes explicit scope changes. No grant can be effective
	// while a concurrent manager transfer is evaluated against an unlocked source.
	v, e := r.q.LockNamedTender(ctx, uuid(id))
	if e != nil {
		return 0, e
	}
	if v.RecordKind != "named" || !v.IsActive {
		return 0, domain.Missing
	}
	version, e := r.q.CurrentNamedGrantVersion(ctx, db.CurrentNamedGrantVersionParams{IntakeID: uuid(id), UserID: uuid(in.UserID)})
	if e != nil {
		return 0, e
	}
	if version != in.ExpectedVersion {
		return 0, &domain.Error{Status: 409, Detail: "Participant scope changed; refresh before saving"}
	}
	until := pgtype.Timestamptz{}
	if in.ValidUntil != nil {
		until = pgtype.Timestamptz{Time: *in.ValidUntil, Valid: true}
	}
	at, e := r.q.RecordNamedGrant(ctx, db.RecordNamedGrantParams{ID: uuid(event), IntakeID: uuid(id), UserID: uuid(in.UserID), Version: version + 1, IsActive: in.Active, ValidUntil: until, Reason: in.Reason, ActorID: uuid(actor)})
	if e != nil {
		return 0, e
	}
	if e = r.journal.Record(ctx, events.Event{ID: event, CorrelationID: event, Module: "tender_fms", EntityType: "tender_participant", EntityID: id, EntityVersion: version + 1, Action: "metadata_scope_changed", ActorID: actor, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: at.Time}, nil); e != nil {
		return 0, e
	}
	return version + 1, nil
}
