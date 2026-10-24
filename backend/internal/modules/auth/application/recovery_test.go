package application

import (
	"context"
	"errors"
	"testing"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type recoveryRepo struct {
	ports.RecoveryRepository
	user                                          *domain.User
	valid, recent                                 bool
	created, consumed, passwords, revoked, events int
	auditError                                    error
}

func (r *recoveryRepo) FindEmail(context.Context, string) (*domain.User, error)  { return r.user, nil }
func (r *recoveryRepo) User(context.Context, string, bool) (*domain.User, error) { return r.user, nil }
func (r *recoveryRepo) RecoveryUserID(context.Context, string) (string, error)   { return "user", nil }
func (r *recoveryRepo) ValidRecovery(context.Context, string) (bool, error)      { return r.valid, nil }
func (r *recoveryRepo) RecentlyRequested(context.Context, string) (bool, error)  { return r.recent, nil }
func (r *recoveryRepo) CreateRecovery(_ context.Context, hash, id string, expiry int64) error {
	r.created++
	if hash == "rrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr" || expiry != 1000 {
		return errors.New("invalid persisted token")
	}
	return nil
}
func (r *recoveryRepo) ConsumeRecoveries(context.Context, string) error { r.consumed++; return nil }
func (r *recoveryRepo) SetPassword(context.Context, string, string, string) error {
	r.passwords++
	return nil
}
func (r *recoveryRepo) RevokeUserSessions(context.Context, string) error { r.revoked++; return nil }
func (r *recoveryRepo) RecordAccountChange(_ context.Context, _ string, _ string, action string) error {
	if action != "password_reset" {
		return errors.New("wrong audit")
	}
	r.events++
	return r.auditError
}

type recoveryPasswords struct{ ports.Passwords }

func (recoveryPasswords) Hash(context.Context, string) (string, error) { return "encoded", nil }
func (recoveryPasswords) Verify(context.Context, string, string) bool  { return false }

type recoveryMail struct {
	enabled bool
	sent    int
	err     error
}

func (m *recoveryMail) Ready() bool                                        { return m.enabled }
func (m *recoveryMail) SendRecovery(context.Context, string, string) error { m.sent++; return m.err }

type recoveryTokens struct{ fakeTokens }

func (recoveryTokens) Refresh() (string, string) {
	return "rrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}
func TestRecoveryRequestPrivacyAndCooldown(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name   string
		user   *domain.User
		recent bool
		sent   int
	}{
		{"unknown", nil, false, 0}, {"inactive", &domain.User{Email: "person@example.invalid"}, false, 0},
		{"throttled", &domain.User{ID: "user", Email: "person@example.invalid", IsActive: true}, true, 0},
		{"active", &domain.User{ID: "user", Email: "person@example.invalid", IsActive: true}, false, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := &recoveryRepo{user: test.user, recent: test.recent}
			m := &recoveryMail{enabled: true}
			u := &fakeUnit[ports.RecoveryRepository]{repo: r}
			s := NewRecovery(u, recoveryPasswords{}, recoveryTokens{fakeTokens{now: 100}}, m)
			if err := s.Request(ctx, "person@example.invalid"); err != nil {
				t.Fatal(err)
			}
			if m.sent != test.sent || r.created != test.sent || u.commits != 1 {
				t.Fatal("privacy/cooldown changed")
			}
		})
	}
}
func TestRecoveryCompletionDenialsAndAtomicAudit(t *testing.T) {
	ctx := context.Background()
	raw := "rrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr"
	for _, valid := range []bool{false, true} {
		r := &recoveryRepo{user: &domain.User{ID: "user", IsActive: true}, valid: valid}
		u := &fakeUnit[ports.RecoveryRepository]{repo: r}
		s := NewRecovery(u, recoveryPasswords{}, fakeTokens{}, &recoveryMail{})
		err := s.Complete(ctx, raw, "New-Strong-Password9!")
		if valid {
			if err != nil || r.consumed != 1 || r.passwords != 1 || r.revoked != 1 || r.events != 1 || u.commits != 1 {
				t.Fatal("reset not atomic", err)
			}
		} else if err == nil || r.passwords != 0 {
			t.Fatal("invalid/expired/replayed token changed password")
		}
	}
	r := &recoveryRepo{user: &domain.User{ID: "user", IsActive: true}, valid: true, auditError: errors.New("audit unavailable")}
	u := &fakeUnit[ports.RecoveryRepository]{repo: r}
	s := NewRecovery(u, recoveryPasswords{}, fakeTokens{}, &recoveryMail{})
	if s.Complete(ctx, raw, "New-Strong-Password9!") == nil || u.rollbacks != 1 || u.commits != 0 {
		t.Fatal("audit failure did not rollback")
	}
	r.user.IsActive = false
	before := r.passwords
	if s.Complete(ctx, raw, "New-Strong-Password9!") == nil || r.passwords != before {
		t.Fatal("inactive recovery accepted")
	}
	if s.Complete(ctx, raw, "weak") == nil || s.Complete(ctx, "bad", "New-Strong-Password9!") == nil {
		t.Fatal("weak password or malformed token accepted")
	}
}
func TestRecoveryDisabledAndProviderFailure(t *testing.T) {
	m := &recoveryMail{}
	r := &recoveryRepo{user: &domain.User{ID: "user", Email: "person@example.invalid", IsActive: true}}
	u := &fakeUnit[ports.RecoveryRepository]{repo: r}
	s := NewRecovery(u, nil, recoveryTokens{fakeTokens{now: 100}}, m)
	if !errors.Is(s.Request(context.Background(), r.user.Email), ErrRecoveryUnavailable) || u.commits != 0 {
		t.Fatal("disabled mail pretended to send")
	}
	m.enabled = true
	m.err = errors.New("provider error")
	if !errors.Is(s.Request(context.Background(), r.user.Email), ErrRecoveryDelivery) || u.commits != 1 {
		t.Fatal("delivery must follow commit")
	}
}
