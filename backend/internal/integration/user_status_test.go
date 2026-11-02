package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	httpx "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx"
	out "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	app "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventmodule "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

func TestUserStatusRevokesAccessBeforeHandover(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	values, err := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: 15 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	passwords, err := security.NewPasswords()
	if err != nil {
		t.Fatal(err)
	}
	password := "Fixture-password9!"
	hash, err := passwords.Hash(ctx, password)
	if err != nil {
		t.Fatal(err)
	}
	admin, staff, writer := values.ID(), values.ID(), values.ID()
	for i, id := range []string{admin, staff, writer} {
		execFixture(t, pool, `INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,$4,true,$5)`, id, []string{"status_admin", "status_staff", "status_writer"}[i], []string{"admin@example.invalid", "staff@example.invalid", "writer@example.invalid"}[i], hash, i == 0)
	}
	adminToken, _, err := values.Issue(admin, values.Now()+3600, 0)
	if err != nil {
		t.Fatal(err)
	}
	writerToken, _, err := values.Issue(writer, values.Now()+3600, 0)
	if err != nil {
		t.Fatal(err)
	}
	role := values.ID()
	execFixture(t, pool, `INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'status_writer','Fixture',true)`, role)
	execFixture(t, pool, `INSERT INTO auth.permissions(id,code,description)VALUES($1,'admin.users.write','Fixture')ON CONFLICT(code)DO NOTHING`, values.ID())
	execFixture(t, pool, `INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='admin.users.write'`, role)
	execFixture(t, pool, `INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)`, writer, role)
	users := app.NewUsers(out.NewUserUnitOfWork(pool), passwords, values, domain.Policy{})
	sessions := app.NewSessions(out.NewSessionUnitOfWork(pool), passwords, values, domain.Policy{SessionLifetime: time.Hour})
	old, err := sessions.Login(ctx, "status_staff", password)
	if err != nil {
		t.Fatal(err)
	}
	another, err := sessions.Login(ctx, "status_staff", password)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	httpx.Register(router, httpx.Services{Users: users, Sessions: sessions})
	status := func(token, id string, active bool, want int) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"is_active": active, "reason": "Security access status decision"})
		r := httptest.NewRequest("POST", "/api/v1/users/"+id+"/status", strings.NewReader(string(body)))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("status %v: got %d want %d: %s", active, w.Code, want, w.Body.String())
		}
	}
	status(old.AccessToken, writer, false, 403)
	status(writerToken, admin, false, 403)
	status(adminToken, admin, false, 422)
	// No organisation profile, manager, or work-source providers exist. Access
	// revocation still succeeds and a durable pending case is visible to admins.
	status(writerToken, staff, false, 200)
	if _, err = sessions.Login(ctx, "status_staff", password); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("inactive login", err)
	}
	if _, err = sessions.Me(ctx, old.AccessToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("old access survived", err)
	}
	for _, pair := range []domain.TokenPair{old, another} {
		if _, err = sessions.Refresh(ctx, pair.RefreshToken); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal("refresh survived", err)
		}
	}
	var live int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM auth.refresh_tokens WHERE user_id=$1 AND revoked_at IS NULL`, staff).Scan(&live); err != nil || live != 0 {
		t.Fatal("sessions not revoked", live, err)
	}
	handovers := app.NewOffboarding(out.NewOffboardingUnitOfWork(pool, nil, nil), values)
	cases, err := handovers.Cases(ctx, adminToken)
	if err != nil || len(cases) != 1 || cases[0].UserID != staff || cases[0].State != "pending" || cases[0].ManagerID != "" {
		t.Fatalf("missing pending handover: %+v %v", cases, err)
	}
	// Retries are idempotent and concurrent requests cannot create duplicate cases.
	var wg sync.WaitGroup
	failures := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := users.SetActive(ctx, adminToken, staff, false, "Repeated security access revocation")
			failures <- err
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	status(writerToken, staff, true, 200)
	if _, err = sessions.Me(ctx, old.AccessToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("reactivation revived old access", err)
	}
	for _, pair := range []domain.TokenPair{old, another} {
		if _, err = sessions.Refresh(ctx, pair.RefreshToken); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal("reactivation revived refresh", err)
		}
	}
	fresh, err := sessions.Login(ctx, "status_staff", password)
	if err != nil {
		t.Fatal("fresh sign-in", err)
	}
	if _, err = sessions.Me(ctx, fresh.AccessToken); err != nil {
		t.Fatal("fresh access rejected", err)
	}
	cases, err = handovers.Cases(ctx, adminToken)
	if err != nil || len(cases) != 1 || cases[0].State != "pending" {
		t.Fatal("reactivation lost pending handover", err)
	}
	// The compatibility endpoint also revokes immediately, reusing the open case.
	r := httptest.NewRequest("POST", "/api/v1/users/"+staff+"/deactivate", nil)
	r.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("legacy endpoint", w.Code)
	}
	cases, err = handovers.Cases(ctx, adminToken)
	if err != nil || len(cases) != 1 {
		t.Fatal("duplicate handover", err)
	}
	var audit int
	var version int64
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM auth.auth_user_status_changes WHERE user_id=$1`, staff).Scan(&audit); err != nil || audit != 3 {
		t.Fatal("status audit", audit, err)
	}
	if err = pool.QueryRow(ctx, `SELECT credential_version FROM auth.users WHERE id=$1`, staff).Scan(&version); err != nil || version != 3 {
		t.Fatal("version", version, err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM auth.auth_user_status_changes`); err == nil {
		t.Fatal("status history mutable")
	}
	// Set up reporting only after access is revoked. The pending case can then
	// be loaded, approved and completed without reactivating the employee.
	journal := func(c database.Connection) events.Writer { return eventmodule.New().Writer(c) }
	org := app.NewOrganisation(out.NewOrganisationUnitOfWork(pool, journal, workSources), values)
	dep, err := org.SaveDepartment(ctx, adminToken, "", domain.DepartmentInput{Code: "STATUS", Name: "Status fixture", IsActive: true, Reason: "Set up delayed handover reporting"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = org.SaveEmployee(ctx, adminToken, admin, domain.EmployeeInput{FirstName: "Admin", LastName: "Manager", Mobile: "9000000000", DepartmentID: dep.ID, IsCompanyRoot: true, Reason: "Set up active reporting manager"}); err != nil {
		t.Fatal(err)
	}
	if _, err = org.SaveEmployee(ctx, adminToken, staff, domain.EmployeeInput{FirstName: "Inactive", LastName: "Staff", Mobile: "9000000001", DepartmentID: dep.ID, ManagerID: admin, Reason: "Assign manager after access revocation"}); err != nil {
		t.Fatal("cannot configure inactive handover", err)
	}
	managed := app.NewOffboarding(out.NewOffboardingUnitOfWork(pool, journal, workSources), values)
	pending := cases[0]
	for _, action := range []string{"refresh", "approve", "complete"} {
		pending, err = managed.Change(ctx, adminToken, pending.ID, action, domain.HandoverPlan{Version: pending.Version, Reason: "Manage delayed handover without access"})
		if err != nil {
			t.Fatal(action, err)
		}
	}
	inactive, err := users.User(ctx, adminToken, staff)
	if err != nil || inactive.IsActive || pending.State != "completed" {
		t.Fatal("handover changed account access", err)
	}
	// If the local security audit cannot commit, no partial status, session,
	// or handover change is reported as successful.
	execFixture(t, pool, `CREATE TRIGGER fixture_reject_status BEFORE INSERT ON auth.auth_user_status_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation()`)
	if _, err := users.SetActive(ctx, adminToken, writer, false, "Exercise atomic security audit failure"); err == nil {
		t.Fatal("audit failure was ignored")
	}
	stillActive, err := users.User(ctx, adminToken, writer)
	if err != nil || !stillActive.IsActive || stillActive.CredentialVersion != 0 {
		t.Fatal("partial status commit", err)
	}
	execFixture(t, pool, `DROP TRIGGER fixture_reject_status ON auth.auth_user_status_changes`)
	var migration int
	var dirty bool
	if err = pool.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&migration, &dirty); err != nil || dirty {
		t.Fatal("migration state", err)
	}
	t.Logf("tested migration version=%d dirty=%v", migration, dirty)
}
