package application

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

var ErrRecoveryUnavailable = errors.New("email recovery unavailable")
var ErrRecoveryDelivery = errors.New("email recovery delivery failed")

// RecoveryService owns proof-of-email account recovery, separately from account edits.
type RecoveryService struct {
	unit      ports.UnitOfWork[ports.RecoveryRepository]
	passwords ports.Passwords
	tokens    ports.Tokens
	mail      ports.RecoveryMail
}

func NewRecovery(u ports.UnitOfWork[ports.RecoveryRepository], p ports.Passwords, t ports.Tokens, m ports.RecoveryMail) *RecoveryService {
	return &RecoveryService{u, p, t, m}
}

func (s *RecoveryService) Request(ctx context.Context, email string) error {
	if !s.mail.Ready() {
		return ErrRecoveryUnavailable
	}
	raw, hash := s.tokens.Refresh()
	recipient := ""
	err := s.unit.Within(ctx, func(ctx context.Context, r ports.RecoveryRepository) error {
		found, err := r.FindEmail(ctx, domain.RecoveryEmail(email))
		if err != nil || found == nil {
			return err
		}
		user, err := r.User(ctx, found.ID, true)
		if err != nil || user == nil {
			return err
		}
		if !user.IsActive || domain.RecoveryEmail(user.Email) != domain.RecoveryEmail(email) {
			return nil
		}
		recent, err := r.RecentlyRequested(ctx, user.ID)
		if err != nil || recent {
			return err
		}
		if err = r.ConsumeRecoveries(ctx, user.ID); err != nil {
			return err
		}
		if err = r.CreateRecovery(ctx, hash, user.ID, s.tokens.Now()+900); err != nil {
			return err
		}
		recipient = user.Email
		return nil
	})
	if err != nil {
		return err
	}
	if recipient != "" {
		// The record commits before delivery. Never send a link for a rolled-back record.
		if s.mail.SendRecovery(ctx, recipient, raw) != nil {
			return ErrRecoveryDelivery
		}
	}
	return nil
}

func (s *RecoveryService) Complete(ctx context.Context, token, password string) error {
	invalid := &domain.Error{Kind: domain.Invalid, Detail: "This reset link is invalid or expired. Request a new link."}
	if len(token) != 64 {
		return invalid
	}
	if !domain.StrongPassword(password) {
		return &domain.Error{Kind: domain.Invalid, Detail: "Use 8 to 128 characters including uppercase, lowercase, number and symbol."}
	}
	return s.unit.Within(ctx, func(ctx context.Context, r ports.RecoveryRepository) error {
		hash := s.tokens.HashRefresh(token)
		id, err := r.RecoveryUserID(ctx, hash)
		if err != nil {
			return err
		}
		if id == "" {
			return invalid
		}
		user, err := r.User(ctx, id, true)
		if err != nil {
			return err
		}
		if user == nil || !user.IsActive {
			return invalid
		}
		valid, err := r.ValidRecovery(ctx, hash)
		if err != nil {
			return err
		}
		if !valid {
			return invalid
		}
		if s.passwords.Verify(ctx, password, user.PasswordHash) {
			return &domain.Error{Kind: domain.Invalid, Detail: "Choose a different password."}
		}
		encoded, err := s.passwords.Hash(ctx, password)
		if err != nil {
			return err
		}
		if err = r.SetPassword(ctx, id, encoded, id); err != nil {
			return err
		}
		if err = r.ConsumeRecoveries(ctx, id); err != nil {
			return err
		}
		if err = r.RevokeUserSessions(ctx, id); err != nil {
			return err
		}
		return r.RecordAccountChange(ctx, s.tokens.ID(), id, "password_reset")
	})
}
