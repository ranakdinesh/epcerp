package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestNativeAPI executes the actual Go binary against a database built only with
// Go migrations. Neither the Python checkout nor its interpreter participates.
func TestNativeAPI(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	// Port reservation is a read-only availability check, never a process kill.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("cannot reserve a fixture API port")
	}
	apiAddress := listener.Addr().String()
	listener.Close()
	root := filepath.Join("..", "..")
	binary := filepath.Join(t.TempDir(), "api")
	build := exec.Command("go", "build", "-o", binary, "./cmd/api")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("Go API build failed: %s", out)
	}
	password, secret := "Aa1!"+rand.Text(), rand.Text()+rand.Text()
	var evidenceKey [32]byte
	if _, err = rand.Read(evidenceKey[:]); err != nil {
		t.Fatal("fixture encryption key generation failed")
	}
	logPath := filepath.Join(t.TempDir(), "api.log")
	logs, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary)
	env := map[string]string{
		"EPCERP_DOCUMENT_PACKAGE_ROOT": filepath.Join(t.TempDir(), "document-packages"),
		"EPCERP_DOCUMENT_READING_KEY":  hex.EncodeToString(evidenceKey[:]),
		"EPCERP_IMAGE_ENCRYPTION_KEY":  hex.EncodeToString(evidenceKey[:]),
		"VTPL_STORAGE_SECRET_KEY":      rand.Text(),
		"EPCERP_HTTP_ADDRESS":          apiAddress, "EPCERP_DATABASE_URL": dsn, "EPCERP_SUPPLIER_DOCUMENT_KEY": hex.EncodeToString(evidenceKey[:]), "VTPL_ENVIRONMENT": "test", "VTPL_JWT_SECRET_KEY": secret,
		"VTPL_JWT_ISSUER": "epcERP", "VTPL_JWT_AUDIENCE": "epcERP-api",
		"VTPL_BOOTSTRAP_SUPERADMIN_USERNAME": "native_admin", "VTPL_BOOTSTRAP_SUPERADMIN_EMAIL": "admin@example.invalid", "VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD": password,
		"EPCERP_BREVO_API_KEY": "", "EPCERP_EMAIL_SENDER_EMAIL": "", "EPCERP_RECOVERY_SENDER_EMAIL": "", "VTPL_LOG_LEVEL": "info", "VTPL_ALLOWED_HOSTS": "127.0.0.1,localhost", "VTPL_FORCE_HTTPS": "false", "VTPL_RATE_LIMIT_REQUESTS": "10000", "VTPL_RATE_LIMIT_WINDOW_SECONDS": "60",
	}
	if os.Getenv("EPCERP_GO_MINIO_INTEGRATION") == "1" {
		env["VTPL_STORAGE_ENDPOINT"] = os.Getenv("EPCERP_MINIO_TEST_ENDPOINT")
		env["VTPL_STORAGE_ACCESS_KEY"] = os.Getenv("EPCERP_MINIO_TEST_ACCESS_KEY")
		env["VTPL_STORAGE_SECRET_KEY"] = os.Getenv("EPCERP_MINIO_TEST_SECRET_KEY")
		env["VTPL_STORAGE_BUCKET"] = "api-fixture-images"
	}
	command.Env = append([]string{}, os.Environ()...)
	for key, value := range env {
		command.Env = append(command.Env, key+"="+value)
	}
	command.Stdout, command.Stderr = logs, logs
	if err = command.Start(); err != nil {
		logs.Close()
		t.Fatal("API start failed")
	}
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()
	var stopped bool
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		command.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-finished:
			if err != nil {
				t.Error("API shutdown failed")
			}
		case <-time.After(15 * time.Second):
			command.Process.Kill()
			<-finished
			t.Error("API shutdown timed out")
		}
		logs.Close()
	}
	t.Cleanup(stop)
	client := &http.Client{Timeout: 10 * time.Second}
	base := "http://" + apiAddress
	ready := false
	for deadline := time.Now().Add(25 * time.Second); time.Now().Before(deadline); {
		response, e := client.Get(base + "/api/v1/health/ready")
		if e == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("Go API did not become ready")
	}
	call := func(method, path, token string, body any, want int) any {
		t.Helper()
		var reader io.Reader
		if body != nil {
			data, e := json.Marshal(body)
			if e != nil {
				t.Fatal(e)
			}
			reader = bytes.NewReader(data)
		}
		request, e := http.NewRequest(method, base+path, reader)
		if e != nil {
			t.Fatal(e)
		}
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response, e := client.Do(request)
		if e != nil {
			t.Fatal("API request failed")
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("%s %s: got %d want %d", method, path, response.StatusCode, want)
		}
		var value any
		if e = json.NewDecoder(response.Body).Decode(&value); e != nil && e != io.EOF {
			t.Fatal("invalid API JSON")
		}
		return value
	}
	login := call("POST", "/api/v1/auth/login", "", map[string]any{"identifier": "native_admin", "password": password}, 200).(map[string]any)
	token := login["access_token"].(string)
	me := call("GET", "/api/v1/auth/me", token, nil, 200).(map[string]any)
	admin := me["id"].(string)
	// All modules are registered without opt-in environment flags; authorization
	// still applies to their endpoints, including formerly disabled capabilities.
	for _, path := range []string{
		"/api/v1/tender-fms", "/api/v1/suppliers", "/api/v1/clients", "/api/v1/manufacturers",
		"/api/v1/categories", "/api/v1/categories/browse", "/api/v1/catalogue-items",
		"/api/v1/client-references/client-types", "/api/v1/client-references/industries",
		"/api/v1/client-references/states", "/api/v1/client-references/cities",
		"/api/v1/documents?entity_type=project&entity_id=" + admin,
	} {
		call("GET", path, "", nil, 401)
		call("GET", path, token, nil, 200)
	}
	// Removed legacy-import paths must not accept records, even from an administrator.
	call("POST", "/api/v1/clients/legacy-import", token, nil, http.StatusMethodNotAllowed)
	call("POST", "/api/v1/projects/legacy-import", token, nil, http.StatusMethodNotAllowed)
	createReference := func(kind, name, code, state string) string {
		payload := map[string]any{"name": name, "code": code}
		if state != "" {
			payload["state_id"] = state
		}
		return call("POST", "/api/v1/client-references/"+kind, token, payload, 201).(map[string]any)["id"].(string)
	}
	ctype := createReference("client-types", "Native Fixture Type", "NATIVE-TYPE", "")
	industry := createReference("industries", "Native Fixture Industry", "NATIVE-INDUSTRY", "")
	state := createReference("states", "Maharashtra", "MH", "")
	city := createReference("cities", "Native Fixture City", "NATIVE-CITY", state)
	call("POST", "/api/v1/client-references/client-types", token, map[string]any{"name": "native fixture type", "code": "OTHER"}, 409)
	createReference("industries", "Straße", "STREET-DE", "")
	createReference("industries", "Strasse", "STREET-ASCII", "") // intentional Go simple-fold rule
	call("PATCH", "/api/v1/client-references/industries/"+industry, token, map[string]any{"name": "Native Fixture Industry Updated", "code": "NATIVE-INDUSTRY"}, 200)
	clientBody := map[string]any{"legal_name": "Native Fixture Client", "client_type_id": ctype, "industry_id": industry, "state_id": state, "city_id": city, "postal_code": "400001", "address_line_1": "Fixture address", "pan": "AAACW4202F", "gstin": "27AAACW4202F1ZQ", "contacts": []any{map[string]any{"full_name": "Native Fixture Contact", "email": "contact@example.invalid", "phone": "+91 98765 43210", "is_primary": true}}}
	record := call("POST", "/api/v1/clients", token, clientBody, 201).(map[string]any)
	cid := record["id"].(string)
	// Missing type coverage is reported, but does not block creation.
	if len(record["missing_contact_types"].([]any)) != 3 {
		t.Fatal("missing coverage not reported")
	}
	contact := func(kind string) any {
		return map[string]any{"full_name": "Coverage fixture " + kind, "contact_type": kind, "email": "coverage@example.invalid", "phone": "9876543210"}
	}
	clientBody["contacts"] = []any{contact("EIC"), contact("EIC"), contact("HOD"), contact("SITE_CONTACT")}
	complete := call("PATCH", "/api/v1/clients/"+cid, token, clientBody, 200).(map[string]any)
	if len(complete["missing_contact_types"].([]any)) != 0 || len(complete["contacts"].([]any)) != 5 {
		t.Fatal("coverage or repeated types lost")
	}
	if _, e := pool.Exec(context.Background(), "UPDATE client_contacts SET contact_type='INVALID' WHERE client_id=$1", cid); e == nil {
		t.Fatal("database accepted unknown type")
	}
	clientBody["contacts"] = []any{contact("INVALID")}
	call("PATCH", "/api/v1/clients/"+cid, token, clientBody, 422)
	clientBody["contacts"] = []any{}
	empty := call("PATCH", "/api/v1/clients/"+cid, token, clientBody, 200).(map[string]any)
	if len(empty["missing_contact_types"].([]any)) != 3 {
		t.Fatal("retired contacts counted as coverage")
	}

	identity := call("GET", "/api/v1/auth/me", token, nil, 200).(map[string]any)
	project := call("POST", "/api/v1/projects", token, map[string]any{"name": "Native Ownership Project", "client_id": cid, "project_manager_id": identity["id"], "procurement_executive_id": identity["id"], "project_type": "electrical", "site_state": "Delhi", "site_city": "New Delhi", "contract_value": "100000.25", "currency": "INR", "schedule_start_basis": "loi_date", "loi_date": "2026-10-01", "planned_duration_days": "31"}, 201).(map[string]any)
	if project["procurement_executive_id"] != identity["id"] || project["contract_value"] != "100000.25" || project["planned_completion_date"] != "2026-10-31" {
		t.Fatal("project API lost capture")
	}
	call("GET", "/api/v1/projects/"+project["id"].(string), token, nil, 200)
	call("GET", "/api/v1/projects/"+project["id"].(string)+"/manager-history", token, nil, 200)
	call("GET", "/api/v1/projects/assignment-options", token, nil, 200)
	call("GET", "/api/v1/clients/"+cid, token, nil, 200)
	call("POST", "/api/v1/clients", token, clientBody, 409)
	clientBody["contacts"] = []any{map[string]any{"full_name": "Replacement Fixture Contact", "email": "replacement@example.invalid", "phone": "+91 98765 43211", "is_primary": true}}
	call("PATCH", "/api/v1/clients/"+cid, token, clientBody, 200)
	call("POST", "/api/v1/clients/"+cid+"/deactivate", token, nil, 200)
	call("POST", "/api/v1/clients/"+cid+"/activate", token, nil, 200)
	call("GET", "/api/v1/clients", token, nil, 200)
	var retired int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM client_contacts WHERE client_id=$1 AND NOT is_active", cid).Scan(&retired); err != nil || retired != 6 {
		t.Fatal("client contact history lost", err)
	}
	call("POST", "/api/v1/client-references/states/"+state+"/deactivate", token, nil, 200)
	call("POST", "/api/v1/client-references/states/"+state+"/activate", token, nil, 200)
	var active bool
	pool.QueryRow(context.Background(), "SELECT is_active FROM indian_cities WHERE id=$1", city).Scan(&active)
	if active {
		t.Fatal("state activation incorrectly revived city")
	}
	call("POST", "/api/v1/client-references/cities/"+city+"/activate", token, nil, 200)
	// Catalogue has no administrative API yet; own fixture is deliberately created only
	// in this disposable cluster to exercise the published read contract.
	catalogueID := "00000000-0000-4000-8000-000000000001"
	if _, err = pool.Exec(context.Background(), "WITH unit AS (INSERT INTO catalogue_units(id,code,name) VALUES(md5('manufacturer-unit')::uuid,'MU','Manufacturer unit') RETURNING id) INSERT INTO categories(id,code,name,unit_id,created_by_id,updated_by_id) SELECT $1,'ACTIVE','Native Fixture Category',id,$2,$2 FROM unit", catalogueID, admin); err != nil {
		t.Fatal(err)
	}
	manufacturer := map[string]any{"code": "NATIVE", "name": "Native Fixture Manufacturer", "data": map[string]any{"eligible": []string{"ACTIVE"}, "notes": "Private fixture notes"}, "reason": "Private fixture create reason"}
	created := call("POST", "/api/v1/manufacturers", token, manufacturer, 201).(map[string]any)
	mid := created["id"].(string)
	call("GET", "/api/v1/manufacturers", token, nil, 200)
	call("POST", "/api/v1/manufacturers", token, manufacturer, 422)
	call("PATCH", "/api/v1/manufacturers/"+mid+"/profile", token, map[string]any{"name": "Native Fixture Manufacturer Updated", "data": manufacturer["data"], "version": 1, "reason": "Private fixture update reason"}, 200)
	call("PATCH", "/api/v1/manufacturers/"+mid, token, map[string]any{"is_active": false, "version": 1, "reason": "Stale fixture"}, 409)
	call("PATCH", "/api/v1/manufacturers/"+mid, token, map[string]any{"is_active": "off", "version": 2, "reason": "Invalid boolean"}, 422)
	call("PATCH", "/api/v1/manufacturers/"+mid, token, map[string]any{"is_active": false, "version": 2, "reason": "Private fixture retire reason"}, 200)
	history := call("GET", "/api/v1/manufacturers/"+mid+"/history", token, nil, 200).([]any)
	if len(history) != 3 {
		t.Fatal("missing immutable history")
	}
	for _, value := range history {
		if value.(map[string]any)["created_by_id"] != admin {
			t.Fatal("audit actor lost")
		}
	}
	// A deferred audit error proves late commit failure rolls back the master as well.
	_, err = pool.Exec(context.Background(), `CREATE FUNCTION reject_fixture_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture audit rejected'; END; $$; CREATE CONSTRAINT TRIGGER reject_fixture_audit AFTER INSERT ON manufacturers_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_fixture_audit()`)
	if err != nil {
		t.Fatal(err)
	}
	call("PATCH", "/api/v1/manufacturers/"+mid, token, map[string]any{"is_active": true, "version": 3, "reason": "Failed fixture"}, 500)
	var version int
	pool.QueryRow(context.Background(), "SELECT version FROM manufacturers WHERE id=$1", mid).Scan(&version)
	if version != 3 {
		t.Fatal("late commit retained master change")
	}
	pool.Exec(context.Background(), "DROP TRIGGER reject_fixture_audit ON manufacturers_events; DROP FUNCTION reject_fixture_audit()")
	// Same-version concurrent changes must result in one committed mutation.
	statuses := make(chan int, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			body := strings.NewReader(`{"is_active":true,"version":3,"reason":"Concurrent fixture"}`)
			req, _ := http.NewRequest("PATCH", base+"/api/v1/manufacturers/"+mid, body)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			res, e := client.Do(req)
			if e != nil {
				statuses <- 0
				return
			}
			defer res.Body.Close()
			statuses <- res.StatusCode
		}()
	}
	workers.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal("lost-update protection", counts)
	}
	role := call("POST", "/api/v1/roles", token, map[string]any{"name": "native.reader", "description": "Fixture reader", "permission_codes": []string{"settings.manufacturers.read"}}, 201).(map[string]any)
	staffPassword := "Aa1!" + rand.Text()
	call("POST", "/api/v1/users", token, map[string]any{"username": "native_reader", "email": "reader@example.invalid", "password": staffPassword, "role_names": []string{"native.reader"}}, 422)
	call("POST", "/api/v1/users", token, map[string]any{"username": "native_reader", "email": "reader@example.invalid", "first_name": "Native", "last_name": "Reader", "mobile": "+91 9000000000", "role_names": []string{"native.reader"}}, 503)
	// Permanent staff fixture for unrelated API scenarios; onboarding is separately
	// exercised through real services, encrypted queue and disposable DB tests.
	fixturePasswords, e := security.NewPasswords()
	if e != nil {
		t.Fatal(e)
	}
	fixtureHash, e := fixturePasswords.Hash(context.Background(), staffPassword)
	if e != nil {
		t.Fatal(e)
	}
	tokensForFixture, e := security.NewTokens(security.TokenConfig{Secret: secret, Issuer: "epcERP", Audience: "epcERP-api", AccessLifetime: time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	fixtureID := tokensForFixture.ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'native_reader','reader@example.invalid',$2,true,false)", fixtureID, fixtureHash)
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", fixtureID, role["id"])
	staff := call("POST", "/api/v1/auth/login", "", map[string]any{"identifier": "native_reader", "password": staffPassword}, 200).(map[string]any)["access_token"].(string)

	staffMe := call("GET", "/api/v1/auth/me", staff, nil, 200).(map[string]any)
	staffID := staffMe["id"].(string)
	call("GET", "/api/v1/auth/user_profile/me", "", nil, 401)
	call("PATCH", "/api/v1/auth/user_profile/me", staff, map[string]any{"display_name": "Staff Reader", "user_id": admin}, 422)
	call("PATCH", "/api/v1/auth/user_profile/me", staff, map[string]any{"display_name": "Staff Reader"}, 200)
	call("GET", "/api/v1/auth/user_profile/"+staffID, token, nil, 200)
	if os.Getenv("EPCERP_GO_MINIO_INTEGRATION") == "1" {
		var upload bytes.Buffer
		if e := png.Encode(&upload, image.NewRGBA(image.Rect(0, 0, 32, 16))); e != nil {
			t.Fatal(e)
		}
		req, e := http.NewRequest("PUT", base+"/api/v1/auth/user_profile/me/photo", &upload)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+staff)
		req.Header.Set("Content-Type", "image/png")
		response, e := client.Do(req)
		if e != nil {
			t.Fatal("photo upload request failed")
		}
		var metadata map[string]any
		e = json.NewDecoder(response.Body).Decode(&metadata)
		response.Body.Close()
		if e != nil || response.StatusCode != 200 || metadata["has_photo"] != true {
			t.Fatal("photo upload contract failed")
		}
		var key string
		var raw []byte
		if e = pool.QueryRow(context.Background(), "SELECT photo_object_key,photo FROM auth.user_profile WHERE user_id=$1", staffID).Scan(&key, &raw); e != nil || key == "" || len(raw) != 0 {
			t.Fatal("image persisted outside object store", e)
		}
		req, e = http.NewRequest("GET", base+"/api/v1/auth/user_profile/"+staffID+"/photo", nil)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		response, e = client.Do(req)
		if e != nil {
			t.Fatal("photo read request failed")
		}
		cfg, format, e := image.DecodeConfig(response.Body)
		response.Body.Close()
		if e != nil || response.StatusCode != 200 || format != "jpeg" || cfg.Width != 256 || cfg.Height != 256 || response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("photo read contract failed")
		}
		call("DELETE", "/api/v1/auth/user_profile/me/photo", staff, nil, 200)
		call("GET", "/api/v1/auth/user_profile/"+staffID+"/photo", token, nil, 204)
	}
	call("PATCH", "/api/v1/auth/me/email", staff, map[string]any{"email": "changed@example.invalid", "current_password": rand.Text()}, 422)
	call("PATCH", "/api/v1/auth/me/email", staff, map[string]any{"email": "changed@example.invalid", "current_password": staffPassword}, 204)
	// A rejected central event must roll back identity and session changes too.
	execFixture(t, pool, `CREATE FUNCTION reject_account_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END; $$; CREATE CONSTRAINT TRIGGER reject_account_event AFTER INSERT ON business_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.source_module='auth') EXECUTE FUNCTION reject_account_event();`)
	replacement := "Aa1!" + rand.Text()
	call("POST", "/api/v1/auth/me/password", staff, map[string]any{"current_password": staffPassword, "password": replacement}, 500)
	call("POST", "/api/v1/auth/login", "", map[string]any{"identifier": "native_reader", "password": staffPassword}, 200)
	execFixture(t, pool, "DROP TRIGGER reject_account_event ON business_events; DROP FUNCTION reject_account_event()")
	call("POST", "/api/v1/auth/me/password", staff, map[string]any{"current_password": rand.Text(), "password": replacement}, 422)
	call("POST", "/api/v1/auth/me/password", staff, map[string]any{"current_password": staffPassword, "password": replacement, "user_id": admin}, 422)
	call("POST", "/api/v1/auth/me/password", staff, map[string]any{"current_password": staffPassword, "password": replacement}, 204)
	call("POST", "/api/v1/auth/login", "", map[string]any{"identifier": "native_reader", "password": staffPassword}, 401)
	replacementLogin := call("POST", "/api/v1/auth/login", "", map[string]any{"identifier": "native_reader", "password": replacement}, 200).(map[string]any)
	call("GET", "/api/v1/manufacturers", staff, nil, 401)
	staff = replacementLogin["access_token"].(string)
	var live int
	if e := pool.QueryRow(context.Background(), "SELECT count(*) FROM auth.refresh_tokens WHERE user_id=$1 AND revoked_at IS NULL", staffID).Scan(&live); e != nil || live != 1 {
		t.Fatal("prior refresh sessions were not revoked", e)
	}
	call("GET", "/api/v1/manufacturers", staff, nil, 200)
	call("POST", "/api/v1/manufacturers", staff, manufacturer, 403)
	call("PATCH", "/api/v1/roles/"+role["id"].(string), token, map[string]any{"is_active": false}, 200)
	call("GET", "/api/v1/manufacturers", staff, nil, 403)
	rotated := call("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": login["refresh_token"]}, 200).(map[string]any)
	call("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": login["refresh_token"]}, 401)
	call("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": rotated["refresh_token"]}, 401)
	var journalCount, sourceAuditCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM business_events WHERE source_module='manufacturers'").Scan(&journalCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM manufacturers_events").Scan(&sourceAuditCount); err != nil {
		t.Fatal(err)
	}
	if journalCount == 0 || journalCount != sourceAuditCount {
		t.Fatal("central manufacturer events lost or duplicated", journalCount, sourceAuditCount)
	}
	var invalid int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM business_events e LEFT JOIN manufacturers_events a ON a.id=e.id WHERE e.source_module='manufacturers' AND (a.id IS NULL OR e.actor_id IS DISTINCT FROM a.created_by_id OR e.entity_id<>a.subject_id)").Scan(&invalid); err != nil || invalid != 0 {
		t.Fatal("publisher attribution mismatch", err)
	}
	stop()
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{password, staffPassword, secret, token, staff, dsn, "Native Ownership Project", "Native Fixture Manufacturer", "Private fixture", "Fixture address"} {
		if bytes.Contains(logged, []byte(private)) {
			t.Fatal("structured logs exposed private values")
		}
	}
	t.Log("Go-migrated actual API: Auth rotation/live permissions, Clients/contact history, references/cascade, Manufacturers/audit/version/rollback/race and log privacy passed")
}
