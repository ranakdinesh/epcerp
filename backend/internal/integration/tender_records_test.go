package integration

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	migrate "github.com/golang-migrate/migrate/v4"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	authmodule "github.com/ranakdinesh/epcERP/internal/modules/auth"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	tender "github.com/ranakdinesh/epcERP/internal/modules/tender_fms"
	in "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	tenderapp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNamedTenderFoundationAccessEligibilityAndAtomicity(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	v := out.Values{}
	sales, manager, outsider, bundle, salesBundle := v.ID(), v.ID(), v.ID(), v.ID(), v.ID()
	for i, id := range []string{sales, manager, outsider} {
		name := []string{"sales", "manager", "outsider"}[i]
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser,must_change_password) VALUES($1,$2,$3,'unusable',true,false,false)", id, name, name+"@fixture.invalid")
	}
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'reviewed-manager-designation','Fixture designation',true),($2,'sales-capture','Fixture capture',true)", bundle, salesBundle)
	for _, code := range domain.ManagerPermissions() {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'Fixture only')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", bundle, code)
		if !strings.HasPrefix(code, "tenders.evaluation.") {
			execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", salesBundle, code)
		}
	}
	for _, id := range []string{manager, outsider} {
		execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", id, bundle)
	}
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", sales, salesBundle)
	authmod := &authmodule.Module{}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	bind := func(c database.Connection) contracts.Records {
		return tender.Records(c, authmod.Assignments(c), authmod.Bundles(c), []string{bundle}, journal(c))
	}
	unit := database.NewManager(pool)
	run := func(f func(context.Context, contracts.Records) error) error {
		return unit.Within(ctx, func(ctx context.Context, c database.Connection) error { return f(ctx, bind(c)) })
	}
	if _, e := bind(pool).ManagerOptions(ctx, sales); e == nil {
		t.Fatal("pool accepted instead of shared transaction")
	}
	// Capturing twice is not deduplicated by name, owner or the source fingerprint.
	var one, two domain.Record
	for _, dst := range []*domain.Record{&one, &two} {
		if e := run(func(ctx context.Context, r contracts.Records) error {
			var e error
			*dst, e = r.CreateNamed(ctx, sales, domain.NamedInput{Name: "Independent solar opportunity", ManagerID: manager})
			return e
		}); e != nil {
			t.Fatal(e)
		}
	}
	if one.ID == two.ID || one.SalesCreatorID != sales || one.ManagerID != manager || one.BusinessVersion != 1 || one.ContentRevision != 1 {
		t.Fatal("identity separation", one, two)
	}
	for _, id := range []string{one.ID, two.ID} {
		execFixture(t, pool, "UPDATE tender_fms_intakes SET initial_sha256=$2 WHERE id=$1", id, strings.Repeat("a", 64))
	}
	for _, id := range []string{sales, manager} {
		if e := run(func(ctx context.Context, r contracts.Records) error { _, e := r.Read(ctx, id, one.ID); return e }); e != nil {
			t.Fatal("own scope", e)
		}
	}
	if e := run(func(ctx context.Context, r contracts.Records) error { _, e := r.Read(ctx, outsider, one.ID); return e }); !errors.Is(e, domain.Missing) {
		t.Fatal("cross tender leak", e)
	}
	// Explicit metadata share supports expiration/revocation, without any permission grants.
	grant := domain.ParticipantGrant{UserID: outsider, Active: true, Reason: "Share reviewed Tender metadata"}
	if e := run(func(ctx context.Context, r contracts.Records) error {
		_, e := r.GrantParticipant(ctx, sales, one.ID, grant)
		return e
	}); !errors.Is(e, domain.Missing) {
		t.Fatal("creator granted assignment administration", e)
	}
	if e := run(func(ctx context.Context, r contracts.Records) error {
		_, e := r.GrantParticipant(ctx, manager, one.ID, grant)
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if e := run(func(ctx context.Context, r contracts.Records) error { _, e := r.Read(ctx, outsider, one.ID); return e }); e != nil {
		t.Fatal("participant", e)
	}
	// Named metadata scope cannot enter legacy raw-source or owner APIs.
	legacyIDs := projectIdentities{"manager": {ID: manager, Active: true, Permissions: domain.ManagerPermissions()}, "participant": {ID: outsider, Active: true, Permissions: domain.ManagerPermissions()}}
	legacy := tenderapp.New(out.NewUnitOfWork(pool, func(database.Connection) ports.Documents { return nil }, journal).WithUsers(authmod.Assignments), out.NewAuthBridge(legacyIDs), v)
	for _, token := range []string{"manager", "participant"} {
		if _, e := legacy.Artifact(ctx, token, one.ID, v.ID(), "original"); !errors.Is(e, domain.Missing) {
			t.Fatal("named source legacy bypass", e)
		}
	}
	if rows, e := legacy.List(ctx, "manager"); e != nil || len(rows) != 0 {
		t.Fatal("named records leaked to legacy list", e)
	}
	oldOwnership := tenderapp.NewOwnership(out.NewOwnershipUnit(pool, authmod.Assignments, journal), out.NewAuthBridge(legacyIDs), v)
	if _, _, e := oldOwnership.Options(ctx, "manager", one.ID); !errors.Is(e, domain.Missing) {
		t.Fatal("named ownership legacy bypass", e)
	}
	// Two stale concurrent commands cannot both consume the same grant version.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	grant.ExpectedVersion = 1
	grant.Active = false
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- run(func(ctx context.Context, r contracts.Records) error {
				_, e := r.GrantParticipant(ctx, manager, one.ID, grant)
				return e
			})
		}()
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else {
			var d *domain.Error
			if errors.As(e, &d) && d.Status == 409 {
				conflict++
			} else {
				t.Fatal(e)
			}
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("grant concurrency", success, conflict)
	}
	if e := run(func(ctx context.Context, r contracts.Records) error { _, e := r.Read(ctx, outsider, one.ID); return e }); !errors.Is(e, domain.Missing) {
		t.Fatal("revoked share", e)
	}
	past := time.Now().Add(-time.Hour)
	grant.ExpectedVersion = 2
	grant.Active = true
	grant.ValidUntil = &past
	if e := run(func(ctx context.Context, r contracts.Records) error {
		_, e := r.GrantParticipant(ctx, manager, one.ID, grant)
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if e := run(func(ctx context.Context, r contracts.Records) error { _, e := r.Read(ctx, outsider, one.ID); return e }); !errors.Is(e, domain.Missing) {
		t.Fatal("expired share", e)
	}
	// Stable bundle identity survives a display-name change, but not revocation.
	options := func(want int) {
		t.Helper()
		e := run(func(ctx context.Context, r contracts.Records) error {
			rows, e := r.ManagerOptions(ctx, sales)
			if e == nil && len(rows) != want {
				t.Fatalf("options %d want %d", len(rows), want)
			}
			return e
		})
		if e != nil {
			t.Fatal(e)
		}
	}
	options(2)
	execFixture(t, pool, "UPDATE auth.roles SET name='renamed-approved-bundle' WHERE id=$1", bundle)
	options(2)
	execFixture(t, pool, "UPDATE auth.users SET must_change_password=true WHERE id=$1", manager)
	options(1)
	execFixture(t, pool, "UPDATE auth.users SET must_change_password=false WHERE id=$1", manager)
	execFixture(t, pool, "DELETE FROM auth.user_roles WHERE user_id=$1", outsider)
	options(1)
	execFixture(t, pool, "UPDATE auth.roles SET is_active=false WHERE id=$1", bundle)
	options(0)
	execFixture(t, pool, "UPDATE auth.roles SET is_active=true WHERE id=$1", bundle)
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='tenders.evaluation.write')", bundle)
	options(0)
	if e := run(func(ctx context.Context, r contracts.Records) error {
		_, e := r.CreateNamed(ctx, sales, domain.NamedInput{Name: "Recipient lost capability", ManagerID: manager})
		return e
	}); e == nil {
		t.Fatal("candidate not revalidated on assignment")
	}
	execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='tenders.evaluation.write'", bundle)
	// No configured designation means no arbitrary default, even with permissions.
	if e := unit.Within(ctx, func(ctx context.Context, c database.Connection) error {
		r := tender.Records(c, authmod.Assignments(c), authmod.Bundles(c), nil, journal(c))
		rows, e := r.ManagerOptions(ctx, sales)
		if e == nil && len(rows) != 0 {
			t.Fatal("default designation")
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
	// Live grants, not a stale resolver snapshot, authorize the options endpoint.
	identities := projectIdentities{"sales": auth.Identity{ID: sales, Active: true, Superuser: true}}
	router := chi.NewRouter()
	in.RegisterManagerOptions(router, out.NewManagerOptions(pool, identities, bind))
	request := func(want int) {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/tender-fms/assignment-options", nil)
		req.Header.Set("Authorization", "Bearer sales")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatal(res.Code, res.Body.String())
		}
	}
	request(200)
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='tenders.register.assign')", salesBundle)
	request(403)
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", sales)
	request(401)
	// An active, valid token subject with forced-password credentials gets 403.
	execFixture(t, pool, "UPDATE auth.users SET is_active=true,must_change_password=true WHERE id=$1", sales)
	request(403)
	execFixture(t, pool, "UPDATE auth.users SET must_change_password=false WHERE id=$1", sales)
	// Audit failure rolls back identity plus assignment and record history together.
	var count, before int
	if e := pool.QueryRow(ctx, "SELECT count(*) FROM tender_fms_intakes").Scan(&before); e != nil {
		t.Fatal(e)
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=true WHERE id=$1", sales)
	execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='tenders.register.assign'", salesBundle)
	e := unit.Within(ctx, func(ctx context.Context, c database.Connection) error {
		r := tender.Records(c, authmod.Assignments(c), authmod.Bundles(c), []string{bundle}, failingCategoryJournal{})
		_, e := r.CreateNamed(ctx, sales, domain.NamedInput{Name: "Must roll back", ManagerID: manager})
		return e
	})
	if e == nil {
		t.Fatal("journal failure accepted")
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM tender_fms_intakes").Scan(&count); e != nil || count != before {
		t.Fatal("partial capture", e, count, before)
	}
	// Old source/handover routes cannot treat named-manager ownership as a source grant.
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM tender_fms_intakes WHERE record_kind='legacy'").Scan(&count); e != nil || count != 0 {
		t.Fatal(e)
	}
	for _, q := range []string{"UPDATE tender_fms_manager_assignments SET reason='Tamper with evidence'", "DELETE FROM tender_fms_record_events", "TRUNCATE tender_fms_participant_grants"} {
		if _, e = pool.Exec(ctx, q); e == nil {
			t.Fatal("mutable evidence", q)
		}
	}
	// Database constraints defend the current-manager reference and version floor.
	for _, q := range []string{"UPDATE tender_fms_intakes SET business_version=0 WHERE id=$1", "UPDATE tender_fms_intakes SET owner_id='00000000-0000-0000-0000-000000000000' WHERE id=$1"} {
		if _, e = pool.Exec(ctx, q, one.ID); e == nil {
			t.Fatal("invalid identity state accepted")
		}
	}
	// Current manager inventory blocks offboarding; creator identity remains provenance.
	if e = unit.Within(ctx, func(ctx context.Context, c database.Connection) error {
		work, e := tender.WorkInventory(c).Inventory(ctx, manager)
		if e == nil && len(work) != 2 {
			t.Fatal("manager offboarding inventory", len(work))
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", manager)
	if e = run(func(ctx context.Context, r contracts.Records) error { _, e := r.Read(ctx, manager, one.ID); return e }); !errors.Is(e, domain.Unauthorized) {
		t.Fatal("inactive manager", e)
	}
	// Record reads did not advance business/content revisions.
	if e = pool.QueryRow(ctx, "SELECT business_version FROM tender_fms_intakes WHERE id=$1", one.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("read mutated revision", e, count)
	}
}
func TestNamedTenderMigrationPreservesLegacyAndRefusesEvidenceLoss(t *testing.T) {
	dsn, pool := owned(t)
	ctx := context.Background()
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(84); e != nil {
		t.Fatal(e)
	}
	v := out.Values{}
	owner, creator, id, event := v.ID(), v.ID(), v.ID(), v.ID()
	for i, user := range []string{owner, creator} {
		name := []string{"original-owner", "original-creator"}[i]
		execFixture(t, pool, "INSERT INTO public.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,'unusable',true,false)", user, name, name+"@fixture.invalid")
	}
	execFixture(t, pool, "INSERT INTO tender_fms_intakes(id,title,owner_id,initial_sha256,version,created_by_id,updated_by_id)VALUES($1,'Original filename.pdf',$2,'legacy-hash',7,$3,$2)", id, owner, creator)
	execFixture(t, pool, "INSERT INTO tender_fms_owner_changes(id,intake_id,previous_owner_id,owner_id,version,reason,created_by_id)VALUES($1,$2,$3,$4,7,'Original owner handover',$3)", event, id, creator, owner)
	var original string
	if e = pool.QueryRow(ctx, "SELECT row_to_json(p)::text FROM tender_fms_intakes p WHERE id=$1", id).Scan(&original); e != nil {
		t.Fatal(e)
	}
	if e = m.Migrate(85); e != nil {
		t.Fatal(e)
	}
	var title, kind string
	var creatorNull bool
	if e = pool.QueryRow(ctx, "SELECT legacy_title,record_kind,sales_creator_id IS NULL FROM tender_fms_intakes WHERE id=$1", id).Scan(&title, &kind, &creatorNull); e != nil || title != "Original filename.pdf" || kind != "legacy" || !creatorNull {
		t.Fatal("provenance inferred or changed", e)
	}
	if e = m.Steps(-1); e != nil {
		t.Fatal(e)
	}
	var after string
	if e = pool.QueryRow(ctx, "SELECT row_to_json(p)::text FROM tender_fms_intakes p WHERE id=$1", id).Scan(&after); e != nil || after != original {
		t.Fatal("legacy drift on downgrade", e)
	}
	if e = m.Migrate(85); e != nil {
		t.Fatal(e)
	}
	// A source-history ID survives all upgrades; no replacement intake is created.
	var historyID string
	if e = pool.QueryRow(ctx, "SELECT intake_id::text FROM tender_fms_owner_changes WHERE id=$1", event).Scan(&historyID); e != nil || historyID != id {
		t.Fatal("history lost", e)
	}
	// Reject a named record without its current matching Manager assignment.
	if _, e = pool.Exec(ctx, "INSERT INTO tender_fms_intakes(id,title,owner_id,record_kind,sales_creator_id,manager_assignment_id)VALUES($1,'Invalid named tender',$2,'named',$2,$3)", v.ID(), owner, v.ID()); e == nil {
		t.Fatal("missing manager evidence accepted")
	}
	// Rollback refuses retained participant evidence rather than silently deleting it.
	execFixture(t, pool, "INSERT INTO tender_fms_participant_grants(id,intake_id,user_id,version,is_active,reason,created_by_id)VALUES($1,$2,$3,1,true,'Retained scope evidence',$3)", v.ID(), id, owner)
	if e = m.Steps(-1); e == nil || errors.Is(e, migrate.ErrNoChange) {
		t.Fatal("evidence discarded")
	}
}
