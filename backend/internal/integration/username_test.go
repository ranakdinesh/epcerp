package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	httpx "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx"
	out "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	app "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUsernameChangeAvailabilityConcurrencyAndBootstrap(t *testing.T) {
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
	const password = "Abcd123!"
	bootstrap := app.NewBootstrap(out.NewBootstrapUnitOfWork(pool), passwords, values)
	if _, e = bootstrap.Bootstrap(ctx, "root_fixture", "root@example.invalid", password); e != nil {
		t.Fatal(e)
	}
	sessions := app.NewSessions(out.NewSessionUnitOfWork(pool), passwords, values, domain.Policy{SessionLifetime: time.Hour})
	root, e := sessions.Login(ctx, "root_fixture", password)
	if e != nil {
		t.Fatal(e)
	}
	self, e := sessions.Me(ctx, root.AccessToken)
	if e != nil {
		t.Fatal(e)
	}
	journal := func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }
	account := app.NewAccount(out.NewAccountUnitOfWork(pool, journal), passwords, values)
	first, second, inactive := values.ID(), values.ID(), values.ID()
	hash, e := passwords.Hash(ctx, password)
	if e != nil {
		t.Fatal(e)
	}
	for i, id := range []string{first, second, inactive} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,$5,false)", id, []string{"claim_first", "claim_second", "inactive_name"}[i], []string{"one@example.invalid", "two@example.invalid", "inactive@example.invalid"}[i], hash, i != 2)
	}
	verifyUsernameHTTP(t, account, root.AccessToken)
	for _, name := range []string{"root_fixture", "new_available"} {
		if ok, e := account.UsernameAvailable(ctx, root.AccessToken, name); e != nil || !ok {
			t.Fatal("available name rejected", e)
		}
	}
	for _, name := range []string{"claim_first", "inactive_name"} {
		if ok, e := account.UsernameAvailable(ctx, root.AccessToken, name); e != nil || ok {
			t.Fatal("reserved name offered", e)
		}
		if e = account.ChangeUsername(ctx, root.AccessToken, name, "root_fixture"); e == nil || e.Error() != name+" already exists. Please choose a different username." {
			t.Fatal("duplicate accepted or wrong message", e)
		}
	}
	if _, e = account.UsernameAvailable(ctx, "invalid", "new_available"); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("anonymous availability", e)
	}
	if e = account.ChangeUsername(ctx, root.AccessToken, "root_renamed", "stale_name"); !errors.Is(e, domain.ErrUsernameStale) {
		t.Fatal("stale rename", e)
	}
	if e = account.ChangeUsername(ctx, root.AccessToken, "root_renamed", "root_fixture"); e != nil {
		t.Fatal("own rename", e)
	}
	current, e := sessions.Me(ctx, root.AccessToken)
	if e != nil || current.ID != self.ID || current.Username != "root_renamed" {
		t.Fatal("existing session lost identity", e)
	}
	if _, e = sessions.Login(ctx, "root_fixture", password); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("old name still logged in", e)
	}
	if _, e = sessions.Login(ctx, "root_renamed", password); e != nil {
		t.Fatal("new name cannot log in", e)
	}
	// A later claimant of the old username must never become the recovery admin.
	claimant := values.ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'root_fixture','claimant@example.invalid',$2,true,false)", claimant, hash)
	made, e := bootstrap.Bootstrap(ctx, "root_fixture", "root@example.invalid", password)
	if e != nil || made {
		t.Fatal("bootstrap replaced renamed administrator", e)
	}
	var super bool
	var bound string
	if e = pool.QueryRow(ctx, "SELECT is_superuser FROM auth.users WHERE id=$1", claimant).Scan(&super); e != nil || super {
		t.Fatal("old-name claimant promoted", e)
	}
	if e = pool.QueryRow(ctx, "SELECT user_id::text FROM auth.auth_bootstrap_identity").Scan(&bound); e != nil || bound != self.ID {
		t.Fatal("bootstrap ID drift", e)
	}
	if _, e = pool.Exec(ctx, "UPDATE auth.auth_bootstrap_identity SET user_id=$1", claimant); e == nil {
		t.Fatal("bootstrap binding mutable")
	}
	a, e := sessions.Login(ctx, "claim_first", password)
	if e != nil {
		t.Fatal(e)
	}
	b, e := sessions.Login(ctx, "claim_second", password)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	result := make(chan error, 2)
	for i, token := range []string{a.AccessToken, b.AccessToken} {
		wg.Add(1)
		go func(i int, token string) {
			defer wg.Done()
			<-start
			result <- account.ChangeUsername(ctx, token, "shared_claim", []string{"claim_first", "claim_second"}[i])
		}(i, token)
	}
	close(start)
	wg.Wait()
	close(result)
	wins, conflicts := 0, 0
	for e := range result {
		if e == nil {
			wins++
		} else if strings.Contains(e.Error(), "shared_claim already exists") {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("concurrent claims did not have one winner")
	}
	if _, e = pool.Exec(ctx, "UPDATE auth.auth_username_changes SET username='tampered'"); e == nil {
		t.Fatal("username history mutable")
	}
	execFixture(t, pool, `CREATE FUNCTION reject_username_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END; $$; CREATE CONSTRAINT TRIGGER reject_username_event AFTER INSERT ON business_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_username_event();`)
	if e = account.ChangeUsername(ctx, root.AccessToken, "rolled_back", "root_renamed"); e == nil {
		t.Fatal("audit failure accepted")
	}
	current, e = sessions.Me(ctx, root.AccessToken)
	if e != nil || current.Username != "root_renamed" {
		t.Fatal("rename not rolled back", e)
	}
	execFixture(t, pool, "UPDATE auth.users SET must_change_password=true,temporary_password_expires_at=now()+interval '15 minutes' WHERE id=$1", self.ID)
	if _, e = account.UsernameAvailable(ctx, root.AccessToken, "available_name"); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("restricted identity checked availability", e)
	}
	execFixture(t, pool, "UPDATE auth.users SET must_change_password=false,is_active=false WHERE id=$1", self.ID)
	if e = account.ChangeUsername(ctx, root.AccessToken, "blocked_name", "root_renamed"); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("inactive identity renamed", e)
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.auth_username_changes WHERE user_id=$1 AND previous_username='root_fixture' AND username='root_renamed' AND created_by_id=$1", self.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("username snapshot missing", e)
	}
}

