package integration

import (
	"bytes"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	in "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectResponsibilitiesStrictScopedHTTPAndCommitFailure(t *testing.T) {
	f := newPurchaseFixture(t)
	v := out.Values{}
	role := v.ID()
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.manager)
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'specialist','fixture',true)", role)
	codes := []string{"projects.team.read", "projects.budgets.read", "projects.budgets.write"}
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.outsider, role)
	ids := projectIdentities{"admin": auth.Identity{ID: f.manager, Active: true, Superuser: true}, "specialist": auth.Identity{ID: f.outsider, Active: true, Permissions: codes}}
	router := func(j func(database.Connection) events.Writer) http.Handler {
		r := chi.NewRouter()
		in.Register(r, app.New(out.NewUnitOfWork(f.pool, accessProviders, j), out.NewAuthBridge(ids), v))
		return r
	}
	normal := router(purchaseJournal)
	request := func(h http.Handler, method, path, token string, raw []byte) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	path := "/api/v1/projects/" + f.project + "/responsibilities"
	input := domain.Input{UserID: f.outsider, BundleID: role, Responsibility: "Finance", Permissions: codes, Active: true, Reason: "Appoint Finance"}
	body, _ := json.Marshal(input)
	for _, tc := range []struct {
		name   string
		raw    []byte
		status int
	}{
		{"missing fields", []byte(`{}`), 422},
		{"null", bytes.Replace(body, []byte(`"version":0`), []byte(`"version":null`), 1), 422},
		{"unknown", append(append([]byte{}, body[:len(body)-1]...), []byte(`,"unexpected":true}`)...), 422},
		{"fractional version", bytes.Replace(body, []byte(`"version":0`), []byte(`"version":1.5`), 1), 422},
		{"large body", []byte(strings.Repeat(" ", 33<<10)), 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := request(normal, "POST", path, "admin", tc.raw)
			if rr.Code != tc.status {
				t.Fatalf("status %d want %d", rr.Code, tc.status)
			}
		})
	}
	if rr := request(normal, "GET", path, "", nil); rr.Code != 401 || rr.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("anonymous team read")
	}
	if rr := request(normal, "GET", path, "specialist", nil); rr.Code != 403 {
		t.Fatal("global specialist team read")
	}
	denied := router(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	if rr := request(denied, "POST", path, "admin", body); rr.Code != 500 || strings.Contains(rr.Body.String(), "fixture") {
		t.Fatal("journal failure exposed internal detail")
	}
	var count int
	if e := f.pool.QueryRow(t.Context(), "SELECT count(*) FROM project_responsibility_profiles").Scan(&count); e != nil || count != 0 {
		t.Fatal("HTTP journal failure persisted", count, e)
	}
	rr := request(normal, "POST", path, "admin", body)
	if rr.Code != 201 {
		t.Fatal("valid profile create", rr.Code, rr.Body.String())
	}
	var profile domain.Profile
	if e := json.Unmarshal(rr.Body.Bytes(), &profile); e != nil || profile.Version != 1 {
		t.Fatal("profile response", e)
	}
	if rr = request(normal, "GET", path, "specialist", nil); rr.Code != 200 || strings.Contains(rr.Body.String(), "password") {
		t.Fatal("scoped specialist safe list", rr.Code)
	}
	if rr = request(normal, "GET", path+"/options", "specialist", nil); rr.Code != 403 {
		t.Fatal("specialist options leak")
	}
	if rr = request(normal, "GET", "/api/v1/projects/"+f.otherProject+"/responsibilities", "specialist", nil); rr.Code != 403 {
		t.Fatal("other project read")
	}
	// A real PostgreSQL deferred failure proves nothing from the staged write escapes.
	execFixture(t, f.pool, `CREATE FUNCTION reject_profile_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced profile commit rejection';END $$`)
	execFixture(t, f.pool, `CREATE CONSTRAINT TRIGGER reject_profile_commit AFTER UPDATE ON project_responsibility_profiles DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_profile_commit()`)
	input.Version = 1
	input.Reason = "Change responsibility"
	input.Responsibility = "Accounts"
	body, _ = json.Marshal(input)
	if rr = request(normal, "PATCH", path+"/"+profile.ID, "admin", body); rr.Code != 500 || strings.Contains(rr.Body.String(), "Accounts") {
		t.Fatal("commit failure exposed uncommitted profile", rr.Code)
	}
	var version int
	if e := f.pool.QueryRow(t.Context(), "SELECT version FROM project_responsibility_profiles WHERE id=$1", profile.ID).Scan(&version); e != nil || version != 1 {
		t.Fatal("failed commit changed profile", version, e)
	}
	if e := f.pool.QueryRow(t.Context(), "SELECT count(*) FROM project_responsibility_events").Scan(&count); e != nil || count != 1 {
		t.Fatal("failed commit retained audit", count, e)
	}
	if rr = request(normal, "DELETE", path+"/"+profile.ID, "admin", nil); rr.Code != 405 {
		t.Fatal("unsupported method allowed", rr.Code)
	}
}
