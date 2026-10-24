package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/go-chi/chi/v5"
	httpx "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"

	out "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	app "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventmodule "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

func TestSuperadminCreationAuthorityAndConcurrentLimit(t *testing.T) {
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
	_, e = app.NewBootstrap(out.NewBootstrapUnitOfWork(pool), passwords, values).Bootstrap(ctx, "cap_admin", "cap@example.invalid", "Aa9!"+rand.Text())
	if e != nil {
		t.Fatal(e)
	}
	var admin string
	if e = pool.QueryRow(ctx, "SELECT id::text FROM auth.users WHERE username='cap_admin'").Scan(&admin); e != nil {
		t.Fatal(e)
	}
	token, _, e := values.Issue(admin, values.Now()+3600, 0)
	if e != nil {
		t.Fatal(e)
	}
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	cipher, e := infrastructure.NewEmailCipher(hex.EncodeToString(key))
	if e != nil {
		t.Fatal(e)
	}
	journal := func(c database.Connection) events.Writer { return eventmodule.New().Writer(c) }
	unit := out.NewCredentialUnitOfWork(pool, journal, cipher)
	mail := &fixtureEmails{enabled: true}
	credentials := app.NewCredentials(unit, passwords, values, security.TemporaryPasswords{}, mail, "https://fixture.invalid/login")
	writer := values.ID()
	role := values.ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'ordinary_writer','writer@example.invalid','fixture',true,false)", writer)
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,'cap_writer','Fixture',true)", role)
	execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id) SELECT $1,id FROM auth.permissions WHERE code IN ('admin.users.write','system:admin')", role)
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", writer, role)
	writerToken, _, e := values.Issue(writer, values.Now()+3600, 0)
	if e != nil {
		t.Fatal(e)
	}
	router := chi.NewRouter()
	httpx.Register(router, httpx.Services{Credentials: credentials})
	checkHTTP := func(tok string, want int) {
		t.Helper()
		body := `{"first_name":"Fixture","last_name":"Admin","mobile":"9000000000","username":"http_fixture","email":"http@example.invalid","role_names":[],"is_superuser":true}`
		request := httptest.NewRequest("POST", "/api/v1/users", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+tok)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("superadmin create HTTP %d, want %d", response.Code, want)
		}
	}
	req := domain.UserCreate{IsSuperuser: true, FirstName: "Fixture", LastName: "Admin", Mobile: "9000000000", Username: "forbidden_admin", Email: "forbidden@example.invalid"}
	if _, e = credentials.Create(ctx, writerToken, req); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("ordinary role holder granted superadmin", e)
	}
	checkHTTP(writerToken, 403)
	if e = credentials.Reset(ctx, writerToken, admin, "Unauthorized reset attempt"); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("ordinary writer reset superadmin", e)
	}
	users := app.NewUsers(out.NewUserUnitOfWork(pool), passwords, values, domain.Policy{})
	replacementEmail := "replacement@example.invalid"
	if _, e = users.UpdateUser(ctx, writerToken, admin, domain.UserUpdate{Email: &replacementEmail}); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("ordinary writer edited superadmin", e)
	}
	disabled := app.NewCredentials(unit, passwords, values, security.TemporaryPasswords{}, &fixtureEmails{}, "https://fixture.invalid/login")
	if _, e = disabled.Create(ctx, token, req); e == nil {
		t.Fatal("unconfigured onboarding accepted")
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.users WHERE is_superuser").Scan(&count); e != nil || count != 1 {
		t.Fatal("failed creation consumed slot", e)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			<-start
			r := req
			if i == 0 {
				r.Username = "second_admin_a"
				r.Email = "second_a@example.invalid"
			} else {
				r.Username = "second_admin_b"
				r.Email = "second_b@example.invalid"
			}
			_, err := credentials.Create(ctx, token, r)
			results <- err
		}(i)
	}
	close(start)
	success, limited := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, domain.ErrSuperadminLimit) {
			limited++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || limited != 1 {
		t.Fatalf("concurrent slot results: success %d limited %d", success, limited)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.users WHERE is_superuser").Scan(&count); e != nil || count != 2 {
		t.Fatal("superadmin cap", e)
	}
	checkHTTP(token, 409)
	var audits int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.auth_credential_changes WHERE reason='New superadmin account; full platform access'").Scan(&audits); e != nil || audits != 1 {
		t.Fatal("missing superadmin audit", e)
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE is_superuser AND id<>$1", admin)
	req.Username = "third_admin"
	req.Email = "third@example.invalid"
	if _, e = credentials.Create(ctx, token, req); !errors.Is(e, domain.ErrSuperadminLimit) {
		t.Fatal("inactive superadmin freed slot", e)
	}
}

func TestBootstrapCannotCreateThirdSuperadmin(t *testing.T) {
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
	for _, name := range []string{"existing_a", "existing_b"} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,'fixture',true,true)", values.ID(), name, name+"@example.invalid")
	}
	if _, e = app.NewBootstrap(out.NewBootstrapUnitOfWork(pool), passwords, values).Bootstrap(ctx, "new_bootstrap", "new@example.invalid", "Aa9!"+rand.Text()); !errors.Is(e, domain.ErrSuperadminLimit) {
		t.Fatal("bootstrap bypassed cap", e)
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.users WHERE is_superuser").Scan(&count); e != nil || count != 2 {
		t.Fatal("bootstrap changed superadmin count", e)
	}
}
