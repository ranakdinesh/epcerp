package integration

import (
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func verifyNativePurchasePackage(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.IndentsFactory, indent, demand string) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", "projects.purchase_workflow.write", "projects.purchase_workflow.execute", "projects.purchase_workflow.resolve", "work.tasks.view", "work.tasks.read"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'package native fixture','fixture',true)", role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	for _, id := range []string{f.actor, f.outsider} {
		execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", id, role)
	}
	ids["package-manager"] = auth.Identity{ID: f.actor, Active: true, Permissions: codes}
	ids["package-worker"] = auth.Identity{ID: f.outsider, Active: true, Permissions: codes}
	factory := func(c database.Connection) purchaseRepository.WorkflowProviders {
		source := p(c)
		return purchaseRepository.WorkflowProviders{Projects: projectout.NewContextReader(c), Purchase: source.Scope, Access: accessout.NewGate(c, accessProviders(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Indents: purchaseRepository.NewIndentsReader(c), Preparation: purchaseRepository.NewIndentsPreparationReader(c, source)}
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.WorkflowService {
		return purchaseApp.NewWorkflow(purchaseRepository.NewWorkflowUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	s := build(purchaseJournal)
	if _, e := s.Listing(ctx, "package-worker", f.project); e == nil {
		t.Fatal("external worker listed project packages")
	}
	q := purchaseDomain.WorkflowCreate{IndentID: indent, DemandID: demand, Reason: "Create actual released-line package"}
	w, e := s.Create(ctx, "package-manager", f.project, q)
	if e != nil || len(w.Package.Steps) != 20 || w.Package.Steps[0].State != "source_linked" || w.Package.Version != 1 {
		t.Fatal("native package create", e)
	}
	if _, e = s.Create(ctx, "package-manager", f.project, q); !errors.Is(e, purchaseDomain.WorkflowDuplicate) {
		t.Fatal("duplicate release package", e)
	}
	assign := purchaseDomain.WorkflowAssign{Version: 1, Reason: "Assign external task owner", Rows: []purchaseDomain.WorkflowAssignment{{Number: 5, Team: "Procurement", DoerID: f.outsider, ResolverID: f.actor, PlannedDate: "2026-10-20"}}}
	w, e = s.Assign(ctx, "package-manager", f.project, w.Package.ID, assign)
	if e != nil || w.Package.Version != 2 {
		t.Fatal(e)
	}
	id := w.Package.ID
	verifyPersonalPurchaseTasks(t, f, ids, factory, id, "not_started", false, true)
	listing, e := s.Listing(ctx, "package-manager", f.project, purchaseDomain.WorkflowPageFilter{Page: 1, Size: 20, Search: w.Package.Description, Sort: "code", Direction: "desc"})
	if e != nil || listing.Total != 1 || len(listing.Packages) != 1 || listing.Packages[0].ID != id {
		t.Fatal("native paged package list", e)
	}
	listing, e = s.Listing(ctx, "package-manager", f.project, purchaseDomain.WorkflowPageFilter{Page: 2, Size: 20})
	if e != nil || listing.Total != 1 || len(listing.Packages) != 0 {
		t.Fatal("native second package page", e)
	}

	external, e := s.Workspace(ctx, "package-worker", f.project, id)
	if e != nil || !external.TaskOnly || external.CanAssign || len(external.Package.Steps) != 1 || external.Package.Steps[0].Number != 5 || len(external.History) != 0 || len(external.Members) != 0 {
		t.Fatal("external task-only projection", e)
	}
	if _, e = s.Workspace(ctx, "package-worker", f.otherProject, id); e == nil {
		t.Fatal("task crossed project identity")
	}
	if _, e = s.Command(ctx, "package-worker", f.project, id, 6, purchaseDomain.WorkflowCommand{Version: 2, Action: "start", Reason: "Unassigned step"}); e == nil {
		t.Fatal("unassigned step execution")
	}
	if _, e = s.Command(ctx, "package-worker", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: 2, Action: "complete", Reason: "Fake commercial completion"}); e == nil {
		t.Fatal("completion bypass")
	}
	started, e := s.Command(ctx, "package-worker", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: 2, Action: "start", Reason: "Prepare assigned task", Notes: "RFQ evidence preparation"})
	if e != nil || started.Package.Version != 3 || started.Package.Steps[0].State != "preparing" {
		t.Fatal(e)
	}
	due := v.Now().AddDate(0, 0, 1).Format("2006-01-02")
	blocked, e := s.Command(ctx, "package-worker", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: 3, Action: "block", Reason: "Vendor evidence missing", ResolutionDue: &due})
	if e != nil || blocked.Package.Version != 4 || blocked.Package.Steps[0].BlockedResolverID == nil || *blocked.Package.Steps[0].BlockedResolverID != f.actor {
		t.Fatal(e)
	}
	verifyPersonalPurchaseTasks(t, f, ids, factory, id, "blocked", true, true)
	assign.Version = 4
	if _, e = s.Assign(ctx, "package-manager", f.project, id, assign); e == nil {
		t.Fatal("blocked task reassigned")
	}
	if _, e = s.Command(ctx, "package-worker", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: 4, Action: "resolve", Reason: "Wrong resolver"}); e == nil {
		t.Fatal("doer resolved own blocker")
	}
	resolved, e := s.Command(ctx, "package-manager", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: 4, Action: "resolve", Reason: "Resolved captured issue"})
	if e != nil || resolved.Package.Version != 5 || resolved.Package.Steps[4].State != "preparing" {
		t.Fatal(e)
	}
	verifyPersonalPurchaseTasks(t, f, ids, factory, id, "preparing", false, true)
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_workflow.execute')", role)
	if _, e = s.Command(ctx, "package-worker", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: 5, Action: "save", Reason: "Revoked actor"}); e == nil {
		t.Fatal("revoked global execution accepted")
	}
	verifyPersonalPurchaseTasks(t, f, ids, factory, id, "preparing", false, false)
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_workflow.execute'", role)
	broken := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	if w, e := broken.Command(ctx, "package-worker", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: 5, Action: "save", Reason: "Fail later audit"}); e == nil || w.Package.Version != 0 {
		t.Fatal("failed audit exposed derived work")
	}
	current, e := s.Workspace(ctx, "package-manager", f.project, id)
	if e != nil || current.Package.Version != 5 || len(current.History) != 5 {
		t.Fatal("package audit rollback", e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Command(ctx, "package-worker", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: 5, Action: "save", Reason: "Concurrent source preparation"})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success, stale := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, purchaseDomain.WorkflowStale) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatal("package concurrency", success, stale)
	}

	// Reassignment must change the personal queue immediately without project membership.
	moved, e := s.Assign(ctx, "package-manager", f.project, id, purchaseDomain.WorkflowAssign{Version: 6, Reason: "Reassign actual task", Rows: []purchaseDomain.WorkflowAssignment{{Number: 5, Team: "Purchase", DoerID: f.actor, ResolverID: f.outsider, PlannedDate: "2026-10-20"}}})
	if e != nil {
		t.Fatal(e)
	}
	verifyPersonalPurchaseTasks(t, f, ids, factory, id, "preparing", false, false)
	_, e = s.Assign(ctx, "package-manager", f.project, id, purchaseDomain.WorkflowAssign{Version: moved.Package.Version, Reason: "Restore task owner", Rows: assign.Rows})
	if e != nil {
		t.Fatal(e)
	}
	verifyPersonalPurchaseTasks(t, f, ids, factory, id, "preparing", false, true)
	router := chi.NewRouter()
	purchaseHTTP.WorkflowRegister(router, s)
	url := "/api/v1/projects/" + f.project + "/purchase-workflow/" + id
	for _, check := range []struct {
		token, path string
		want        int
	}{{"package-worker", url, 200}, {"", url, 401}, {"package-worker", url + "?actor_id=forged", 422}, {"package-worker", "/api/v1/projects/" + f.project + "/purchase-workflow", 403}} {
		req := httptest.NewRequest("GET", check.path, nil)
		req.Header.Set("Authorization", "Bearer "+check.token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != check.want {
			t.Fatal("native package HTTP", check.want, response.Code)
		}
	}
	for _, statement := range []string{"UPDATE purchase_work_packages SET source_snapshot='{}'::jsonb,version=version+1 WHERE id=$1", "UPDATE purchase_work_packages SET steps=jsonb_set(steps,'{4,state}','\"approved\"'),version=version+1 WHERE id=$1", "DELETE FROM purchase_work_packages WHERE id=$1", "UPDATE purchase_work_package_events SET reason='rewritten' WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, statement, id); e == nil {
			t.Fatal("mutable package evidence", strings.Split(statement, " ")[0])
		}
	}
	packageVersion := verifyNativePurchaseSourcingRFQ(t, f, ids, p, factory, s, id)
	migration, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer migration.Close()
	if e = migration.Migrate(42); e == nil {
		t.Fatal("populated package rollback removed evidence")
	}

	// Adversarial current source changes must not be silently accepted by task writes.
	execFixture(t, f.pool, "UPDATE project_material_requirements SET version=version+1 WHERE id=$1", demand)
	if _, e = s.Command(ctx, "package-worker", f.project, id, 5, purchaseDomain.WorkflowCommand{Version: packageVersion, Action: "save", Reason: "Changed source"}); e == nil {
		t.Fatal("stale released requirement accepted")
	}
}
