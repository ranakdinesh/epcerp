package integration

import (
	"context"
	"errors"
	migrate "github.com/golang-migrate/migrate/v4"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	out "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	"strings"
	"sync"
	"testing"
)

func accessProviders(c database.Connection) out.Providers {
	users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
	return out.Providers{Projects: projectout.NewContextReader(c), Purchase: purchaseRepository.NewWorkspaceOwnership(c, users, purchaseJournal(c)), Users: users, Bundles: authout.NewBundleDirectory(c)}
}
func TestProjectResponsibilityLiveFinanceScopeRevisionsAndAudit(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	v := out.Values{}
	managerRole, financeRole, otherRole := v.ID(), v.ID(), v.ID()
	managerCodes := []string{"projects.team.read", "projects.team.write", "projects.budgets.read", "projects.budgets.approve"}
	financeCodes := []string{"projects.dashboard.read", "projects.team.read", "projects.budgets.read", "projects.budgets.write", "projects.budgets.approve", "accounts.quotation_approvals.write"}
	role := func(id, name string, codes []string) {
		execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'fixture',true)", id, name)
		for _, code := range codes {
			execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture') ON CONFLICT(code) DO NOTHING", v.ID(), code)
			execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", id, code)
		}
	}
	role(managerRole, "coordinator", managerCodes)
	role(financeRole, "financial specialist", financeCodes)
	role(otherRole, "another eligible role", financeCodes)
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2),($3,$4)", f.manager, managerRole, f.outsider, financeRole)
	identities := projectIdentities{"manager": auth.Identity{ID: f.manager, Active: true, Permissions: managerCodes}, "finance": auth.Identity{ID: f.outsider, Active: true, Permissions: financeCodes}}
	build := func(j func(database.Connection) events.Writer) *app.Service {
		return app.New(out.NewUnitOfWork(f.pool, accessProviders, j), out.NewAuthBridge(identities), v)
	}
	service := build(purchaseJournal)
	if _, e := out.NewGate(f.pool, accessProviders(f.pool)).Owners(ctx, f.project); e == nil {
		t.Fatal("owner directory accepted an independent pool connection")
	}
	check := func(user, project, code string, write bool) error {
		return database.NewManager(f.pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
			_, e := out.NewGate(c, accessProviders(c)).Check(ctx, domain.Actor{ID: user, Active: true, Superuser: true, Permissions: []string{code}}, project, code, write)
			return e
		})
	}
	// Supplied role flags are not trusted: live Auth facts inside the transaction decide.
	if e := check(f.manager, f.project, "projects.budgets.approve", true); !errors.Is(e, domain.Forbidden) {
		t.Fatal("PM implicitly became financial approver", e)
	}
	if e := check(f.outsider, f.project, "projects.budgets.read", false); !errors.Is(e, domain.Forbidden) {
		t.Fatal("global Finance role gave membership", e)
	}
	input := domain.Input{UserID: f.outsider, BundleID: financeRole, Responsibility: "Finance", Permissions: financeCodes, Active: true, Reason: "Appoint project Finance"}
	if _, e := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Save(ctx, "manager", f.project, "", input); e == nil {
		t.Fatal("journal failure ignored")
	}
	var count int
	if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM project_responsibility_profiles").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed profile persisted", count, e)
	}
	profile, e := service.Save(ctx, "manager", f.project, "", input)
	if e != nil || profile.Version != 1 || profile.CreatedBy == nil || *profile.CreatedBy != f.manager {
		t.Fatal("profile attribution", e)
	}
	if e = check(f.outsider, f.project, "projects.budgets.approve", true); e != nil {
		t.Fatal("scoped Finance needs no purchasing permission", e)
	}
	if e = check(f.outsider, f.otherProject, "projects.budgets.approve", true); !errors.Is(e, domain.Forbidden) {
		t.Fatal("profile leaked to other project", e)
	}
	if _, e = service.Save(ctx, "manager", f.project, "", input); !errors.Is(e, domain.Duplicate) {
		t.Fatal("duplicate assignment", e)
	}
	if _, e = service.Save(ctx, "finance", f.project, "", input); !errors.Is(e, domain.Forbidden) {
		t.Fatal("specialist managed team", e)
	}
	options, e := service.Options(ctx, "manager", f.project)
	if e != nil || len(options.Bundles) != 3 {
		t.Fatal("bounded role options", e)
	}
	if _, e = service.Options(ctx, "finance", f.project); !errors.Is(e, domain.Forbidden) {
		t.Fatal("options disclosed without team management", e)
	}
	w, e := service.Workspace(ctx, "finance", f.project)
	if e != nil || w.CanWrite || len(w.Profiles) != 1 {
		t.Fatal("scoped readonly team", e)
	}
	var owners int
	if e = database.NewManager(f.pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
		list, e := out.NewGate(c, accessProviders(c)).Owners(ctx, f.project)
		owners = len(list)
		return e
	}); e != nil || owners != 3 {
		t.Fatal("PM/PE/live Finance owner projection", owners, e)
	}

	readable := func() ([]string, error) {
		var ids []string
		e := database.NewManager(f.pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
			var e error
			ids, e = out.NewReadableProjects(c, accessProviders(c)).Readable(ctx, domain.Actor{ID: f.outsider, Active: true, Permissions: financeCodes})
			return e
		})
		return ids, e
	}
	if ids, e := readable(); e != nil || len(ids) != 1 || ids[0] != f.project {
		t.Fatal("assigned Finance project list", ids, e)
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=false WHERE id=$1", f.outsider)
	if e = check(f.outsider, f.project, "projects.budgets.read", false); !errors.Is(e, domain.Unauthorized) {
		t.Fatal("inactive identity retained scope", e)
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=true WHERE id=$1", f.outsider)
	execFixture(t, f.pool, "UPDATE auth.roles SET is_active=false WHERE id=$1", financeRole)
	if ids, e := readable(); e != nil || len(ids) != 0 {
		t.Fatal("revoked role retained project visibility", ids, e)
	}
	if e = check(f.outsider, f.project, "projects.budgets.read", false); !errors.Is(e, domain.Forbidden) {
		t.Fatal("inactive bundle retained scope", e)
	}
	execFixture(t, f.pool, "UPDATE auth.roles SET is_active=true WHERE id=$1", financeRole)
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.outsider, otherRole)
	execFixture(t, f.pool, "DELETE FROM auth.user_roles WHERE user_id=$1 AND role_id=$2", f.outsider, financeRole)
	if e = check(f.outsider, f.project, "projects.budgets.read", false); !errors.Is(e, domain.Forbidden) {
		t.Fatal("another global role replaced revoked profile bundle", e)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.outsider, financeRole)
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.budgets.approve')", financeRole)
	if e = check(f.outsider, f.project, "projects.budgets.approve", true); !errors.Is(e, domain.Forbidden) {
		t.Fatal("union role bypassed removed scoped permission", e)
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.budgets.approve'", financeRole)
	execFixture(t, f.pool, "UPDATE projects SET lifecycle_status='on_hold' WHERE id=$1", f.project)
	if e = check(f.outsider, f.project, "projects.budgets.write", true); e == nil {
		t.Fatal("held project accepts financial writes")
	}
	if e = check(f.outsider, f.project, "projects.budgets.read", false); e != nil {
		t.Fatal("held project audit unavailable", e)
	}
	execFixture(t, f.pool, "UPDATE projects SET lifecycle_status='draft' WHERE id=$1", f.project)
	// Concurrent edits are serialized by the owning project lock and optimistic version.
	input.Version = profile.Version
	input.Reason = "Profile clarification"
	results := make(chan error, 2)
	var group sync.WaitGroup
	for n := 0; n < 2; n++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, e := service.Save(ctx, "manager", f.project, profile.ID, input)
			results <- e
		}()
	}
	group.Wait()
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
		t.Fatal("version race", success, stale)
	}
	input.Version = 2
	input.Active = false
	input.Reason = "Revoke closed project responsibility"
	execFixture(t, f.pool, "UPDATE projects SET lifecycle_status='closed',is_active=false WHERE id=$1", f.project)
	revoked, e := service.Save(ctx, "manager", f.project, profile.ID, input)
	if e != nil || revoked.Active || revoked.Version != 3 {
		t.Fatal("closed project cannot revoke", e)
	}
	if e = check(f.outsider, f.project, "projects.budgets.read", false); !errors.Is(e, domain.Forbidden) {
		t.Fatal("revoked profile retained scope", e)
	}
	history, e := service.History(ctx, "manager", f.project, profile.ID)
	if e != nil || len(history) != 3 || history[0].Before == nil || history[0].Action != "revoke" {
		t.Fatal("immutable revision history", e)
	}
	if _, e = f.pool.Exec(ctx, "UPDATE project_responsibility_events SET reason='rewritten'"); e == nil {
		t.Fatal("mutable history")
	}
	if _, e = f.pool.Exec(ctx, `INSERT INTO project_responsibility_events(id,profile_id,project_id,action,reason,before_snapshot,after_snapshot)VALUES($1,$2,$3,'revise','Cross-project injection','null','{}')`, v.ID(), profile.ID, f.otherProject); e == nil {
		t.Fatal("history accepted a different project scope")
	}
	if _, e = f.pool.Exec(ctx, "UPDATE project_responsibility_profiles SET user_id=$1,version=version+1", f.actor); e == nil {
		t.Fatal("mutable assigned identity")
	}
	if _, e = f.pool.Exec(ctx, "DELETE FROM project_responsibility_profiles"); e == nil {
		t.Fatal("profile deleted")
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='project_access'").Scan(&count); e != nil || count != 3 {
		t.Fatal("local/central event agreement", count, e)
	}
	m, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Steps(-(schema.LatestVersion - 36)); e == nil || errors.Is(e, migrate.ErrNoChange) || !strings.Contains(e.Error(), "populated project responsibility") {
		t.Fatal("populated responsibility rollback allowed", e)
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM project_responsibility_profiles").Scan(&count); e != nil || count != 1 {
		t.Fatal("rollback destroyed profile", count, e)
	}
}
