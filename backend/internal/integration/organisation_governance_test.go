package integration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	in "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/application"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func directorUsers(c database.Connection) auth.AssignmentDirectory {
	return authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
}
func TestOrganisationDirectorNativeAuthorityAtomicityAndRecipient(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := t.Context()
	v := out.Values{}
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.manager)
	codes := []string{"admin.governance.read", "admin.governance.write"}
	role := v.ID()
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'governance fixture','test',true)", role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	ids := projectIdentities{"admin": auth.Identity{ID: f.manager, Active: true, Superuser: true}, "delegated": auth.Identity{ID: f.actor, Active: true, Permissions: codes}, "spoofed": auth.Identity{ID: f.outsider, Active: true, Superuser: true}}
	build := func(j func(database.Connection) events.Writer) *app.Service {
		return app.New(out.NewUnitOfWork(f.pool, directorUsers, j), out.NewAuthBridge(ids), v)
	}
	s := build(purchaseJournal)
	if w, e := s.Workspace(ctx, "admin"); e != nil || w.Record != nil || w.Effective || !w.CanWrite {
		t.Fatal("unassigned state", w, e)
	}
	if _, e := s.Workspace(ctx, "spoofed"); !errors.Is(e, domain.Forbidden) {
		t.Fatal("stale superuser flag trusted", e)
	}
	if _, e := s.Workspace(ctx, ""); !errors.Is(e, domain.Unauthorized) {
		t.Fatal("authentication", e)
	}
	input := domain.Input{UserID: f.outsider, Active: true, Reason: "Initial Director designation"}
	if _, e := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Save(ctx, "admin", input); e == nil {
		t.Fatal("journal failure ignored")
	}
	var count int
	if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM organisation_director_designation").Scan(&count); e != nil || count != 0 {
		t.Fatal("journal rollback", count, e)
	}
	// The slot lock serializes concurrent initial writes; one succeeds and one is stale.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Save(ctx, "admin", input); results <- e }()
	}
	wg.Wait()
	close(results)
	success, stale := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, domain.Stale) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatal("initial designation race", success, stale)
	}
	w, e := s.Workspace(ctx, "delegated")
	if e != nil || w.Record.Version != 1 || !w.Effective || len(w.History) != 1 || w.Record.CreatedBy == nil || *w.Record.CreatedBy != f.manager {
		t.Fatal("native designation", w, e)
	}
	original := *w.Record
	if _, e = s.Save(ctx, "admin", domain.Input{UserID: f.manager, Active: false, Version: 1, Reason: "Retarget revocation"}); e == nil {
		t.Fatal("revocation retarget accepted")
	}
	m := database.NewManager(f.pool)
	if _, e = out.NewDirectory(f.pool, directorUsers(f.pool)).Current(ctx); e == nil {
		t.Fatal("pool-backed recipient permitted")
	}
	recipient := func() (*domain.Designation, error) {
		var result *domain.Designation
		e := m.Within(ctx, func(ctx context.Context, c database.Connection) error {
			var e error
			result, e = out.NewDirectory(c, directorUsers(c)).Current(ctx)
			return e
		})
		return result, e
	}
	if current, e := recipient(); e != nil || current == nil || current.UserID != f.outsider {
		t.Fatal("recipient directory", current, e)
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=false WHERE id=$1", f.outsider)
	if current, e := recipient(); e != nil || current != nil {
		t.Fatal("inactive Director recipient", current, e)
	}
	w, e = s.Workspace(ctx, "admin")
	if e != nil || w.Effective || w.Record == nil {
		t.Fatal("inactive history lost", e)
	}
	if _, e = s.Save(ctx, "admin", domain.Input{UserID: f.outsider, Active: true, Version: 1, Reason: "Inactive account"}); e == nil {
		t.Fatal("inactive designation accepted")
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=true WHERE id=$1", f.outsider)
	// Deferred commit rejection must roll back local and central history and return no record.
	execFixture(t, f.pool, `CREATE FUNCTION reject_director_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture commit rejected';END $$`)
	execFixture(t, f.pool, `CREATE CONSTRAINT TRIGGER reject_director_commit AFTER UPDATE ON organisation_director_designation DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_director_commit()`)
	failed, e := s.Save(ctx, "delegated", domain.Input{UserID: f.actor, Active: true, Version: 1, Reason: "Handover"})
	if e == nil || failed.Record != nil {
		t.Fatal("uncommitted Director exposed", e)
	}
	var commitError *pgconn.PgError
	if !errors.As(e, &commitError) || commitError.Code != "P0001" || commitError.Message != "fixture commit rejected" {
		t.Fatal("expected deferred commit rejection", e)
	}
	execFixture(t, f.pool, "DROP TRIGGER reject_director_commit ON organisation_director_designation;DROP FUNCTION reject_director_commit()")
	w, e = s.Workspace(ctx, "admin")
	if e != nil || w.Record.Version != 1 || len(w.History) != 1 {
		t.Fatal("deferred rollback", e)
	}
	w, e = s.Save(ctx, "delegated", domain.Input{UserID: f.actor, Active: true, Version: 1, Reason: "Director handover"})
	if e != nil || w.Record.Version != 2 || w.Record.ID != original.ID || !w.Record.CreatedAt.Equal(original.CreatedAt) || *w.Record.CreatedBy != f.manager || *w.Record.UpdatedBy != f.actor || w.History[0].Before.UserID != f.outsider {
		t.Fatal("handover identity/history", e)
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='admin.governance.write')", role)
	if _, e = s.Save(ctx, "delegated", domain.Input{UserID: f.actor, Version: 2, Active: false, Reason: "Revoked authority"}); !errors.Is(e, domain.Forbidden) {
		t.Fatal("stale write permission trusted", e)
	}
	w, e = s.Workspace(ctx, "delegated")
	if e != nil || w.CanWrite {
		t.Fatal("read-only governance", e)
	}
	if _, e = s.Options(ctx, "delegated"); !errors.Is(e, domain.Forbidden) {
		t.Fatal("read user directory leak", e)
	}
	w, e = s.Save(ctx, "admin", domain.Input{UserID: f.actor, Version: 2, Active: false, Reason: "Revoke designation"})
	if e != nil || w.Effective || w.Record.Version != 3 {
		t.Fatal("revocation", e)
	}
	if current, e := recipient(); e != nil || current != nil {
		t.Fatal("revoked recipient", e)
	}
	w, e = s.Save(ctx, "admin", domain.Input{UserID: f.actor, Version: 3, Active: true, Reason: "Reactivate Director"})
	if e != nil || !w.Effective || w.Record.Version != 4 {
		t.Fatal("reactivation", e)
	}
	for _, sql := range []string{"DELETE FROM organisation_director_designation", "UPDATE organisation_director_designation SET created_at=clock_timestamp(),version=version+1", "UPDATE organisation_director_designation SET version=version+2", "DELETE FROM organisation_director_events", "UPDATE organisation_director_events SET reason='overwrite'", "TRUNCATE organisation_director_events"} {
		if _, e = f.pool.Exec(ctx, sql); e == nil {
			t.Fatal("SQL evidence guard", sql)
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='organisation_governance'").Scan(&count); e != nil || count != 4 {
		t.Fatal("central atomic history", count, e)
	}
	migration, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer migration.Close()
	if e = migration.Steps(-(schema.LatestVersion - 38)); e == nil {
		t.Fatal("populated designation rollback permitted")
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM organisation_director_events").Scan(&count); e != nil || count != 4 {
		t.Fatal("rollback erased evidence", e)
	}
}
func TestDirectorStrictHTTP(t *testing.T) {
	f := newPurchaseFixture(t)
	v := out.Values{}
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.manager)
	ids := projectIdentities{"admin": auth.Identity{ID: f.manager, Active: true, Superuser: true}}
	s := app.New(out.NewUnitOfWork(f.pool, directorUsers, purchaseJournal), out.NewAuthBridge(ids), v)
	r := chi.NewRouter()
	in.Register(r, s)
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		return response
	}
	path := "/api/v1/administration/director"
	if response := request("GET", path, "", ""); response.Code != 401 || response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("HTTP authentication", response.Code)
	}
	valid := `{"user_id":"` + f.outsider + `","version":0,"is_active":true,"reason":"Initial Director"}`
	for _, body := range []string{`null`, valid + `{}`, strings.Replace(valid, `"version":0,`, "", 1), strings.Replace(valid, `"is_active":true`, `"is_active":null`, 1), strings.Replace(valid, `"version":0`, `"version":0.5`, 1), strings.Replace(valid, `"reason":"Initial Director"`, `"reason":"Initial Director","created_by_id":"forged"`, 1)} {
		if response := request("POST", path, "admin", body); response.Code != 422 {
			t.Fatal("strict HTTP accepted", response.Code)
		}
	}
	if response := request("POST", path, "admin", strings.Repeat(" ", 16<<10)+valid); response.Code != http.StatusRequestEntityTooLarge {
		t.Fatal("body limit", response.Code)
	}
	response := request("POST", path, "admin", valid)
	var w domain.Workspace
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &w) != nil || w.Record == nil || !w.Effective {
		t.Fatal("HTTP save", response.Code)
	}
	if response = request("GET", path, "admin", ""); response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("HTTP read", response.Code)
	}
	if response = request("GET", path+"/options", "admin", ""); response.Code != 200 {
		t.Fatal("HTTP options", response.Code)
	}
}
