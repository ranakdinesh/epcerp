package integration

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	materialout "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	materialapp "github.com/ranakdinesh/epcERP/internal/modules/project_items/application"
	material "github.com/ranakdinesh/epcERP/internal/modules/project_items/contracts"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"

	"net/http/httptest"
	"sync"
	"testing"
)

func TestPurchaseIndentNativeCommitmentsReviewPolicyAndConcurrentRelease(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	permissions := []string{"settings.items.read", "settings.items.write", "settings.purchase_rules.read", "settings.purchase_rules.write", "projects.items.read", "projects.items.write", "projects.purchase_indents.read", "projects.purchase_indents.view", "projects.purchase_indents.write", "projects.purchase_indents.review", "projects.purchase_indents.release", "projects.purchase_workflow.view", "projects.purchase_workflow.read"}
	ids := projectIdentities{"writer": {ID: f.actor, Active: true, Permissions: permissions}, "outsider": {ID: f.outsider, Active: true, Permissions: permissions}}
	item, e := f.itemService(ids).Create(ctx, "writer", f.template, f.make, map[string]string{"C": "3C"}, "MODEL")
	if e != nil {
		t.Fatal(e)
	}
	materialsFactory := func(c database.Connection) materialout.Providers {
		return f.materialProviders(c, purchaseRepository.NewIndentsReader(c))
	}
	materials := materialapp.New(materialout.NewUnitOfWork(f.pool, materialsFactory, purchaseJournal), materialout.NewAuthBridge(ids), materialout.Values{})
	demand, e := materials.Save(ctx, "writer", f.project, "", material.Save{ItemID: item.ID, Quantity: "10", RequiredDate: "2026-10-20", Active: true, Version: 1, Reason: "BO requirement"})
	if e != nil {
		t.Fatal(e)
	}
	factory := func(c database.Connection) purchaseRepository.IndentsProviders {
		p := materialsFactory(c)
		return purchaseRepository.IndentsProviders{Scope: p.Scope, Items: p.Items, Rules: p.Rules, Projects: p.Projects, Clients: f.providers(c).Clients, Demands: materialout.NewDemandPort(c, p, purchaseJournal(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Holds: purchaseRepository.NewHoldsEvaluator(c)}
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.IndentsService {
		return purchaseApp.NewIndents(purchaseRepository.NewIndentsUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	}
	service := build(purchaseJournal)
	input := purchaseDomain.IndentsSave{Version: 1, Reason: "Prepare indent", Data: purchaseDomain.IndentsInput{Currency: "INR", DoerID: f.actor, ResolverID: f.outsider, Documents: "BO reference", Lines: []purchaseDomain.IndentsLine{{DemandID: demand.ID, Quantity: "6", Budget: "1000.00", FullKit: "Reviewed specification"}}}}
	if _, e = service.Save(ctx, "outsider", f.project, "", input); !errors.Is(e, purchaseDomain.IndentsForbidden) {
		t.Fatal("scope", e)
	}
	record, e := service.Save(ctx, "writer", f.project, "", input)
	if e != nil || record.Version != 1 || record.Data.State != "draft" {
		t.Fatal("draft", e)
	}
	workspace, e := service.Workspace(ctx, "writer", f.project)
	if e != nil || len(workspace.Rows) != 1 || len(workspace.Lines) != 1 || workspace.Lines[0].ID != demand.ID || workspace.Lines[0].RemainingQuantity != "10.000" || workspace.Currency != "INR" || !workspace.Permissions["purchase_indents.write"] || !workspace.Permissions["purchase_indents.review"] || !workspace.Permissions["purchase_indents.release"] {
		t.Fatal("native formal indent workspace", e)
	}
	if _, e := service.Workspace(ctx, "outsider", f.project); !errors.Is(e, purchaseDomain.IndentsForbidden) {
		t.Fatal("outsider formal indent workspace", e)
	}
	act := func(r purchaseDomain.IndentsRecord, action string) (purchaseDomain.IndentsRecord, error) {
		return service.Transition(ctx, "writer", f.project, r.ID, purchaseDomain.IndentsAction{Version: r.Version, Action: action, Reason: "Reviewed workflow action", Evidence: "Verified source", LineIDs: []string{demand.ID}})
	}
	if _, e = act(record, "release"); e == nil {
		t.Fatal("unreviewed release")
	}
	record, e = act(record, "submit")
	if e != nil {
		t.Fatal(e)
	}
	record, e = act(record, "review")
	if e != nil {
		t.Fatal(e)
	}
	rules := purchaseApp.NewClientRules(purchaseRepository.NewClientRulesUnitOfWork(f.pool, f.providers, purchaseJournal), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	policy, e := rules.Save(ctx, "writer", f.client, "", purchaseDomain.ClientRulesSave{Version: 1, Active: true, Reason: "Client excluded make", Data: purchaseDomain.ClientRulesData{ScopeCode: "CAB", Mode: "unrestricted", Allowed: []string{}, Excluded: []string{f.make}, EffectiveFrom: "2026-10-01", Evidence: "Client exclusion instruction"}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = act(record, "release"); e == nil {
		t.Fatal("explicit client exclusion ignored")
	}
	if _, e = rules.Save(ctx, "writer", f.client, policy.ID, purchaseDomain.ClientRulesSave{Version: policy.Version, Active: false, Reason: "Client withdrew exclusion", Data: policy.Data}); e != nil {
		t.Fatal(e)
	}
	_, e = rules.Save(ctx, "writer", f.client, "", purchaseDomain.ClientRulesSave{Version: 1, Active: true, Reason: "Client approved make", Data: purchaseDomain.ClientRulesData{ScopeCode: "CAB", Mode: "approved_only", Allowed: []string{f.make}, Excluded: []string{}, EffectiveFrom: "2026-10-01", Evidence: "Client make schedule"}})
	if e != nil {
		t.Fatal(e)
	}
	transition := purchaseDomain.IndentsAction{Version: record.Version, Action: "release", Reason: "Release reviewed quantities", Evidence: "Release authority", LineIDs: []string{demand.ID}}
	if _, e = build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Transition(ctx, "writer", f.project, record.ID, transition); e == nil {
		t.Fatal("audit failure committed release")
	}
	committed, e := purchaseRepository.NewIndentsReader(f.pool).Committed(ctx, f.project, demand.ID)
	if e != nil || committed != "0" {
		t.Fatal("release projection rollback", committed, e)
	}
	second, e := service.Save(ctx, "writer", f.project, "", input)
	if e != nil {
		t.Fatal(e)
	}
	second, e = act(second, "submit")
	if e != nil {
		t.Fatal(e)
	}
	second, e = act(second, "review")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, r := range []purchaseDomain.IndentsRecord{record, second} {
		wg.Add(1)
		go func(r purchaseDomain.IndentsRecord) { defer wg.Done(); _, e := act(r, "release"); results <- e }(r)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("concurrent release oversubscribed", success)
	}
	committed, e = purchaseRepository.NewIndentsReader(f.pool).Committed(ctx, f.project, demand.ID)
	if e != nil || committed != "6.000" {
		t.Fatal("native commitment", committed, e)
	}
	if _, e = materials.Save(ctx, "writer", f.project, demand.ID, material.Save{ItemID: item.ID, Quantity: "5", RequiredDate: demand.RequiredDate, Active: true, Version: demand.Version, Reason: "Reduce below release"}); e == nil {
		t.Fatal("material below native released quantity")
	}
	if _, e = f.pool.Exec(ctx, "UPDATE purchase_indent_releases SET quantity=1"); e == nil {
		t.Fatal("mutable release snapshot")
	}
	readOnly := ids["writer"]
	readOnly.Permissions = []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", "projects.purchase_indents.view", "projects.purchase_indents.read"}
	ids["indent-reader"] = readOnly
	readWorkspace, e := service.Workspace(ctx, "indent-reader", f.project)
	if e != nil || readWorkspace.Permissions["purchase_indents.write"] || readWorkspace.Permissions["purchase_indents.release"] || readWorkspace.Permissions["purchase_indents.review"] || len(readWorkspace.Rows) != 2 || len(readWorkspace.Lines) != 1 || readWorkspace.Lines[0].ReleasedQuantity != "6.000" || readWorkspace.Lines[0].RemainingQuantity != "4.000" {
		t.Fatal("read-only indent source and exact remaining quantities", e)
	}
	router := chi.NewRouter()
	purchaseHTTP.IndentsRegister(router, service)
	url := "/api/v1/projects/" + f.project + "/purchase-indents/workspace"
	for _, check := range []struct {
		token, suffix string
		want          int
	}{{"indent-reader", "", 200}, {"outsider", "", 403}, {"", "", 401}, {"indent-reader", "?actor_id=forged", 422}} {
		req := httptest.NewRequest("GET", url+check.suffix, nil)
		if check.token != "" {
			req.Header.Set("Authorization", "Bearer "+check.token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != check.want {
			t.Fatal("native indent workspace HTTP", check.want, response.Code)
		}
	}
	list, e := service.List(ctx, "writer", f.project)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range list {
		if r.Data.State == "released" {
			verifyNativePurchasePackage(t, f, ids, factory, r.ID, demand.ID)
			break
		}
	}
	for _, r := range list {
		if r.Data.State == "released" {
			history, e := service.History(ctx, "writer", f.project, r.ID)
			if e != nil || len(history) != 4 || history[0].After.Data.Released[0].Source.DemandVersion != demand.Version {
				t.Fatal("release source history", e)
			}
			if _, e = act(r, "withdraw"); e == nil {
				t.Fatal("fully released commitment withdrawn")
			}
		}
	}
}
