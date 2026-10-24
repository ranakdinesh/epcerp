package integration

import (
	"context"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type captureRecoveryMail struct {
	raw   string
	count int
}

func (m *captureRecoveryMail) Ready() bool { return true }
func (m *captureRecoveryMail) SendRecovery(_ context.Context, _ string, raw string) error {
	m.raw = raw
	m.count++
	return nil
}

func TestPasswordRecoveryOwnedDatabase(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	tokens, e := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	passwords, e := security.NewPasswords()
	if e != nil {
		t.Fatal(e)
	}
	id := tokens.ID()
	original := "Original-Strong-Password8!"
	hash, e := passwords.Hash(ctx, original)
	if e != nil {
		t.Fatal(e)
	}
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'recovery_user','recovery@example.invalid',$2,true,false)", id, hash)
	execFixture(t, pool, "INSERT INTO auth.refresh_tokens(id,user_id,token_hash,family_id,expires_at) VALUES($1,$2,$3,$4,now()+interval '1 day')", tokens.ID(), id, tokens.HashRefresh(rand.Text()), tokens.ID())
	unit := authout.NewRecoveryUnitOfWork(pool, func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) })
	mail := &captureRecoveryMail{}
	service := authapp.NewRecovery(unit, passwords, tokens, mail)
	if e = service.Request(ctx, " RECOVERY@example.invalid "); e != nil || mail.count != 1 || len(mail.raw) != 64 {
		t.Fatal("request did not deliver after persistence", e)
	}
	first := mail.raw
	if e = service.Request(ctx, "recovery@example.invalid"); e != nil || mail.count != 1 {
		t.Fatal("account cooldown failed", e)
	}
	var persisted string
	if e = pool.QueryRow(ctx, "SELECT token_hash FROM auth.auth_password_recovery WHERE user_id=$1", id).Scan(&persisted); e != nil || persisted == first || persisted != tokens.HashRefresh(first) {
		t.Fatal("raw token persisted", e)
	}
	execFixture(t, pool, "UPDATE auth.auth_password_recovery SET created_at=now()-interval '20 minutes',expires_at=now()-interval '5 minutes' WHERE user_id=$1", id)
	if service.Complete(ctx, first, "Abcd123!") == nil {
		t.Fatal("expired link accepted")
	}
	if e = service.Request(ctx, "recovery@example.invalid"); e != nil || mail.count != 2 {
		t.Fatal(e)
	}
	second := mail.raw
	execFixture(t, pool, "UPDATE auth.users SET email='changed@example.invalid',updated_at=clock_timestamp() WHERE id=$1", id)
	if service.Complete(ctx, second, "Abcd123!") == nil {
		t.Fatal("link survived email change")
	}
	execFixture(t, pool, "UPDATE auth.auth_password_recovery SET created_at=now()-interval '2 minutes' WHERE user_id=$1 AND expires_at>now()", id)
	if e = service.Request(ctx, "changed@example.invalid"); e != nil || mail.count != 3 {
		t.Fatal(e)
	}
	broken := authapp.NewRecovery(authout.NewRecoveryUnitOfWork(pool, nil), passwords, tokens, mail)
	if broken.Complete(ctx, mail.raw, "Abcd123!") == nil {
		t.Fatal("audit failure accepted")
	}
	var current string
	var consumed bool
	if e = pool.QueryRow(ctx, "SELECT password_hash FROM auth.users WHERE id=$1", id).Scan(&current); e != nil || current != hash {
		t.Fatal("password escaped rollback", e)
	}
	if e = pool.QueryRow(ctx, "SELECT consumed_at IS NOT NULL FROM auth.auth_password_recovery WHERE token_hash=$1", tokens.HashRefresh(mail.raw)).Scan(&consumed); e != nil || consumed {
		t.Fatal("consumption escaped rollback", e)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- service.Complete(ctx, mail.raw, "Abcd123!") }()
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("reset link not single use", success)
	}
	if service.Complete(ctx, mail.raw, "Another-Password8!") == nil {
		t.Fatal("replay accepted")
	}
	if e = pool.QueryRow(ctx, "SELECT password_hash FROM auth.users WHERE id=$1", id).Scan(&current); e != nil || !passwords.Verify(ctx, "Abcd123!", current) {
		t.Fatal("new password not stored", e)
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.refresh_tokens WHERE user_id=$1 AND revoked_at IS NULL", id).Scan(&count); e != nil || count != 0 {
		t.Fatal("refresh session survived", e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE entity_id=$1 AND action='password_reset' AND actor_id=$1", id).Scan(&count); e != nil || count != 1 {
		t.Fatal("immutable reset event missing", e)
	}
	execFixture(t, pool, "UPDATE auth.auth_password_recovery SET created_at=now()-interval '2 minutes' WHERE user_id=$1 AND expires_at>now()", id)
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", id)
	if e = service.Request(ctx, "changed@example.invalid"); e != nil || mail.count != 3 {
		t.Fatal("inactive account received mail", e)
	}
}
