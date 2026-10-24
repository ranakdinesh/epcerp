package integration

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"

	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	projectsOut "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"testing"
)

type purchaseFixture struct {
	dsn                                                                                                         string
	pool                                                                                                        *pgxpool.Pool
	actor, manager, outsider, client, otherClient, project, otherProject, make, category, subcategory, template string
}

func newPurchaseFixture(t *testing.T) purchaseFixture {
	t.Helper()
	dsn, pool := owned(t)
	apply(t, dsn)
	v := purchaseRepository.Values{}
	f := purchaseFixture{dsn: dsn, pool: pool, actor: v.ID(), manager: v.ID(), outsider: v.ID(), client: v.ID(), otherClient: v.ID(), project: v.ID(), otherProject: v.ID(), make: v.ID(), category: v.ID(), subcategory: v.ID(), template: v.ID()}
	for i, id := range []string{f.actor, f.manager, f.outsider} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,'fixture',true,false)", id, []string{"procurement", "manager", "outsider"}[i], []string{"p@fixture.invalid", "m@fixture.invalid", "o@fixture.invalid"}[i])
	}
	for i, id := range []string{f.client, f.otherClient} {
		name := []string{"Client one", "Client two"}[i]
		execFixture(t, pool, `INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,legacy_source,legacy_source_id,is_active)VALUES($1,$2,$3,$3,$3,'','','','','','','','','','','','','India','','',true)`, id, []string{"F1", "F2"}[i], name)
	}
	for i, id := range []string{f.project, f.otherProject} {
		client := []string{f.client, f.otherClient}[i]
		name := []string{"Procurement fixture one", "Procurement fixture two"}[i]
		execFixture(t, pool, "INSERT INTO projects(id,project_number,name,normalized_name,client_id,project_manager_id,purchase_enabled,currency)VALUES($1,$2,$3,$3,$4,$5,true,'INR')", id, []string{"PRJ-P1", "PRJ-P2"}[i], name, client, f.manager)
		execFixture(t, pool, "INSERT INTO purchase_fms_workspaces(project_id,purchase_manager_id,is_active,version,created_by_id,updated_by_id)VALUES($1,$2,true,1,$2,$2)", id, f.actor)
	}
	execFixture(t, pool, `INSERT INTO catalogue_units(id,code,name)VALUES(md5('proc_unit')::uuid,'M','Meter')`)
	execFixture(t, pool, `INSERT INTO categories(id,code,name,unit_id)VALUES($1,'CAB','Cables',md5('proc_unit')::uuid)`, f.category)
	execFixture(t, pool, `INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration)VALUES($1,'LTPC','LT Power Cable',$2,md5('proc_unit')::uuid,'{"family_code":"LTPC"}')`, f.subcategory, f.category)
	execFixture(t, pool, `INSERT INTO configuration_templates(id,subcategory_id,template_name,model_required,revision,is_active,legacy_definition,source_fingerprint)VALUES($1,$2,'Cable',false,1,true,'{}','fixture')`, f.template, f.subcategory)
	execFixture(t, pool, `INSERT INTO template_fields(id,template_id,field_code,field_name,input_type,mandatory,allow_na,use_in_description,description_order,description_prefix,description_suffix,use_in_duplicate_check,sort_order,is_active)VALUES(md5('proc_field')::uuid,$1,'C','Cores','dropdown',true,false,true,1,'','',true,1,true)`, f.template)
	execFixture(t, pool, `INSERT INTO template_options(id,field_id,option_code,option_label,sort_order,is_active)VALUES(md5('proc_option')::uuid,md5('proc_field')::uuid,'3C','3 core',1,true)`)
	execFixture(t, pool, `INSERT INTO manufacturers(id,code,name,data,is_active)VALUES($1,'POL','Polycab','{"eligible":["CAB"],"notes":""}',true)`, f.make)
	return f
}
func (f purchaseFixture) providers(c database.Connection) purchaseRepository.ClientRulesProviders {
	return purchaseRepository.ClientRulesProviders{Clients: masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c)), Projects: projectsOut.NewContextReader(c), Categories: masterRepository.NewCategoryDirectory(c), Items: masterRepository.NewCategoryDirectory(c), Makes: masterRepository.NewManufacturerItemReader(c)}
}
func TestPublishedPurchaseScopeGateLiveOwnershipAndAvailability(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	permission := "projects.purchase_bo.write"
	actor := purchaseDomain.Principal{ID: f.actor, Active: true, Permissions: []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", permission}}
	check := func(p purchaseDomain.Principal, write bool) error {
		return database.NewManager(f.pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
			v, e := purchaseRepository.NewWorkspaceAccessGate(c, projectsOut.NewContextReader(c), masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))).Check(ctx, p, f.project, permission, write)
			if e == nil && (v.ClientID != f.client || v.ProjectID != f.project) {
				t.Fatal("wrong scoped source")
			}
			return e
		})
	}
	if e := check(actor, true); e != nil {
		t.Fatal(e)
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	if e := check(actor, true); e == nil {
		t.Fatal("disabled purchase execution")
	}
	if e := check(actor, false); e != nil {
		t.Fatal("disabled historical reading", e)
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	actor.ID = f.outsider
	if e := check(actor, true); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("unrelated global writer", e)
	}
	actor.Permissions = append(actor.Permissions, "projects.purchase_portfolio.read")
	if e := check(actor, true); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("portfolio read permitted mutation", e)
	}
	if e := check(actor, false); e != nil {
		t.Fatal("portfolio read denied", e)
	}
	actor = purchaseDomain.Principal{ID: f.actor, Active: true, Permissions: []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", permission}}
	execFixture(t, f.pool, "UPDATE clients SET is_active=false WHERE id=$1", f.client)
	if e := check(actor, true); e == nil {
		t.Fatal("inactive client mutation")
	}
	if e := check(actor, false); e != nil {
		t.Fatal("historical reading denied", e)
	}
	execFixture(t, f.pool, "UPDATE clients SET is_active=true WHERE id=$1", f.client)
	execFixture(t, f.pool, "UPDATE purchase_fms_workspaces SET purchase_manager_id=$1 WHERE project_id=$2", f.outsider, f.project)
	if e := check(actor, true); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("prior owner retained operational write", e)
	}
	actor.ID = f.manager
	if e := check(actor, true); e != nil {
		t.Fatal("explicitly permitted PM", e)
	}
	execFixture(t, f.pool, "UPDATE projects SET lifecycle_status='on_hold' WHERE id=$1", f.project)
	if e := check(actor, true); !errors.Is(e, purchaseDomain.WorkspaceForbidden) {
		t.Fatal("on-hold project mutation", e)
	}
	actor.Active = false
	if e := check(actor, false); !errors.Is(e, purchaseDomain.WorkspaceUnauthorized) {
		t.Fatal("inactive identity", e)
	}
}
