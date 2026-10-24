package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authidentity "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/identity"
	out "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	app "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventmodule "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type fixtureEmails struct {
	mu       sync.Mutex
	messages []string
	fail     bool
	enabled  bool
}

func (m *fixtureEmails) Ready() bool { return m.enabled }
func (m *fixtureEmails) Send(_ context.Context, _, _, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("fixture delivery unavailable")
	}
	m.messages = append(m.messages, message)
	return nil
}
func TestCredentialOnboardingAndForcedPasswordChange(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	values, e := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: 15 * time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	passwords, e := security.NewPasswords()
	if e != nil {
		t.Fatal(e)
	}
	_, e = app.NewBootstrap(out.NewBootstrapUnitOfWork(pool), passwords, values).Bootstrap(ctx, "onboarding_admin", "admin@example.invalid", "Aa9!"+rand.Text())
	if e != nil {
		t.Fatal(e)
	}
	var admin string
	if e = pool.QueryRow(ctx, "SELECT id::text FROM auth.users WHERE username='onboarding_admin'").Scan(&admin); e != nil {
		t.Fatal(e)
	}
	adminToken, _, e := values.Issue(admin, values.Now()+3600, 0)
	if e != nil {
		t.Fatal(e)
	}
	role := values.ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,'fixture_employee','Onboarding fixture',true)", role)
	execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id) SELECT $1,id FROM auth.permissions WHERE code='admin.users.read'", role)
	rawKey := make([]byte, 32)
	_, _ = rand.Read(rawKey)
	cipher, e := infrastructure.NewEmailCipher(hex.EncodeToString(rawKey))
	if e != nil {
		t.Fatal(e)
	}
	journal := func(c database.Connection) events.Writer { return eventmodule.New().Writer(c) }
	unit := out.NewCredentialUnitOfWork(pool, journal, cipher)
	mail := &fixtureEmails{enabled: true}
	credentials := app.NewCredentials(unit, passwords, values, security.TemporaryPasswords{}, mail, "https://erp.vishaltechnopower.in/login")
	request := domain.UserCreate{FirstName: "Fixture", LastName: "Employee", Mobile: "+91 9000000000", Username: "employee_fixture", Email: "employee@example.invalid", RoleNames: []string{"fixture_employee"}}
	created, e := credentials.Create(ctx, adminToken, request)
	if e != nil || !created.MustChangePassword || created.CredentialVersion != 1 {
		t.Fatal("onboarding not persisted", e)
	}
	var encrypted []byte
	if e = pool.QueryRow(ctx, "SELECT payload FROM auth.auth_credential_emails WHERE user_id=$1", created.ID).Scan(&encrypted); e != nil || bytes.Contains(encrypted, []byte("Temporary password")) || bytes.Contains(encrypted, []byte(request.Email)) {
		t.Fatal("queued secret not encrypted", e)
	}
	delivery := app.NewCredentialDelivery(out.NewCredentialDeliveryUnitOfWork(pool, cipher), mail, values)
	worked, e := delivery.Deliver(ctx)
	if e != nil || !worked || len(mail.messages) != 1 {
		t.Fatal("welcome email not delivered", e)
	}
	message := mail.messages[0]
	if !strings.Contains(message, "https://erp.vishaltechnopower.in/login") {
		t.Fatal("wrong canonical login domain")
	}
	temporary := strings.Split(strings.Split(message, "Temporary password: ")[1], "\n")[0]
	if !domain.StrongPassword(temporary) {
		t.Fatal("weak generated password")
	}
	var wiped bool
	if e = pool.QueryRow(ctx, "SELECT payload IS NULL AND state='delivered' FROM auth.auth_credential_emails WHERE user_id=$1", created.ID).Scan(&wiped); e != nil || !wiped {
		t.Fatal("delivered plaintext not erased", e)
	}
	sessions := app.NewSessions(out.NewSessionUnitOfWork(pool), passwords, values, domain.Policy{SessionLifetime: 12 * time.Hour})
	pair, e := sessions.Login(ctx, created.Username, temporary)
	if e != nil || !pair.MustChangePassword || pair.RefreshExpiresIn > 900 {
		t.Fatal("temporary session unrestricted", e)
	}
	self, e := sessions.Me(ctx, pair.AccessToken)
	if e != nil || !self.MustChangePassword || len(domain.EffectivePermissions(*self)) != 0 {
		t.Fatal("pending account permissions leaked", e)
	}
	users := app.NewUsers(out.NewUserUnitOfWork(pool), passwords, values, domain.Policy{})
	if _, e = users.Users(ctx, pair.AccessToken, domain.UserFilter{}); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("temporary identity reached administration", e)
	}
	identity, e := authidentity.NewIdentityResolver(sessions).Resolve(ctx, pair.AccessToken)
	if e != nil || identity.Active || identity.Superuser || len(identity.Permissions) != 0 {
		t.Fatal("temporary identity escaped cross-module bridge", e)
	}
	account := app.NewAccount(out.NewAccountUnitOfWork(pool, journal), passwords, values)
	changedEmail := "other@example.invalid"
	if e = account.Change(ctx, pair.AccessToken, temporary, &changedEmail, nil); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("temporary identity changed email", e)
	}
	permanent := "Abcd123!"
	if e = account.Change(ctx, pair.AccessToken, temporary, nil, &permanent); e != nil {
		t.Fatal("first password change failed", e)
	}
	if _, e = sessions.Me(ctx, pair.AccessToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("old access token survived password change", e)
	}
	if _, e = sessions.Refresh(ctx, pair.RefreshToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("old refresh survived", e)
	}
	if _, e = sessions.Login(ctx, created.Username, temporary); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("temporary password survived", e)
	}
	normal, e := sessions.Login(ctx, created.Username, permanent)
	if e != nil || normal.MustChangePassword {
		t.Fatal("personal password remained restricted", e)
	}
	if e = credentials.Reset(ctx, normal.AccessToken, admin, "An unprivileged actor cannot reset this"); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("unprivileged reset accepted", e)
	}
	// An administrator reset invalidates both access and refresh immediately.
	if e = credentials.Reset(ctx, adminToken, created.ID, "Employee requested access recovery"); e != nil {
		t.Fatal(e)
	}
	if _, e = sessions.Me(ctx, normal.AccessToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("reset did not invalidate access")
	}
	if e = credentials.Reset(ctx, adminToken, admin, "Testing own reset is not allowed"); e == nil {
		t.Fatal("self reset accepted")
	}
	if e = credentials.Reset(ctx, adminToken, created.ID, "short"); e == nil {
		t.Fatal("missing reason accepted")
	}
	// Issuing a newer reset cancels an older pending credential email.
	if e = credentials.Reset(ctx, adminToken, created.ID, "Issue a newer credential email after correction"); e != nil {
		t.Fatal(e)
	}
	var cancelled int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.auth_credential_emails WHERE user_id=$1 AND state='cancelled' AND payload IS NULL", created.ID).Scan(&cancelled); e != nil || cancelled != 1 {
		t.Fatal("stale email not erased", e)
	}
	// Failed delivery retries the same encrypted credential; terminal success erases it.
	mail.fail = true
	if worked, e = delivery.Deliver(ctx); !errors.Is(e, app.ErrCredentialDelivery) || !worked {
		t.Fatal("delivery retry not recorded", e)
	}
	var state string
	var attempts int
	// Check the current credential generation directly, not timestamp ordering
	// between cancelled history and the retryable current mail.
	if e = pool.QueryRow(ctx, "SELECT e.state,e.attempts FROM auth.auth_credential_emails e JOIN auth.users u ON u.id=e.user_id WHERE e.user_id=$1 AND e.credential_version=u.credential_version", created.ID).Scan(&state, &attempts); e != nil || state != "pending" || attempts != 1 {
		t.Fatalf("provider error lost pending mail: state=%q attempts=%d error=%v", state, attempts, e)
	}
	execFixture(t, pool, "UPDATE auth.auth_credential_emails SET next_attempt_at=now()-interval '1 minute' WHERE user_id=$1 AND state='pending'", created.ID)
	mail.fail = false
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = delivery.Deliver(ctx) }()
	}
	wg.Wait()
	if len(mail.messages) != 2 {
		t.Fatal("concurrent workers duplicated a leased email")
	}
	// Event-writer failure rolls back reset, credential version and encrypted queue together.
	var before int64
	_ = pool.QueryRow(ctx, "SELECT credential_version FROM auth.users WHERE id=$1", created.ID).Scan(&before)
	broken := app.NewCredentials(out.NewCredentialUnitOfWork(pool, nil, cipher), passwords, values, security.TemporaryPasswords{}, mail, "https://erp.vishaltechnopower.in/login")
	if e = broken.Reset(ctx, adminToken, created.ID, "Rollback proof for unavailable auditing"); e == nil {
		t.Fatal("missing audit writer accepted")
	}
	var after int64
	_ = pool.QueryRow(ctx, "SELECT credential_version FROM auth.users WHERE id=$1", created.ID).Scan(&after)
	if before != after {
		t.Fatal("credential change escaped rollback")
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.auth_credential_changes WHERE user_id=$1", created.ID).Scan(&count); e != nil || count != 3 {
		t.Fatal("credential audit missing", e)
	}
	if _, e = pool.Exec(ctx, "UPDATE auth.auth_credential_changes SET reason='tampered history' WHERE user_id=$1", created.ID); e == nil {
		t.Fatal("credential history mutable")
	}
	lastTemporary := strings.Split(strings.Split(mail.messages[1], "Temporary password: ")[1], "\n")[0]
	execFixture(t, pool, "UPDATE auth.users SET temporary_password_expires_at=now()-interval '1 minute' WHERE id=$1", created.ID)
	if _, e = sessions.Login(ctx, created.Username, lastTemporary); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("expired temporary password accepted", e)
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", created.ID)
	if e = credentials.Reset(ctx, adminToken, created.ID, "Inactive users must not receive credentials"); e == nil {
		t.Fatal("inactive reset accepted")
	}
	// Unconfigured provider rejects creation atomically, without orphan users.
	mail.enabled = false
	request.Username = "disabled_mail_user"
	request.Email = "disabled@example.invalid"
	if _, e = credentials.Create(ctx, adminToken, request); e == nil {
		t.Fatal("unconfigured mail accepted onboarding")
	}
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM auth.users WHERE username=$1", request.Username).Scan(&count)
	if count != 0 {
		t.Fatal("unconfigured email left orphan user")
	}
	// A changed email cancels the old recipient immediately, without delivery.
	mail.enabled = true
	recipientUser, e := credentials.Create(ctx, adminToken, request)
	if e != nil {
		t.Fatal(e)
	}
	changedRecipient := "corrected@example.invalid"
	if _, e = users.UpdateUser(ctx, adminToken, recipientUser.ID, domain.UserUpdate{Email: &changedRecipient}); e != nil {
		t.Fatal(e)
	}
	if _, e = delivery.Deliver(ctx); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, "SELECT state,payload IS NULL FROM auth.auth_credential_emails WHERE user_id=$1", recipientUser.ID).Scan(&state, &wiped); e != nil || state != "cancelled" || !wiped || len(mail.messages) != 2 {
		t.Fatal("changed recipient received old credential", e)
	}
	// A crashed final attempt becomes terminal and erases the encrypted credential.
	if e = credentials.Reset(ctx, adminToken, recipientUser.ID, "Recover from final delivery attempt crash"); e != nil {
		t.Fatal(e)
	}
	execFixture(t, pool, "UPDATE auth.auth_credential_emails SET attempts=5,state='sending',lease_until=now()-interval '1 minute' WHERE user_id=$1 AND state='pending'", recipientUser.ID)
	if _, e = delivery.Deliver(ctx); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, "SELECT state,payload IS NULL FROM auth.auth_credential_emails WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1", recipientUser.ID).Scan(&state, &wiped); e != nil || state != "failed" || !wiped {
		t.Fatal("crashed final attempt retained credential", e)
	}

}
