package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authmodule "github.com/ranakdinesh/epcERP/internal/modules/auth"
	out "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	app "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventmodule "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	master "github.com/ranakdinesh/epcERP/internal/modules/master"
	governance "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	budgets "github.com/ranakdinesh/epcERP/internal/modules/project_budgets"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects"
	projectdomain "github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	purchase "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms"
	purchasedomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	tenders "github.com/ranakdinesh/epcERP/internal/modules/tender_fms"
	tenderout "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	tenderapp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	tenderdomain "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

func workSources(c database.Connection) map[string]contracts.WorkSource {
	return map[string]contracts.WorkSource{
		"projects": projects.WorkInventory(c), "purchase_fms": purchase.WorkInventory(c, projects.ContextReader(c)), "master": master.WorkInventory(c), "project_access": access.WorkInventory(c, projects.ContextReader(c)), "project_budgets": budgets.WorkInventory(c, projects.ContextReader(c)), "organisation_governance": governance.WorkInventory(c), "tender_fms": tenders.WorkInventory(c),
	}
}
func TestAuthOffboardingManagerApprovalAndAdminFinalization(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	v, e := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: 15 * time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	ids := []string{v.ID(), v.ID(), v.ID(), v.ID(), v.ID()}
	tokens := []string{}
	for i, id := range ids {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,'fixture',true,$4)", id, []string{"off_admin", "off_manager", "off_departing", "off_report", "off_replacement"}[i], []string{"a@fixture.invalid", "m@fixture.invalid", "d@fixture.invalid", "r@fixture.invalid", "p@fixture.invalid"}[i], i == 0)
		token, _, e := v.Issue(id, v.Now()+3600, 0)
		if e != nil {
			t.Fatal(e)
		}
		tokens = append(tokens, token)
	}
	journal := func(c database.Connection) events.Writer { return eventmodule.New().Writer(c) }
	org := app.NewOrganisation(out.NewOrganisationUnitOfWork(pool, journal, workSources), v)
	dep, e := org.SaveDepartment(ctx, tokens[0], "", domain.DepartmentInput{Code: "OPS", Name: "Operations", IsActive: true, Reason: "Initial department setup"})
	if e != nil {
		t.Fatal(e)
	}
	profiles := map[string]domain.Employee{}
	for i, id := range ids {
		manager := ids[1]
		root := false
		if i == 0 {
			manager = ""
			root = true
		}
		if i == 1 {
			manager = ids[0]
		}
		if i == 3 {
			manager = ids[2]
		}
		p, e := org.SaveEmployee(ctx, tokens[0], id, domain.EmployeeInput{FirstName: "Fixture", LastName: "Employee", Mobile: "+91 9000000000", DepartmentID: dep.ID, ManagerID: manager, IsCompanyRoot: root, Reason: "Initial reporting line setup"})
		if e != nil {
			t.Fatal(e)
		}
		profiles[id] = p
	}
	// Completed indent actors and the retained legacy operations identity are evidence,
	// not open work to rewrite during departure.
	clientID, projectID := v.ID(), v.ID()
	execFixture(t, pool, `INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,legacy_source,legacy_source_id,is_active)VALUES($1,'OFF-FIXTURE','Fixture client','fixture client','Fixture client','','','','','','','','','','','','','India','','',true)`, clientID)
	execFixture(t, pool, "INSERT INTO projects(id,project_number,name,normalized_name,client_id,project_manager_id,operations_manager_id,lifecycle_status)VALUES($1,'OFF-PRJ','Offboarding fixture','offboarding fixture',$2,$3,$4,'active')", projectID, clientID, ids[0], ids[2])
	execFixture(t, pool, `INSERT INTO purchase_indents(id,project_id,doer_id,resolver_id,data,is_active,version)VALUES($1,$2,$3,$4,'{"state":"released"}',true,1)`, v.ID(), projectID, ids[2], ids[1])
	execFixture(t, pool, "INSERT INTO purchase_fms_workspaces(project_id,purchase_manager_id)VALUES($1,$2)", projectID, ids[0])
	// Offboarding must preserve the actual source modules' existing eligibility
	// predicates, not merely a convenient subset of their capabilities.
	if e = database.NewManager(pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
		for _, source := range []contracts.WorkSource{projects.WorkInventory(c), purchase.WorkInventory(c, projects.ContextReader(c))} {
			rows, e := source.Inventory(ctx, ids[0])
			if e != nil {
				return e
			}
			for _, w := range rows {
				if w.Kind != "project_manager" && w.Kind != "purchasing_owner" {
					continue
				}
				var variants [][]string
				if w.Kind == "project_manager" {
					variants = [][]string{{}, {"projects.master.read"}, {"projects.master.write"}, {"projects.master.read", "projects.master.write"}}
				} else {
					variants = [][]string{{}, {"projects.purchase_workflow.read"}, {"projects.purchase_workflow.view"}, {"projects.purchase_workflow.view", "projects.purchase_workflow.read"}}
				}
				for _, permissions := range variants {
					want := false
					if w.Kind == "project_manager" {
						want = (projectdomain.Candidate{ID: ids[4], Active: true, Permissions: permissions}).Eligible()
					} else {
						want = (purchasedomain.WorkspaceCandidate{ID: ids[4], Active: true, Permissions: permissions}).Eligible()
					}
					got := domain.EligibleReplacement(domain.User{ID: ids[4], IsActive: true, Roles: []domain.Role{{IsActive: true, Permissions: permissions}}}, ids[2], w)
					if got != want {
						t.Errorf("%s inventory eligibility disagrees with its source policy for %v", w.Kind, permissions)
					}
				}
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if t.Failed() {
		return
	}
	closedProject, archiveRole := v.ID(), v.ID()
	execFixture(t, pool, "INSERT INTO projects(id,project_number,name,normalized_name,client_id,project_manager_id,lifecycle_status)VALUES($1,'OFF-CLOSED','Closed fixture','closed fixture',$2,$3,'closed')", closedProject, clientID, ids[2])
	execFixture(t, pool, "INSERT INTO purchase_fms_workspaces(project_id,purchase_manager_id)VALUES($1,$2)", closedProject, ids[2])
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'archived_scope','Archived scope evidence',true)", archiveRole)
	execFixture(t, pool, `INSERT INTO project_responsibility_profiles(id,project_id,user_id,bundle_id,responsibility,responsibility_key,permission_codes,version)VALUES($1,$2,$3,$4,'Archived responsibility','archived','["projects.budgets.write"]',1)`, v.ID(), closedProject, ids[2], archiveRole)
	execFixture(t, pool, "INSERT INTO project_budgets(project_id,version,revision,state,total,currency,owner_id,notes,pricing_mode,is_active)VALUES($1,1,1,'draft',0,'INR',$2,'','',true)", closedProject, ids[2])
	tenderID := v.ID()
	execFixture(t, pool, "INSERT INTO tender.tender_fms_intakes(id,title,owner_id,initial_sha256)VALUES($1,'Departure tender fixture',$2,$3)", tenderID, ids[2], "fixture-hash")
	// A ready, explicitly permitted recipient; the manager does not gain source permissions.
	role := v.ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'tender_successor','Fixture tender processor',true)", role)
	for _, code := range []string{"tenders.register.view", "tenders.register.read", "tenders.register.write", "tenders.processing.read", "tenders.processing.write"} {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'Fixture permission')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", ids[4], role)
	directory := func(c database.Connection) contracts.AssignmentDirectory {
		return (&authmodule.Module{}).Assignments(c)
	}
	ownership := tenderapp.NewOwnership(tenderout.NewOwnershipUnit(pool, directory, journal), tenderout.NewAuthBridge(projectIdentities{"admin": {ID: ids[0], Active: true, Superuser: true}}), tenderout.Values{})
	svc := app.NewOffboarding(out.NewOffboardingUnitOfWork(pool, journal, workSources), v)
	unconfigured := app.NewOffboarding(out.NewOffboardingUnitOfWork(pool, journal, nil), v)
	if _, e = unconfigured.Create(ctx, tokens[0], ids[2], "Missing source inventory coverage"); e == nil {
		t.Fatal("missing source coverage accepted")
	}
	if _, e = svc.Create(ctx, tokens[0], ids[0], "Reject own departure case"); !errors.Is(e, domain.ErrSelfDeactivation) {
		t.Fatal("self-offboarding accepted", e)
	}
	c, e := svc.Create(ctx, tokens[0], ids[2], "Employee departure handover")
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Items) != 2 || c.Items[0].Responsibility.Kind != "reporting_manager" {
		t.Fatal("live responsibility inventory incomplete", len(c.Items))
	}
	if _, e = svc.Create(ctx, tokens[0], ids[2], "Duplicate departure case"); e == nil {
		t.Fatal("duplicate open case")
	}
	if _, e = svc.Get(ctx, tokens[4], c.ID); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("unrelated employee read departure", e)
	}
	in := domain.HandoverPlan{Version: c.Version, Reason: "Manager reviewed replacement"}
	if _, e = svc.Change(ctx, tokens[0], c.ID, "approve", in); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("administrator bypassed manager approval", e)
	}
	if _, e = svc.Change(ctx, tokens[1], c.ID, "approve", in); e == nil {
		t.Fatal("manager approved abandoned work")
	}
	item := c.Items[0].Responsibility
	in.Selections = []domain.HandoverSelection{{Source: item.Source, Kind: item.Kind, ID: item.ID, ReplacementID: ids[4]}}
	for _, other := range c.Items {
		if other.Responsibility.Source == "tender_fms" {
			w := other.Responsibility
			in.Selections = append(in.Selections, domain.HandoverSelection{Source: w.Source, Kind: w.Kind, ID: w.ID, ReplacementID: ids[4]})
		}
	}
	c, e = svc.Change(ctx, tokens[1], c.ID, "plan", in)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ownership.Handover(ctx, "admin", tenderID, tenderdomain.OwnershipInput{OwnerID: ids[1], Version: 1, Reason: "Ineligible tender replacement"}); e == nil {
		t.Fatal("ineligible tender handover")
	}
	if _, e = ownership.Handover(ctx, "admin", tenderID, tenderdomain.OwnershipInput{OwnerID: ids[4], Version: 1, Reason: "Manager selected tender successor"}); e != nil {
		t.Fatal("source-owned tender handover", e)
	}
	if _, e = ownership.Handover(ctx, "admin", tenderID, tenderdomain.OwnershipInput{OwnerID: ids[0], Version: 1, Reason: "Stale tender ownership change"}); e == nil {
		t.Fatal("stale tender handover accepted")
	}
	if _, e = pool.Exec(ctx, "DELETE FROM tender.tender_fms_owner_changes"); e == nil {
		t.Fatal("tender handover evidence mutable")
	}
	report := profiles[ids[3]]
	_, e = org.SaveEmployee(ctx, tokens[0], ids[3], domain.EmployeeInput{FirstName: report.FirstName, LastName: report.LastName, Mobile: report.Mobile, DepartmentID: dep.ID, ManagerID: ids[4], Version: report.Version, Reason: "Manager selected successor handover"})
	if e != nil {
		t.Fatal(e)
	}
	in = domain.HandoverPlan{Version: c.Version, Reason: "Manager verified completed handover"}
	c, e = svc.Change(ctx, tokens[1], c.ID, "approve", in)
	if e != nil || c.State != "approved" || c.ApprovalByID != ids[1] {
		t.Fatal("manager approval", e)
	}
	in.Version = c.Version
	if _, e = svc.Change(ctx, tokens[1], c.ID, "deactivate", in); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("manager deactivated without administrator permission", e)
	}
	// A recipient that loses a capability required by Tender FMS must not pass clearance.
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id IN(SELECT id FROM auth.permissions WHERE code='tenders.processing.write')", role)
	if _, e = svc.Change(ctx, tokens[0], c.ID, "deactivate", in); e == nil {
		t.Fatal("replacement lost required processing permission but departure completed")
	}
	execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='tenders.processing.write'", role)
	// A source version change after approval invalidates final clearance.
	report.Version++
	_, e = org.SaveEmployee(ctx, tokens[0], ids[3], domain.EmployeeInput{FirstName: "Updated", LastName: report.LastName, Mobile: report.Mobile, DepartmentID: dep.ID, ManagerID: ids[4], Version: report.Version, Reason: "Profile corrected after approval"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Change(ctx, tokens[0], c.ID, "deactivate", in); !errors.Is(e, domain.ErrOffboardingStale) {
		t.Fatal("stale approval accepted", e)
	}
	c, e = svc.Change(ctx, tokens[0], c.ID, "refresh", in)
	if e != nil {
		t.Fatal(e)
	}
	in.Version = c.Version
	c, e = svc.Change(ctx, tokens[1], c.ID, "approve", in)
	if e != nil {
		t.Fatal(e)
	}
	in.Version = c.Version
	// A separately permissioned administrator cannot deactivate the final active
	// recovery administrator, even with a valid reporting-manager approval.
	adminRole := v.ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'departure_admin','Fixture departure administrator',true)", adminRole)
	for _, code := range []string{"admin.offboarding.read", "admin.offboarding.write"} {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'Fixture permission')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", adminRole, code)
	}
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", ids[0], adminRole)
	execFixture(t, pool, "UPDATE auth.users SET is_superuser=false WHERE id=$1", ids[0])
	execFixture(t, pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", ids[2])
	if _, e = svc.Change(ctx, tokens[0], c.ID, "deactivate", in); !errors.Is(e, domain.ErrLastSuperadmin) {
		t.Fatal("final active recovery administrator deactivated", e)
	}
	execFixture(t, pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", ids[0])
	execFixture(t, pool, "UPDATE auth.users SET is_superuser=false WHERE id=$1", ids[2])
	// Audit failure rolls back deactivation and case completion together.
	broken := app.NewOffboarding(out.NewOffboardingUnitOfWork(pool, nil, workSources), v)
	if _, e = broken.Change(ctx, tokens[0], c.ID, "deactivate", in); e == nil {
		t.Fatal("audit failure committed departure")
	}
	var active bool
	pool.QueryRow(ctx, "SELECT is_active FROM auth.users WHERE id=$1", ids[2]).Scan(&active)
	if !active {
		t.Fatal("partial departure commit")
	}
	c, e = svc.Change(ctx, tokens[0], c.ID, "deactivate", in)
	if e != nil || c.State != "completed" {
		t.Fatal("administrator finalization", e)
	}
	pool.QueryRow(ctx, "SELECT is_active FROM auth.users WHERE id=$1", ids[2]).Scan(&active)
	if active {
		t.Fatal("departing account still active")
	}
	if _, e = ownership.Handover(ctx, "admin", tenderID, tenderdomain.OwnershipInput{OwnerID: ids[2], Version: 2, Reason: "Reject inactive owner assignment"}); e == nil {
		t.Fatal("source reassigned work to inactive employee")
	}
	var retained string
	if e = pool.QueryRow(ctx, "SELECT operations_manager_id::text FROM projects WHERE id=$1", projectID).Scan(&retained); e != nil || retained != ids[2] {
		t.Fatal("legacy identity rewritten", e)
	}
	if e = pool.QueryRow(ctx, "SELECT doer_id::text FROM purchase_indents WHERE project_id=$1", projectID).Scan(&retained); e != nil || retained != ids[2] {
		t.Fatal("completed indent evidence rewritten", e)
	}

	if _, e = app.NewSessions(out.NewSessionUnitOfWork(pool), nil, v, domain.Policy{}).Me(ctx, tokens[2]); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("old access survives deactivation", e)
	}
	history, e := svc.History(ctx, tokens[1], c.ID)
	if e != nil || len(history) < 5 || history[0].Action != "deactivate" {
		t.Fatal("manager cannot review completed audit history", e)
	}
	if _, e = svc.History(ctx, tokens[4], c.ID); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("unrelated user read offboarding history", e)
	}
	if _, e = pool.Exec(ctx, "DELETE FROM auth.auth_offboarding_changes"); e == nil {
		t.Fatal("departure evidence mutable")
	}
	users := app.NewUsers(out.NewUserUnitOfWork(pool), nil, v, domain.Policy{})
	if _, e = users.Deactivate(ctx, tokens[0], ids[4]); !errors.Is(e, domain.ErrOffboardingRequired) {
		t.Fatal("legacy deactivation bypass", e)
	}
}