func verifyUsernameHTTP(t *testing.T, account *app.AccountService, token string) {
	t.Helper()
	router := chi.NewRouter()
	httpx.Register(router, httpx.Services{Account: account})
	server := httptest.NewServer(router)
	defer server.Close()
	for _, c := range []struct {
		method, path, body string
		authenticated      bool
		status             int
		contains           string
	}{
		{"GET", "/api/v1/auth/me/username?username=claim_first", "", true, 200, `"available":false`},
		{"GET", "/api/v1/auth/me/username?username=root_fixture", "", true, 200, `"available":true`},
		{"GET", "/api/v1/auth/me/username?username=available", "", false, 401, ""},
		{"GET", "/api/v1/auth/me/username?username=ab", "", true, 422, "3 to 64"},
		{"PATCH", "/api/v1/auth/me/username", `{"username":"claim_first","expected_username":"root_fixture"}`, true, 409, "claim_first already exists. Please choose a different username."},
		{"PATCH", "/api/v1/auth/me/username", `{"username":"newname","expected_username":"root_fixture","user_id":"other"}`, true, 422, ""},
		{"PATCH", "/api/v1/auth/me/username", `{"username":"root_fixture","expected_username":"root_fixture"}`, true, 204, ""},
		{"POST", "/api/v1/auth/me/username", `{}`, true, 405, ""},
	} {
		req, e := http.NewRequest(c.method, server.URL+c.path, strings.NewReader(c.body))
		if e != nil {
			t.Fatal(e)
		}
		if c.authenticated {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		body, e := io.ReadAll(response.Body)
		response.Body.Close()
		if e != nil || response.StatusCode != c.status || !strings.Contains(string(body), c.contains) {
			t.Fatalf("username HTTP contract: %s status %d, expected %d", c.method, response.StatusCode, c.status)
		}
	}
}
