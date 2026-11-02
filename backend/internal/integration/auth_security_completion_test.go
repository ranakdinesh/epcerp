package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	out "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	app "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventmodule "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

func TestAuthSecurityCompletion(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	tokens, e := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: 15 * time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	passwords, e := security.NewPasswords()
	if e != nil {
		t.Fatal(e)
	}
	password := "Aa9!" + rand.Text()
	if _, e = app.NewBootstrap(out.NewBootstrapUnitOfWork(pool), passwords, tokens).Bootstrap(ctx, "security_admin", "security@example.invalid", password); e != nil {
		t.Fatal(e)
	}
	sessions := app.NewSessions(out.NewSessionUnitOfWork(pool), passwords, tokens, domain.Policy{SessionLifetime: time.Hour})
	login := func() domain.TokenPair {
		t.Helper()
		p, e := sessions.Login(ctx, "security_admin", password)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	first, second := login(), login()
	listed, e := sessions.ListSessions(ctx, first.AccessToken)
	if e != nil || len(listed) != 2 {
		t.Fatal("session listing", listed, e)
	}
	current := 0
	other := ""
	for _, v := range listed {
		if v.Current {
			current++
		} else {
			other = v.ID
		}
	}
	if current != 1 {
		t.Fatal("current session not identified")
	}
	// A second user must never be able to revoke another user's session.
	hash, _ := passwords.Hash(ctx, password)
	outsider := tokens.ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'security_other','other@example.invalid',$2,true,false)", outsider, hash)
	stranger, e := sessions.Login(ctx, "security_other", password)
	if e != nil {
		t.Fatal(e)
	}
	if e = sessions.RevokeSession(ctx, stranger.AccessToken, other, false); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("cross-user session revocation", e)
	}
	if e = sessions.Logout(ctx, first.RefreshToken); e != nil {
		t.Fatal(e)
	}
	if _, e = sessions.Me(ctx, first.AccessToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("access survived logout", e)
	}
	if _, e = sessions.Me(ctx, second.AccessToken); e != nil {
		t.Fatal("logout revoked other device", e)
	}
	// Refresh replay revokes access tokens from the entire family too.
	rotated, e := sessions.Refresh(ctx, second.RefreshToken)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sessions.Refresh(ctx, second.RefreshToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("refresh replay accepted", e)
	}
	if _, e = sessions.Me(ctx, rotated.AccessToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("access survived family replay", e)
	}
	live, another := login(), login()
	if e = sessions.RevokeSession(ctx, live.AccessToken, "", true); e != nil {
		t.Fatal(e)
	}
	for _, p := range []domain.TokenPair{live, another} {
		if _, e = sessions.Me(ctx, p.AccessToken); !errors.Is(e, domain.ErrUnauthorized) {
			t.Fatal("access survived signout everywhere", e)
		}
	}
	live = login()
	userService := app.NewUsers(out.NewUserUnitOfWork(pool), passwords, tokens, domain.Policy{})
	u, e := userService.User(ctx, live.AccessToken, outsider)
	if e != nil {
		t.Fatal(e)
	}
	version := u.EditVersion
	name := "Changed"
	if _, e = userService.UpdateUser(ctx, live.AccessToken, outsider, domain.UserUpdate{FirstName: &name, ExpectedVersion: &version}); e != nil {
		t.Fatal(e)
	}
	if _, e = userService.UpdateUser(ctx, live.AccessToken, outsider, domain.UserUpdate{FirstName: &name, ExpectedVersion: &version}); e == nil {
		t.Fatal("stale user edit accepted")
	}
	roles := app.NewRoles(out.NewRoleUnitOfWork(pool), tokens)
	role, e := roles.CreateRole(ctx, live.AccessToken, domain.RoleCreate{Name: "security_reader", Description: "Fixture", PermissionCodes: []string{"admin.users.read"}})
	if e != nil {
		t.Fatal(e)
	}
	names := []string{role.Name}
	if _, e = userService.UpdateUser(ctx, live.AccessToken, outsider, domain.UserUpdate{RoleNames: &names}); e != nil {
		t.Fatal(e)
	}
	if _, e = userService.Users(ctx, stranger.AccessToken, domain.UserFilter{Page: 1, PageSize: 20}); e != nil {
		t.Fatal("new grant not effective", e)
	}
	rv := role.EditVersion
	active := false
	changed, e := roles.UpdateRole(ctx, live.AccessToken, role.ID, domain.RoleUpdate{Active: &active, ExpectedVersion: &rv})
	if e != nil || changed.IsActive {
		t.Fatal("role deactivation", e)
	}
	if _, e = userService.Users(ctx, stranger.AccessToken, domain.UserFilter{Page: 1, PageSize: 20}); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("inactive role still grants permission", e)
	}
	if _, e = roles.UpdateRole(ctx, live.AccessToken, role.ID, domain.RoleUpdate{Active: &active, ExpectedVersion: &rv}); e == nil {
		t.Fatal("stale role edit accepted")
	}
	if _, e = roles.CreateRole(ctx, live.AccessToken, domain.RoleCreate{Name: "fake_admin", PermissionCodes: []string{"system:admin"}}); e == nil {
		t.Fatal("role could grant superadmin sentinel")
	}
	history, e := roles.History(ctx, live.AccessToken, outsider)
	if e != nil || len(history) == 0 {
		t.Fatal("missing account audit", e)
	}
	for _, v := range history {
		if strings.Contains(string(v.Before), "password_hash") || strings.Contains(string(v.After), "password_hash") {
			t.Fatal("audit leaked password hash")
		}
	}
	if _, e = pool.Exec(ctx, "DELETE FROM auth.administration_changes"); e == nil {
		t.Fatal("audit mutable")
	}
	// Verification uses a fake transport: no real messages or providers.
	journal := func(c database.Connection) events.Writer { return eventmodule.New().Writer(c) }
	mail := &fixtureEmails{enabled: true}
	account := app.NewAccount(out.NewAccountUnitOfWork(pool, journal), passwords, tokens).WithEmailSender(mail, "https://example.invalid")
	email := "verified@example.invalid"
	if e = account.Change(ctx, live.AccessToken, password, &email, nil); e != nil {
		t.Fatal(e)
	}
	before, e := sessions.Me(ctx, live.AccessToken)
	if e != nil || before.Email == email {
		t.Fatal("unverified email applied", e)
	}
	message := mail.messages[len(mail.messages)-1]
	parts := strings.Split(message, "#token=")
	if len(parts) != 2 {
		t.Fatal("missing verification link")
	}
	raw := strings.Split(parts[1], "\n")[0]
	if e = account.ConfirmEmail(ctx, raw); e != nil {
		t.Fatal(e)
	}
	if e = account.ConfirmEmail(ctx, raw); e == nil {
		t.Fatal("verification replay accepted")
	}
	if _, e = sessions.Me(ctx, live.AccessToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("email change did not revoke sessions", e)
	}
	p, e := sessions.Login(ctx, email, password)
	if e != nil {
		t.Fatal("verified address cannot sign in", e)
	}
	// Per-account throttling persists across requests and identifier aliases.
	for i := 0; i < 5; i++ {
		if _, e = sessions.Login(ctx, "security_other", "wrong password"); !errors.Is(e, domain.ErrUnauthorized) {
			t.Fatal(e)
		}
	}
	if _, e = sessions.Login(ctx, "other@example.invalid", password); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("account throttle bypassed using email", e)
	}
	if _, e = sessions.Me(ctx, p.AccessToken); e != nil {
		t.Fatal("another account affected by throttle", e)
	}
	// Store only digests; verification tokens are never stored in plaintext.
	var persisted string
	if e = pool.QueryRow(ctx, "SELECT token_hash FROM auth.email_changes LIMIT 1").Scan(&persisted); e != nil || persisted == raw {
		t.Fatal("unsafe verification token storage", e)
	}
	rawHistory, _ := json.Marshal(history)
	if strings.Contains(string(rawHistory), password) {
		t.Fatal("audit leaked credential")
	}
}

