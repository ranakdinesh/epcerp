package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type MailCipher interface {
	Seal([]byte) ([]byte, string, error)
	Open([]byte, string) ([]byte, error)
}
type credentialRepository struct {
	*userRepository
	cipher MailCipher
}

func NewCredentialUnitOfWork(b database.Beginner, e func(database.Connection) events.Writer, c MailCipher) ports.UnitOfWork[ports.CredentialRepository] {
	return &unitOfWork[ports.CredentialRepository]{manager: database.NewManager(b), events: e, bind: func(s *Store) ports.CredentialRepository { return &credentialRepository{&userRepository{s}, c} }}
}
func (r *credentialRepository) SetTemporaryPassword(ctx context.Context, id, hash, actor string) error {
	return r.queries.SetTemporaryPassword(ctx, sqlc.SetTemporaryPasswordParams{ID: dtos.UUID(id), PasswordHash: hash, UpdatedByID: dtos.UUID(actor)})
}
func (r *credentialRepository) QueueCredentialEmail(ctx context.Context, m domain.CredentialEmail, version int64, actor string) error {
	body, e := json.Marshal(m)
	if e != nil {
		return e
	}
	encrypted, hash, e := r.cipher.Seal(body)
	if e != nil {
		return e
	}
	u, e := r.User(ctx, m.UserID, false)
	if e != nil {
		return e
	}
	if u == nil || u.TemporaryExpiresAt == nil {
		return errors.New("temporary credential expiry required")
	}
	if e = r.queries.CancelStaleCredentialEmails(ctx); e != nil {
		return e
	}
	return r.queries.QueueCredentialEmail(ctx, sqlc.QueueCredentialEmailParams{ID: dtos.UUID(m.ID), UserID: dtos.UUID(m.UserID), CredentialVersion: version, Payload: encrypted, PayloadHash: hash, ExpiresAt: dtos.Stamp(*u.TemporaryExpiresAt), CreatedByID: dtos.UUID(actor)})
}
func (r *credentialRepository) RecordCredentialChange(ctx context.Context, eventID, id, actor, action, reason string) error {
	if r.events == nil {
		return errors.New("credential audit writer required")
	}
	if e := r.queries.RecordCredentialChange(ctx, sqlc.RecordCredentialChangeParams{EventID: dtos.UUID(eventID), UserID: dtos.UUID(id), ActorID: dtos.UUID(actor), Action: action, Reason: reason}); e != nil {
		return e
	}
	v, e := r.queries.AdvanceAccountVersion(ctx, dtos.UUID(id))
	if e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: eventID, CorrelationID: eventID, Module: "auth", EntityType: "account", EntityID: id, EntityVersion: v.AccountVersion, Action: action, ActorID: actor, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.UpdatedAt.Time}, nil)
}

type credentialDeliveryRepository struct {
	*Store
	cipher MailCipher
}

func NewCredentialDeliveryUnitOfWork(b database.Beginner, c MailCipher) ports.UnitOfWork[ports.EmailDeliveryRepository] {
	return &unitOfWork[ports.EmailDeliveryRepository]{manager: database.NewManager(b), bind: func(s *Store) ports.EmailDeliveryRepository { return &credentialDeliveryRepository{s, c} }}
}
func (r *credentialDeliveryRepository) Claim(ctx context.Context, lease string) (domain.EmailJob, error) {
	if e := r.queries.CancelStaleCredentialEmails(ctx); e != nil {
		return domain.EmailJob{}, e
	}
	row, e := r.queries.ClaimCredentialEmail(ctx, dtos.UUID(lease))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.EmailJob{}, nil
	}
	if e != nil {
		return domain.EmailJob{}, e
	}
	job := domain.EmailJob{ID: row.ID.String(), LeaseID: lease}
	body, e := r.cipher.Open(row.Payload, row.PayloadHash)
	if e != nil {
		return job, nil
	}
	if json.Unmarshal(body, &job.Email) != nil || job.Email.ID != job.ID || job.Email.UserID != row.UserID.String() {
		return job, nil
	}
	user, e := r.User(ctx, row.UserID.String(), false)
	if e != nil {
		return job, e
	}
	job.Valid = user != nil && user.Email == job.Email.Recipient && user.IsActive && user.MustChangePassword && user.CredentialVersion == row.CredentialVersion
	if !job.Valid {
		e = r.queries.CancelCredentialEmail(ctx, sqlc.CancelCredentialEmailParams{ID: row.ID, LeaseID: dtos.UUID(lease)})
		return domain.EmailJob{}, e
	}
	return job, nil
}
func (r *credentialDeliveryRepository) Finish(ctx context.Context, id, lease string, delivered bool) error {
	return r.queries.FinishCredentialEmail(ctx, sqlc.FinishCredentialEmailParams{ID: dtos.UUID(id), LeaseID: dtos.UUID(lease), Delivered: delivered})
}
