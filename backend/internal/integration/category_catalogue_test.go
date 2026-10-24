package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	authInfrastructure "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"

	masterHTTP "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type failingCategoryJournal struct{}

func (failingCategoryJournal) Record(context.Context, events.Event, []events.Target) error {
	return errors.New("fixture audit failure")
}
func TestCategoryCatalogueImportHTTPAndAtomicity(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actorID := "135bcaf6-e4ec-4809-b7e9-42b94c332891"
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'category_fixture','fixture@example.invalid','fixture',true,false)", actorID)
	identities := projectIdentities{"admin": auth.Identity{ID: actorID, Active: true, Superuser: true}, "reader": auth.Identity{ID: actorID, Active: true, Permissions: []string{"settings.categories.read"}}, "denied": auth.Identity{ID: actorID, Active: true}}
	journal := func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }
	newService := func(j func(database.Connection) events.Writer) *masterApplication.CategoryService {
		return masterApplication.NewCategoryService(masterRepository.NewCategoryUnitOfWork(pool, j), authInfrastructure.NewAuthBridge(identities), masterRepository.CategoryValues{})
	}
	service := newService(journal)
	now := time.Now().UTC().Truncate(time.Microsecond)
	source := []masterDomain.CategorySource{{ID: "11111111-1111-4111-8111-111111111111", Code: "CAB", Name: "Cables", Active: true, Version: 1, CreatedAt: now, UpdatedAt: now, Data: masterDomain.CategorySourceData{Kind: "category"}}, {ID: "22222222-2222-4222-8222-222222222222", Code: "LTPC", Name: "LT Power Cable", Active: false, Version: 1, CreatedAt: now, UpdatedAt: now, Data: masterDomain.CategorySourceData{Kind: "type", CategoryCode: "CAB", UOM: "Meter", FamilyCode: "LTPC", Revision: 1, Fields: []masterDomain.CategoryField{{Code: "SZ", Label: "Size", Required: true, Options: []masterDomain.CategoryOption{{Code: "4SQMM", Label: "4 sqmm"}}}}}}}
	if _, e := service.Import(ctx, "reader", source); !errors.Is(e, masterDomain.CategoryForbidden) {
		t.Fatal("reader imported", e)
	}
	if _, e := newService(func(database.Connection) events.Writer { return failingCategoryJournal{} }).Import(ctx, "admin", source); e == nil {
		t.Fatal("audit failure committed")
	}
	empty, e := service.List(ctx, "reader")
	if e != nil || len(empty.Categories) != 0 || len(empty.Units) != 0 || len(empty.Subcategories) != 0 {
		t.Fatal("rollback incomplete", e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Go(func() { _, e := service.Import(ctx, "admin", source); errs <- e })
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("concurrent identical import", e)
		}
	}
	repeated, e := service.Import(ctx, "admin", source)
	if e != nil || repeated != (masterDomain.CategoryResult{}) {
		t.Fatal("repeat writes", e)
	}
	got, e := service.List(ctx, "reader")
	if e != nil || len(got.Categories) != 1 || len(got.Subcategories) != 1 || len(got.Units) != 1 {
		t.Fatal("count mismatch", e)
	}
	v := got.Subcategories[0]
	if v.CategoryID != got.Categories[0].ID || v.UnitID != got.Units[0].ID || v.Active || *v.CreatedBy != actorID || v.ID == source[1].ID || v.Configuration.Fields[0].Options[0].Code != "4SQMM" {
		t.Fatal("relationship, source status, actor or configuration lost")
	}
	var eventsCount int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='category_catalogue'").Scan(&eventsCount); e != nil || eventsCount != 3 {
		t.Fatal("events mismatch", eventsCount, e)
	}
	var provenance masterDomain.CategorySource
	var raw []byte
	if e = pool.QueryRow(ctx, "SELECT legacy_source FROM category_subcategories WHERE id=$1", v.ID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if e = masterDTO.UnmarshalJSON(raw, &provenance); e != nil || provenance.ID != source[1].ID {
		t.Fatal("provenance missing", e)
	}
	changed := append([]masterDomain.CategorySource(nil), source...)
	changed[1].Name = "Changed source"
	changed = append(changed, masterDomain.CategorySource{ID: "33333333-3333-4333-8333-333333333333", Code: "EAR", Name: "Earthing", Active: true, Version: 1, CreatedAt: now, UpdatedAt: now, Data: masterDomain.CategorySourceData{Kind: "category"}})
	extra := source[1]
	extra.ID = "44444444-4444-4444-8444-444444444444"
	extra.Code = "CEE"
	extra.Name = "Earthing fixture"
	extra.Data.CategoryCode = "EAR"
	extra.Data.FamilyCode = "CEE"
	changed = append(changed, extra)
	if _, e = service.Import(ctx, "admin", changed); !errors.Is(e, masterDomain.CategoryConflict) {
		t.Fatal("source conflict accepted", e)
	}
	got, e = service.List(ctx, "reader")
	if e != nil || len(got.Categories) != 1 {
		t.Fatal("conflict partially committed", e)
	}
	router := chi.NewRouter()
	masterHTTP.RegisterCategoryRoutes(router, service)
	payload, _ := masterDTO.MarshalJSON(map[string]any{"records": source})
	call := func(method, path, token, body string) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w.Code
	}
	for _, c := range []struct {
		method, path, token, body string
		status                    int
	}{{"GET", "/api/v1/categories", "", "", 401}, {"GET", "/api/v1/categories", "denied", "", 403}, {"GET", "/api/v1/categories", "reader", "", 200}, {"POST", "/api/v1/categories/import", "reader", string(payload), 403}, {"POST", "/api/v1/categories/import", "admin", string(payload), 200}, {"POST", "/api/v1/categories/import", "admin", `{"records":[],"actor_id":"forged"}`, 422}, {"POST", "/api/v1/categories/import", "admin", string(payload) + "{}", 422}, {"DELETE", "/api/v1/categories", "admin", "", http.StatusMethodNotAllowed}, {"POST", "/api/v1/categories/import", "admin", `{"records":` + string(bytes.Repeat([]byte(" "), 1<<20)) + `[]}`, 413}} {
		if status := call(c.method, c.path, c.token, c.body); status != c.status {
			t.Fatal("HTTP contract", c.method, c.status, status)
		}
	}
	if _, e = service.Create(ctx, "reader", "unit", masterDomain.CategoryCreate{Code: "SET", Name: "Set", Reason: "Fixture"}); !errors.Is(e, masterDomain.CategoryForbidden) {
		t.Fatal("reader created unit", e)
	}
	before, _ := service.List(ctx, "admin")
	failCreate := newService(func(database.Connection) events.Writer { return failingCategoryJournal{} })
	if _, e = failCreate.Create(ctx, "admin", "unit", masterDomain.CategoryCreate{Code: "FAILED", Name: "Failed", Reason: "Fixture"}); e == nil {
		t.Fatal("creation audit failure committed")
	}
	after, _ := service.List(ctx, "admin")
	if len(before.Units) != len(after.Units) {
		t.Fatal("failed creation leaked unit")
	}
	u, e := service.Create(ctx, "admin", "unit", masterDomain.CategoryCreate{Code: "SET", Name: "Set", Reason: "Fixture"})
	if e != nil {
		t.Fatal(e)
	}
	c, e := service.Create(ctx, "admin", "category", masterDomain.CategoryCreate{Code: "POL", Name: "Poles fixture", UnitID: u.ID, Reason: "Fixture"})
	if e != nil || c.UnitID != u.ID {
		t.Fatal("category unit", e)
	}
	sub, e := service.Create(ctx, "admin", "subcategory", masterDomain.CategoryCreate{Code: "POLE", Name: "Pole fixture", CategoryID: c.ID, Reason: "Fixture"})
	if e != nil || sub.UnitID != u.ID || sub.CategoryID != c.ID || len(sub.Configuration.Fields) != 0 || *sub.CreatedBy != actorID {
		t.Fatal("native subcategory inheritance", e)
	}
	if _, e = service.Create(ctx, "admin", "subcategory", masterDomain.CategoryCreate{Code: "BAD", Name: "Bad unit", CategoryID: c.ID, UnitID: got.Units[0].ID, Reason: "Fixture"}); e == nil {
		t.Fatal("unit override accepted")
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM category_catalogue_changes").Scan(&count)
	if count != 3 {
		t.Fatal("creation history", count)
	}
	if _, e = pool.Exec(ctx, "UPDATE category_catalogue_changes SET reason='tamper'"); e == nil {
		t.Fatal("history mutation accepted")
	}
	if _, e = pool.Exec(ctx, "UPDATE category_subcategories SET unit_id=$1 WHERE id=$2", got.Units[0].ID, sub.ID); e == nil {
		t.Fatal("database allowed category unit mismatch")
	}
	if _, e = service.Create(ctx, "admin", "category", masterDomain.CategoryCreate{Code: "pol", Name: "Another", UnitID: u.ID, Reason: "Fixture"}); e == nil {
		t.Fatal("duplicate code accepted")
	}
	if status := call("POST", "/api/v1/categories/subcategories", "admin", `{"code":"BAD","name":"Bad","category_id":"`+c.ID+`","unit_id":"`+u.ID+`","reason":"Fixture"}`); status != 422 {
		t.Fatal("HTTP unit override", status)
	}
	if status := call("POST", "/api/v1/categories/units", "admin", `{"code":"PAIR","name":"Pair","reason":"Fixture","created_by_id":"forged"}`); status != 422 {
		t.Fatal("HTTP actor override", status)
	}
	if status := call("POST", "/api/v1/categories/units", "admin", `{"code":"PAIR","name":"Pair","reason":"Fixture"}`); status != 201 {
		t.Fatal("HTTP create", status)
	}
	if status := call("POST", "/api/v1/categories/units", "admin", `{"code":"PAIR2","name":"Pair2"}`); status != 422 {
		t.Fatal("missing reason", status)
	}
	if status := call("POST", "/api/v1/categories/units", "admin", `{"code":"PAIR2","name":"`+strings.Repeat("x", 17<<10)+`","reason":"Fixture"}`); status != 413 {
		t.Fatal("creation size limit", status)
	}
	pool.Exec(ctx, "UPDATE catalogue_units SET is_active=false WHERE id=$1", u.ID)
	if _, e = service.Create(ctx, "admin", "subcategory", masterDomain.CategoryCreate{Code: "OFF", Name: "Inactive fixture", CategoryID: c.ID, Reason: "Fixture"}); e == nil {
		t.Fatal("inactive inherited unit allowed")
	}

	concurrentErrors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Go(func() {
			_, e := service.Create(ctx, "admin", "unit", masterDomain.CategoryCreate{Code: "RACE", Name: "Concurrent unit", Reason: "Fixture"})
			concurrentErrors <- e
		})
	}
	wg.Wait()
	close(concurrentErrors)
	success := 0
	failure := 0
	for e := range concurrentErrors {
		if e == nil {
			success++
		} else {
			failure++
		}
	}
	if success != 1 || failure != 1 {
		t.Fatal("duplicate creation race", success, failure)
	}
	if result, e := service.Import(ctx, "admin", source); e != nil || result != (masterDomain.CategoryResult{}) {
		t.Fatal("source reimport after native creation", e, result)
	}

	editorID := "66666666-6666-4666-8666-666666666666"
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'catalogue_editor','editor@example.invalid','fixture',true,false)", editorID)
	identities["editor"] = auth.Identity{ID: editorID, Active: true, Permissions: []string{"settings.categories.read", "settings.categories.write"}}
	editIn := masterDomain.CategoryEdit{Code: "CAB", Name: "Corrected cables", Reason: "Fixture correction", Version: got.Categories[0].Version}
	if _, e = service.Edit(ctx, "reader", "category", got.Categories[0].ID, editIn); !errors.Is(e, masterDomain.CategoryForbidden) {
		t.Fatal("reader edited", e)
	}
	if _, e = failCreate.Edit(ctx, "admin", "category", got.Categories[0].ID, editIn); e == nil {
		t.Fatal("failed correction audit committed")
	}
	corrected, e := service.Edit(ctx, "editor", "category", got.Categories[0].ID, editIn)
	if e != nil || corrected.Version != editIn.Version+1 || *corrected.UpdatedBy != editorID || *corrected.CreatedBy != actorID || corrected.UnitID != got.Categories[0].UnitID {
		t.Fatal("correction actor/version/link", e)
	}
	if _, e = service.Edit(ctx, "editor", "category", corrected.ID, editIn); !errors.Is(e, masterDomain.CategoryStale) {
		t.Fatal("stale correction accepted", e)
	}
	if _, e = service.Import(ctx, "admin", source); !errors.Is(e, masterDomain.CategoryConflict) {
		t.Fatal("source import overwrote corrected name", e)
	}
	var beforeRaw, afterRaw []byte
	if e = pool.QueryRow(ctx, "SELECT before_snapshot,after_snapshot FROM category_catalogue_changes WHERE subject_id=$1 AND action='update'", corrected.ID).Scan(&beforeRaw, &afterRaw); e != nil {
		t.Fatal(e)
	}
	var beforeRecord, afterRecord masterDomain.Category
	masterDTO.UnmarshalJSON(beforeRaw, &beforeRecord)
	masterDTO.UnmarshalJSON(afterRaw, &afterRecord)
	if beforeRecord.Name != got.Categories[0].Name || afterRecord.Name != corrected.Name || *afterRecord.UpdatedBy != editorID {
		t.Fatal("correction history lost values")
	}
	var originalName string
	if e = pool.QueryRow(ctx, "SELECT legacy_source->>'name' FROM categories WHERE id=$1", corrected.ID).Scan(&originalName); e != nil || originalName != source[0].Name {
		t.Fatal("provenance overwritten", e)
	}
	if status := call("PATCH", "/api/v1/categories/categories/"+corrected.ID, "editor", `{"code":"CAB","name":"Forbidden unit","reason":"Fixture","version":2,"unit_id":"`+u.ID+`"}`); status != 422 {
		t.Fatal("structural tamper", status)
	}
	if status := call("PATCH", "/api/v1/categories/categories/"+corrected.ID, "editor", `{"code":"CAB","name":"HTTP correction","reason":"Fixture","version":2}`); status != 200 {
		t.Fatal("HTTP correction", status)
	}
	latest, _ := service.List(ctx, "admin")
	var categoryNow masterDomain.Category
	for _, row := range latest.Categories {
		if row.ID == corrected.ID {
			categoryNow = row
		}
	}
	collision := masterDomain.CategoryEdit{Code: c.Code, Name: "Collision", Reason: "Fixture", Version: categoryNow.Version}
	if _, e = service.Edit(ctx, "admin", "category", corrected.ID, collision); e == nil {
		t.Fatal("duplicate edit accepted")
	}
	raceErrors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Go(func() {
			_, e := service.Edit(ctx, "editor", "category", corrected.ID, masterDomain.CategoryEdit{Code: "CAB", Name: "Race corrected", Reason: "Fixture", Version: categoryNow.Version})
			raceErrors <- e
		})
	}
	wg.Wait()
	close(raceErrors)
	success = 0
	failure = 0
	for e := range raceErrors {
		if e == nil {
			success++
		} else if errors.Is(e, masterDomain.CategoryStale) {
			failure++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || failure != 1 {
		t.Fatal("stale edit race", success, failure)
	}
	for _, test := range []struct {
		kind string
		row  masterDomain.Category
	}{{"unit", got.Units[0]}, {"subcategory", got.Subcategories[0]}} {
		v, e := service.Edit(ctx, "editor", test.kind, test.row.ID, masterDomain.CategoryEdit{Code: test.row.Code, Name: test.row.Name + " corrected", Reason: "Fixture", Version: test.row.Version})
		if e != nil || v.Code != test.row.Code || v.UnitID != test.row.UnitID || v.CategoryID != test.row.CategoryID {
			t.Fatal("linked correction", test.kind, e)
		}
		if test.kind == "subcategory" && v.Configuration.FamilyCode != test.row.Configuration.FamilyCode {
			t.Fatal("configuration identity changed")
		}
	}

}

func TestCategoryUnitMigrationBackfill(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(map[bool]string{false: "consistent", true: "mixed"}[mixed], func(t *testing.T) {
			dsn, pool := owned(t)
			apply(t, dsn)
			ctx := context.Background()
			m, e := schema.Open(ctx, dsn)
			if e != nil {
				t.Fatal(e)
			}
			defer m.Close()
			if e = m.Steps(-(schema.LatestVersion - 14)); e != nil {
				t.Fatal(e)
			}
			category := "11111111-1111-4111-8111-111111111111"
			unit := "22222222-2222-4222-8222-222222222222"
			unit2 := "33333333-3333-4333-8333-333333333333"
			execFixture(t, pool, "INSERT INTO catalogue_units(id,code,name)VALUES($1,'Meter','Meter'),($2,'Nos','Number')", unit, unit2)
			execFixture(t, pool, "INSERT INTO categories(id,code,name)VALUES($1,'CAB','Cables')", category)
			execFixture(t, pool, "INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration)VALUES('44444444-4444-4444-8444-444444444444','LTPC','Cable',$1,$2,'{}')", category, unit)
			if mixed {
				execFixture(t, pool, "INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration)VALUES('55555555-5555-4555-8555-555555555555','BAD','Other cable',$1,$2,'{}')", category, unit2)
			}
			e = m.Up()
			if mixed {
				if e == nil {
					t.Fatal("ambiguous mapping migrated")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			var mapped string
			if e = pool.QueryRow(ctx, "SELECT unit_id::text FROM categories WHERE id=$1", category).Scan(&mapped); e != nil || mapped != unit {
				t.Fatal("base unit backfill", mapped, e)
			}
		})
	}
}
