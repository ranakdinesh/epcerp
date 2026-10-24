package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/ports"
)

type Journal struct {
	connection database.Connection
	q          *db.Queries
}

func NewJournal(c database.Connection) *Journal { return &Journal{c, db.New(c)} }

var _ ports.Journal = (*Journal)(nil)

func uuid(value string) pgtype.UUID {
	var id pgtype.UUID
	if value != "" {
		_ = id.Scan(value)
	}
	return id
}
func optional(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return id.String()
}
func fromRow(row db.BusinessEvent) domain.Event {
	return domain.Event{ID: row.ID.String(), Module: row.SourceModule, EntityType: row.EntityType, EntityID: row.EntityID.String(), EntityVersion: row.EntityVersion, Action: row.Action, ActorID: optional(row.ActorID), ActorKind: row.ActorKind, CorrelationID: row.CorrelationID.String(), CausationID: optional(row.CausationID), ScopeKind: row.ScopeKind, ScopeID: optional(row.ScopeID), Visibility: row.Visibility, OccurredAt: row.OccurredAt.Time.UTC()}
}
func conflict(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && (pg.Code == "23505" || pg.Code == "23503") {
		return domain.ErrConflict
	}
	return err
}
func (j *Journal) Append(ctx context.Context, e domain.Event, targets []domain.Target) error {
	if !database.IsTransactionConnection(ctx, j.connection) {
		return domain.ErrTransactionRequired
	}
	clean, err := e.Normalize()
	if err != nil {
		return err
	}
	if err = domain.ValidateTargets(targets); err != nil {
		return err
	}
	count, err := j.q.InsertEvent(ctx, db.InsertEventParams{ID: uuid(clean.ID), SourceModule: clean.Module, EntityType: clean.EntityType, EntityID: uuid(clean.EntityID), EntityVersion: clean.EntityVersion, DeliveryCount: int32(len(targets)), Action: clean.Action, ActorID: uuid(clean.ActorID), ActorKind: clean.ActorKind, CorrelationID: uuid(clean.CorrelationID), CausationID: uuid(clean.CausationID), ScopeKind: clean.ScopeKind, ScopeID: uuid(clean.ScopeID), Visibility: clean.Visibility, OccurredAt: pgtype.Timestamptz{Time: clean.OccurredAt, Valid: true}})
	if err != nil {
		return conflict(err)
	}
	if count == 0 {
		existing, err := j.q.GetEvent(ctx, uuid(clean.ID))
		if err != nil {
			return err
		}
		if !domain.Equal(fromRow(existing), clean) || int(existing.DeliveryCount) != len(targets) {
			return domain.ErrConflict
		}
	}
	for _, target := range targets {
		if count == 0 {
			attempts, err := j.q.DeliveryPolicy(ctx, db.DeliveryPolicyParams{EventID: uuid(clean.ID), Consumer: target.Consumer})
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrConflict
			}
			if err != nil {
				return err
			}
			if attempts != target.MaxAttempts {
				return domain.ErrConflict
			}
			continue
		}
		if err = j.q.InsertDelivery(ctx, db.InsertDeliveryParams{ID: uuid(SystemValues{}.ID()), EventID: uuid(clean.ID), Consumer: target.Consumer, MaxAttempts: target.MaxAttempts}); err != nil {
			return err
		}
		attempts, err := j.q.DeliveryPolicy(ctx, db.DeliveryPolicyParams{EventID: uuid(clean.ID), Consumer: target.Consumer})
		if err != nil {
			return err
		}
		if attempts != target.MaxAttempts {
			return domain.ErrConflict
		}
	}
	return nil
}

type Queue struct{ q *db.Queries }

func NewQueue(c *pgxpool.Pool) *Queue { return &Queue{db.New(c)} }

var _ ports.Queue = (*Queue)(nil)

func (q *Queue) Claim(ctx context.Context, consumer, token string, seconds int32) (*domain.Claim, error) {
	if !domain.ValidCode(consumer) || !domain.ValidID(token) || seconds < 2 || seconds > 3600 {
		return nil, domain.ErrInvalid
	}
	row, err := q.q.ClaimDelivery(ctx, db.ClaimDeliveryParams{Consumer: consumer, LeaseToken: uuid(token), LeaseSeconds: seconds})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	event := fromRow(db.BusinessEvent{ID: row.ID, SourceModule: row.SourceModule, EntityType: row.EntityType, EntityID: row.EntityID, EntityVersion: row.EntityVersion, Action: row.Action, ActorID: row.ActorID, ActorKind: row.ActorKind, CorrelationID: row.CorrelationID, CausationID: row.CausationID, ScopeKind: row.ScopeKind, ScopeID: row.ScopeID, Visibility: row.Visibility, OccurredAt: row.OccurredAt, RecordedAt: row.RecordedAt})
	return &domain.Claim{ID: row.DeliveryID.String(), Consumer: row.Consumer, LeaseToken: row.LeaseToken.String(), Attempt: row.Attempts, Event: event}, nil
}
func validClaim(c domain.Claim) bool { return domain.ValidID(c.ID) && domain.ValidID(c.LeaseToken) }
func affected(count int64, err error) error {
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.ErrLeaseLost
	}
	return nil
}
func (q *Queue) Complete(ctx context.Context, c domain.Claim) error {
	if !validClaim(c) {
		return domain.ErrInvalid
	}
	count, err := q.q.CompleteDelivery(ctx, db.CompleteDeliveryParams{ID: uuid(c.ID), LeaseToken: uuid(c.LeaseToken)})
	return affected(count, err)
}
func (q *Queue) Fail(ctx context.Context, c domain.Claim, seconds int32) error {
	if !validClaim(c) || seconds < 1 || seconds > 86400 {
		return domain.ErrInvalid
	}
	count, err := q.q.FailDelivery(ctx, db.FailDeliveryParams{ID: uuid(c.ID), LeaseToken: uuid(c.LeaseToken), RetrySeconds: seconds})
	return affected(count, err)
}
func (q *Queue) ExpireExhausted(ctx context.Context, consumer string) (int64, error) {
	if !domain.ValidCode(consumer) {
		return 0, domain.ErrInvalid
	}
	return q.q.ExpireExhausted(ctx, consumer)
}

type SystemValues struct{}

func (SystemValues) ID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic("secure random unavailable")
	}
	bytes[6] = bytes[6]&15 | 64
	bytes[8] = bytes[8]&63 | 128
	value := hex.EncodeToString(bytes[:])
	return value[:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:]
}