func TestAuthAdministrationPermissionMatrix(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	tokens, e := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: time.Hour})
	if e != nil {
		t.Fatal(e)
	}
	passwords, e := security.NewPasswords()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = app.NewBootstrap(out.NewBootstrapUnitOfWork(pool), passwords, tokens).Bootstrap(ctx, "matrix_root", "matrix-root@example.invalid", "Aa9!"+rand.Text()); e != nil {
		t.Fatal(e)
	}
	var root string
	if e = pool.QueryRow(ctx, "SELECT id::text FROM auth.users WHERE username='matrix_root'").Scan(&root); e != nil {
		t.Fatal(e)
	}
	target := tokens.ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'matrix_target','matrix-target@example.invalid','fixture',true,false)", target)
	roleID := tokens.ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,'matrix_target_role','Fixture',true)", roleID)
	users := app.NewUsers(out.NewUserUnitOfWork(pool), passwords, tokens, domain.Policy{})
	roles := app.NewRoles(out.NewRoleUnitOfWork(pool), tokens)
	type capabilities struct{ usersRead, usersWrite, rolesRead, rolesWrite, audit, super bool }
	for _, tc := range []struct {
		name  string
		codes []string
		want  capabilities
	}{
		{"reader", []string{"admin.users.read", "admin.roles.read"}, capabilities{usersRead: true, rolesRead: true}},
		{"user_manager", []string{"admin.users.read", "admin.users.write"}, capabilities{usersRead: true, usersWrite: true}},
		{"role_manager", []string{"admin.roles.read", "admin.roles.write"}, capabilities{rolesRead: true, rolesWrite: true}},
		{"auditor", []string{"admin.audit.read"}, capabilities{audit: true}},
		{"legacy_sentinel", []string{"system:admin"}, capabilities{}},
		{"root", nil, capabilities{true, true, true, true, true, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := root
			if !tc.want.super {
				actor = tokens.ID()
				bundle := tokens.ID()
				execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,'fixture',true,false)", actor, "matrix_"+tc.name, tc.name+"@example.invalid")
				execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,$2,'Fixture',true)", bundle, "matrix_"+tc.name)
				for _, code := range tc.codes {
					execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id) SELECT $1,id FROM auth.permissions WHERE code=$2", bundle, code)
				}
				execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", actor, bundle)
			}
			token, _, e := tokens.Issue(actor, tokens.Now()+3600, 0)
			if e != nil {
				t.Fatal(e)
			}
			check := func(name string, allowed bool, e error) {
				t.Helper()
				if allowed && e != nil {
					t.Fatalf("%s unexpectedly denied: %v", name, e)
				}
				if !allowed && !errors.Is(e, domain.ErrForbidden) {
					t.Fatalf("%s unexpectedly allowed or wrong denial: %v", name, e)
				}
			}
			_, e = users.Users(ctx, token, domain.UserFilter{Page: 1, PageSize: 20})
			check("list users", tc.want.usersRead, e)
			_, e = roles.Roles(ctx, token)
			check("list roles", tc.want.rolesRead, e)
			name := "Matrix"
			_, e = users.UpdateUser(ctx, token, target, domain.UserUpdate{FirstName: &name})
			check("edit user", tc.want.usersWrite, e)
			description := "Matrix role"
			_, e = roles.UpdateRole(ctx, token, roleID, domain.RoleUpdate{Description: &description})
			check("edit role", tc.want.rolesWrite, e)
			_, e = roles.History(ctx, token, target)
			check("read restricted history", tc.want.audit, e)
			_, e = users.SetActive(ctx, token, target, false, "Matrix revocation verification")
			check("deactivate user", tc.want.usersWrite, e)
			if tc.want.usersWrite {
				_, e = users.SetActive(ctx, token, target, true, "Matrix restoration verification")
				if e != nil {
					t.Fatal(e)
				}
			}
			_, e = users.UpdateUser(ctx, token, root, domain.UserUpdate{FirstName: &name})
			check("edit superadmin", tc.want.super, e)
		})
	}
}
