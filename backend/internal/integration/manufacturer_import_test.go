package integration

import (
	"context"
	"errors"
	authInfrastructure "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"

	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"testing"
	"time"
)

func TestManufacturerNativeDirectoryImportAndAudit(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actor := "135bcaf6-e4ec-4809-b7e9-42b94c332891"
	execFixture(t, pool, `INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'manufacturer_fixture','manufacturer@example.invalid','fixture',true,false)`, actor)
	execFixture(t, pool, `INSERT INTO catalogue_units(id,code,name)VALUES(md5('mu')::uuid,'M','Meter')`)
	execFixture(t, pool, `INSERT INTO categories(id,code,name,unit_id)VALUES(md5('mc')::uuid,'CAB','Cables',md5('mu')::uuid)`)
	execFixture(t, pool, `INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration)VALUES(md5('ms')::uuid,'LTPC','LT Power Cable',md5('mc')::uuid,md5('mu')::uuid,'{}')`)
	identities := projectIdentities{"admin": auth.Identity{ID: actor, Active: true, Superuser: true}, "reader": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.manufacturers.read"}}}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	service := func(j func(database.Connection) events.Writer) *masterApplication.ManufacturerService {
		return masterApplication.NewManufacturerService(masterRepository.NewManufacturerUnitOfWork(pool, func(c database.Connection) masterContracts.CategoryEligibilityReader {
			return masterRepository.NewCategoryDirectory(c)
		}, j), authInfrastructure.NewAuthBridge(identities), masterRepository.ManufacturerSystemValues{})
	}
	s := service(journal)
	active := false
	at := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	in := masterDomain.ManufacturerLegacy{Source: masterDomain.ManufacturerLegacySource{ID: "c222ad10-b1c6-4f18-b2ad-91ad4f06d3f8", Version: 3, CreatedAt: at, UpdatedAt: at}, Code: "BRAND", Name: "Fixture brand", Data: masterDomain.ManufacturerData{Eligible: []string{"LTPC"}, Notes: "Retained reference note"}, Active: &active}
	if _, e := s.Import(ctx, "reader", in); !errors.Is(e, masterDomain.ManufacturerForbidden) {
		t.Fatal("reader imported", e)
	}
	if _, e := service(func(database.Connection) events.Writer { return failingCategoryJournal{} }).Import(ctx, "admin", in); e == nil {
		t.Fatal("failed event committed")
	}
	var count int
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM manufacturers`).Scan(&count); e != nil || count != 0 {
		t.Fatal("rollback", e)
	}
	saved, e := s.Import(ctx, "admin", in)
	if e != nil || saved.ID == in.Source.ID || saved.Active || saved.CreatedBy == nil || *saved.CreatedBy != actor {
		t.Fatal("import result", e)
	}
	again, e := s.Import(ctx, "admin", in)
	if e != nil || again.ID != saved.ID {
		t.Fatal("retry not idempotent", e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM manufacturer_imports WHERE go_id=$1 AND source_snapshot->'source'->>'version'='3' AND created_by_id=$2`, saved.ID, actor).Scan(&count); e != nil || count != 1 {
		t.Fatal("source provenance", e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM business_events WHERE source_module='manufacturers'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("idempotent event count", e)
	}
	choices, e := s.Options(ctx, "reader", "power", nil, 0, "", "")
	if e != nil || choices.Total != 1 || choices.Rows[0].Code != "LTPC" {
		t.Fatal("native choices", e)
	}
	parents, e := s.Options(ctx, "reader", "", nil, 0, "category", "")
	if e != nil || parents.Total != 1 || parents.Rows[0].Kind != "category" {
		t.Fatal("category filter", e)
	}
	children, e := s.Options(ctx, "reader", "", nil, 0, "subcategory", "CAB")
	if e != nil || children.Total != 1 || children.Rows[0].ParentCode != "CAB" {
		t.Fatal("category scoped children", e)
	}
	empty, e := s.Options(ctx, "reader", "", nil, 0, "subcategory", "OTHER")
	if e != nil || empty.Total != 0 {
		t.Fatal("unrelated children leaked", e)
	}
	choices, e = s.Options(ctx, "reader", "", []string{"CAB", "LTPC"}, 0, "", "")
	if e != nil || choices.Total != 2 {
		t.Fatal("native links", e)
	}
	if _, e = s.Profile(ctx, "admin", saved.ID, 1, "Native corrected brand", masterDomain.ManufacturerData{Eligible: []string{"CAB"}, Notes: "Native note"}, "Reviewed correction"); e != nil {
		t.Fatal("native edit", e)
	}
	again, e = s.Import(ctx, "admin", in)
	if e != nil || again.Name != "Native corrected brand" {
		t.Fatal("reimport overwrote native edit", e)
	}
	changed := in
	changed.Name = "Source changed"
	if _, e = s.Import(ctx, "admin", changed); !errors.Is(e, masterDomain.ManufacturerConflict) {
		t.Fatal("source conflict", e)
	}
	execFixture(t, pool, `UPDATE categories SET is_active=false WHERE code='CAB'`)
	if _, e = s.Create(ctx, "admin", "BLOCKED", "Blocked", masterDomain.ManufacturerData{Eligible: []string{"LTPC"}}, "reason"); e == nil {
		t.Fatal("inactive parent eligibility accepted")
	}
	if _, e = pool.Exec(ctx, `UPDATE manufacturer_imports SET source_fingerprint='tampered'`); e == nil {
		t.Fatal("mutable import provenance")
	}
	if _, e = pool.Exec(ctx, `DELETE FROM manufacturers_events`); e == nil {
		t.Fatal("mutable audit history")
	}
}
