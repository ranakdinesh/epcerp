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
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"

	masterHTTP "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestItemSKUConcurrencyEligibilityAndPermanentHistory(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actor := "135bcaf6-e4ec-4809-b7e9-42b94c332891"
	template := "4a325dd1-2a1a-4c5b-a9ec-1b9a9c111111"
	makeID := "4a325dd1-2a1a-4c5b-a9ec-1b9a9c222222"
	execFixture(t, pool, `INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'item_fixture','item@example.invalid','fixture',true,false)`, actor)
	execFixture(t, pool, `INSERT INTO catalogue_units(id,code,name)VALUES(md5('iu')::uuid,'M','Meter')`)
	execFixture(t, pool, `INSERT INTO categories(id,code,name,unit_id)VALUES(md5('ic')::uuid,'CAB','Cables',md5('iu')::uuid)`)
	execFixture(t, pool, `INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration)VALUES(md5('is')::uuid,'LTPC','LT Power Cable',md5('ic')::uuid,md5('iu')::uuid,'{}')`)
	execFixture(t, pool, `INSERT INTO configuration_templates(id,subcategory_id,template_name,model_required,revision,is_active,legacy_definition,source_fingerprint)VALUES($1,md5('is')::uuid,'Cable',false,1,true,'{}','fixture')`, template)
	execFixture(t, pool, `INSERT INTO template_fields(id,template_id,field_code,field_name,input_type,mandatory,allow_na,use_in_description,description_order,description_prefix,description_suffix,use_in_duplicate_check,sort_order,is_active)VALUES(md5('if')::uuid,$1,'C','Cores','dropdown',true,false,true,1,'','',true,1,true)`, template)
	execFixture(t, pool, `INSERT INTO template_options(id,field_id,option_code,option_label,sort_order,is_active)VALUES(md5('io')::uuid,md5('if')::uuid,'3C','3 core',1,true)`)
	execFixture(t, pool, `INSERT INTO manufacturers(id,code,name,data,is_active)VALUES($1,'POL','Polycab','{"eligible":["LTPC"],"notes":""}',true)`, makeID)
	ids := projectIdentities{"writer": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.items.read", "settings.items.write"}}, "reader": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.items.read"}}}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	build := func(j func(database.Connection) events.Writer) *masterApplication.ItemService {
		return masterApplication.NewItemService(masterRepository.NewItemUnitOfWork(pool, func(c database.Connection) masterContracts.CategoryItemReader {
			return masterRepository.NewCategoryDirectory(c)
		}, func(c database.Connection) masterContracts.TemplateItemReader {
			return masterRepository.NewTemplateItemReader(c)
		}, func(c database.Connection) masterContracts.ManufacturerItemReader {
			return masterRepository.NewManufacturerItemReader(c)
		}, j), authInfrastructure.NewAuthBridge(ids), masterRepository.ItemValues{})
	}
	s := build(journal)
	spec := map[string]string{"C": "3C"}
	if _, e := s.Create(ctx, "reader", template, makeID, spec, ""); !errors.Is(e, masterDomain.ItemForbidden) {
		t.Fatal("unauthorized creation", e)
	}
	if _, e := build(func(database.Connection) events.Writer { return failingCategoryJournal{} }).Create(ctx, "writer", template, makeID, spec, ""); e == nil {
		t.Fatal("audit failure committed")
	}
	var count int
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM catalogue_items`).Scan(&count); e != nil || count != 0 {
		t.Fatal("rollback", e)
	}
	results := make(chan masterDomain.Item, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := s.Create(ctx, "writer", template, makeID, spec, " ＭＯＤＥＬ １ ")
			results <- v
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("concurrent create", e)
		}
	}
	var first masterDomain.Item
	for v := range results {
		if first.ID == "" {
			first = v
		}
		if first.ID != v.ID || v.Serial != 1 || v.SKU != "CAB-LTPC-C3C-POL-MODEL1-001" {
			t.Fatal("duplicate/sequence", v.SKU)
		}
	}
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM business_events WHERE source_module='catalogue_items'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate audit", e)
	}
	v, e := s.Create(ctx, "writer", template, makeID, spec, "model2")
	if e != nil || v.Serial != 2 {
		t.Fatal("next serial", e)
	}
	retired, e := s.Change(ctx, "writer", first.ID, "retired", 1, "No longer required")
	if e != nil || retired.Active || retired.SKU != first.SKU {
		t.Fatal("retirement", e)
	}
	again, e := s.Create(ctx, "writer", template, makeID, spec, "model 1")
	if e != nil || again.ID != first.ID || again.Active {
		t.Fatal("retired duplicate silently reactivated", e)
	}
	if _, e = s.Change(ctx, "writer", first.ID, "approved", 1, "restore"); !errors.Is(e, masterDomain.ItemConflict) {
		t.Fatal("stale write", e)
	}
	execFixture(t, pool, `UPDATE manufacturers SET is_active=false WHERE id=$1`, makeID)
	if _, e = s.Change(ctx, "writer", first.ID, "approved", 2, "restore"); e == nil {
		t.Fatal("inactive reactivation")
	}
	execFixture(t, pool, `UPDATE manufacturers SET is_active=true WHERE id=$1`, makeID)
	restored, e := s.Change(ctx, "writer", first.ID, "approved", 2, "Required again")
	if e != nil || restored.SKU != first.SKU || restored.Serial != 1 {
		t.Fatal("permanent SKU", e)
	}
	history, e := s.History(ctx, "reader", first.ID)
	if e != nil || len(history) != 3 || history[0].Actor != actor || history[0].Before == nil {
		t.Fatal("immutable history", e)
	}
	if _, e = pool.Exec(ctx, `UPDATE catalogue_items SET sku='tampered'`); e == nil {
		t.Fatal("identity editable")
	}
	if _, e = pool.Exec(ctx, `DELETE FROM catalogue_item_events`); e == nil {
		t.Fatal("history deletable")
	}
	execFixture(t, pool, `UPDATE categories SET is_active=false WHERE code='CAB'`)
	if _, e = s.Create(ctx, "writer", template, makeID, spec, "model3"); e == nil {
		t.Fatal("inactive category accepted")
	}
	execFixture(t, pool, `UPDATE categories SET is_active=true,code='WIRE' WHERE code='CAB'`)
	after, e := s.Create(ctx, "writer", template, makeID, spec, "model3")
	if e != nil || after.Serial != 1 || !strings.HasPrefix(after.SKU, "WIRE-") {
		t.Fatal("renumbered category", e)
	}
	existing, e := s.Create(ctx, "writer", template, makeID, spec, "model 1")
	if e != nil || existing.SKU != first.SKU {
		t.Fatal("snapshot rewritten", e)
	}

	execFixture(t, pool, `INSERT INTO manufacturers(id,code,name,data,is_active)VALUES(md5('second_make')::uuid,'SECOND','Second make','{"eligible":["WIRE"]}',true)`)
	var secondMake string
	if e = pool.QueryRow(ctx, `SELECT id::text FROM manufacturers WHERE code='SECOND'`).Scan(&secondMake); e != nil {
		t.Fatal(e)
	}
	next, e := s.Create(ctx, "writer", template, secondMake, spec, "model3")
	if e != nil || next.Serial != 2 || next.EngineeringKey != after.EngineeringKey || next.Fingerprint == after.Fingerprint {
		t.Fatal("make identity and shared category sequence", e)
	}
	execFixture(t, pool, `INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration)VALUES(md5('other_type')::uuid,'HTC','HT Cable',md5('ic')::uuid,md5('iu')::uuid,'{}')`)
	execFixture(t, pool, `INSERT INTO configuration_templates(id,subcategory_id,template_name,model_required,revision,is_active,legacy_definition,source_fingerprint)VALUES(md5('other_template')::uuid,md5('other_type')::uuid,'HT',false,1,true,'{}','fixture')`)
	var otherTemplate string
	if e = pool.QueryRow(ctx, `SELECT id::text FROM configuration_templates WHERE template_name='HT'`).Scan(&otherTemplate); e != nil {
		t.Fatal(e)
	}
	other, e := s.Create(ctx, "writer", otherTemplate, secondMake, map[string]string{}, "")
	if e != nil || other.Serial != 3 {
		t.Fatal("category sequence shared across subcategories", e)
	}
	execFixture(t, pool, `INSERT INTO catalogue_items SELECT md5('capacity_fixture')::uuid,configuration_id,manufacturer_id,category_id,'capacity',specifications,snapshot,repeat('a',64),engineering_key,description,'CAPACITY-999',999,status,1,is_active,created_at,updated_at,created_by_id,updated_by_id FROM catalogue_items WHERE id=$1`, after.ID)
	if _, e = s.Create(ctx, "writer", template, makeID, spec, "capacity-new"); e == nil || !strings.Contains(e.Error(), "999") {
		t.Fatal("capacity not enforced", e)
	}
	if _, e = s.Create(ctx, "writer", template, makeID, spec, "model 1"); e != nil {
		t.Fatal("duplicate blocked at capacity", e)
	}
	// Old Go issued models/fingerprints are reused without rewriting evidence.
	execFixture(t, pool, `INSERT INTO catalogue_items SELECT md5('old_unicode')::uuid,configuration_id,manufacturer_id,category_id,'straße',specifications,snapshot,repeat('b',64),engineering_key,description,'LEGACY-STRASSE-998',998,status,1,is_active,created_at,updated_at,created_by_id,updated_by_id FROM catalogue_items WHERE id=$1`, after.ID)
	legacy, err := s.Create(ctx, "writer", template, makeID, spec, "STRASSE")
	if err != nil || legacy.SKU != "LEGACY-STRASSE-998" || legacy.Model != "straße" {
		t.Fatal("legacy normalization reuse", err, legacy.SKU)
	}
	refs, e := s.References(ctx, "reader")
	if e != nil || len(refs.Catalogue) != 3 || len(refs.Manufacturers) != 2 {
		t.Fatal("references", e)
	}
	router := chi.NewRouter()
	masterHTTP.RegisterItemRoutes(router, s)
	for _, body := range []string{`{"configuration_id":"bad","manufacturer_id":"bad","specifications":{}}`, `{"unexpected":true}`, `null`, `{"specifications":{"C":null}}`, `{"model":null}`, `{"specifications":{} } {}`} {
		r := httptest.NewRequest("POST", "/api/v1/catalogue-items", bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer writer")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != 422 {
			t.Fatal("strict input", w.Code)
		}
	}
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(19); e == nil {
		t.Fatal("populated item migration rollback erased issued SKUs")
	}

}
