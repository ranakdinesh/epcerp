package integration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"

	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	materialout "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
	"net/http/httptest"
	"strings"
	"testing"
)

// Actual released material and live identities; no financial readiness or approved decision fixture is fabricated.
func verifyNativePurchaseSourcingRFQ(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.IndentsFactory, wp purchaseRepository.WorkflowFactory, ws *purchaseApp.WorkflowService, packageID string) int32 {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_sourcing.read", "projects.purchase_sourcing.write", "projects.purchase_sourcing.approve"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'sourcing native fixture','fixture',true)", role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	for _, id := range []string{f.actor, f.outsider} {
		execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", id, role)
	}
	claims := append([]string{}, ids["package-manager"].Permissions...)
	claims = append(claims, codes...)
	ids["sourcing-manager"] = auth.Identity{ID: f.actor, Active: true, Permissions: claims}
	ids["sourcing-worker"] = auth.Identity{ID: f.outsider, Active: true, Permissions: claims}
	current, e := ws.Workspace(ctx, "package-manager", f.project, packageID)
	if e != nil {
		t.Fatal(e)
	}
	planned := purchaseDomain.SourcingToday(v.Now())
	assigned, e := ws.Assign(ctx, "package-manager", f.project, packageID, purchaseDomain.WorkflowAssign{Version: current.Package.Version, Reason: "Assign actual sourcing preparation", Rows: []purchaseDomain.WorkflowAssignment{{Number: 3, Team: "Procurement", DoerID: f.outsider, ResolverID: f.actor, PlannedDate: planned}, {Number: 4, Team: "Procurement", DoerID: f.outsider, ResolverID: f.actor, PlannedDate: planned}}})
	if e != nil {
		t.Fatal(e)
	}
	vendor := v.ID()
	data, _ := json.Marshal(map[string]any{"qualification": "approved", "category_ids": []string{f.category}, "registration_reference": "Fixture REG", "contact": "Fixture contact", "location": "Fixture site", "notes": "Native master fixture"})
	execFixture(t, f.pool, "INSERT INTO suppliers(id,code,name,data,is_active,version)VALUES($1,'SRCFIX','Native sourcing supplier',$2,true,1)", vendor, data)
	factory := func(c database.Connection) purchasePorts.SourcingContext {
		source := p(c)
		return purchaseRepository.NewSourcingContext(purchaseRepository.SourcingProviders{Projects: source.Projects, Purchase: source.Scope, Access: accessout.NewGate(c, accessProviders(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Work: purchaseRepository.NewWorkflowSourceReader(c, wp(c)), Indents: purchaseRepository.NewIndentsReader(c), Items: source.Items, Suppliers: masterRepository.NewVendorDirectory(c), BO: purchaseRepository.NewBODemandReader(c)})
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.SourcingService {
		return purchaseApp.NewSourcing(purchaseRepository.NewSourcingUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	s := build(purchaseJournal)
	w, e := s.Execute(ctx, "sourcing-manager", f.project, packageID, purchaseDomain.SourcingCommand{Action: "configure", Reason: "Native independent sourcing configuration", ApproverID: f.actor})
	if e != nil || w.Case == nil || w.Case.Version != 1 || len(w.History) != 1 {
		t.Fatal("native sourcing configuration", e)
	}
	sourceID := w.Case.ID
	w, e = s.Execute(ctx, "sourcing-worker", f.project, packageID, purchaseDomain.SourcingCommand{Version: 1, Action: "rfq", Reason: "Native RFQ without invented financial approval", SupplierID: vendor, DueDate: purchaseDomain.SourcingToday(v.Now().AddDate(0, 0, 1)), Terms: "Exact released material"})
	if e != nil || len(w.Case.Offers) != 1 || !w.TaskOnly || len(w.History) != 0 || len(w.Approvers) != 0 {
		t.Fatal("native assigned RFQ", e)
	}
	offer := w.Case.Offers[0].ID
	if _, e = s.Execute(ctx, "sourcing-worker", f.project, packageID, purchaseDomain.SourcingCommand{Version: 2, Action: "rfq", Reason: "Duplicate supplier", SupplierID: vendor, DueDate: planned, Terms: "Duplicate"}); e == nil {
		t.Fatal("duplicate RFQ accepted")
	}
	dispatch := purchaseDomain.SourcingCommand{Version: 2, Action: "dispatch", Reason: "Recorded manual RFQ dispatch", OfferID: offer, SentDate: planned, Channel: "manual", DispatchReference: "RFQ native proof"}
	w, e = s.Execute(ctx, "sourcing-worker", f.project, packageID, dispatch)
	if e != nil || w.Case.Version != 3 || w.Case.Offers[0].State != "sent" {
		t.Fatal("native RFQ dispatch", e)
	}
	yes := true
	quote := purchaseDomain.SourcingCommand{Version: 3, Action: "quote", Reason: "Record received quotation", OfferID: offer, QuoteReference: "SRC-NATIVE", EvidenceReference: "Native evidence reference", ValidUntil: purchaseDomain.SourcingToday(v.Now().AddDate(0, 0, 10)), UnitPrice: "12.3456", Discount: "0", Tax: "0", Freight: "0", Other: "0", PaymentTerms: "Receipt", Warranty: "12 months", Compliant: &yes, ComplianceNotes: "Exact specified item"}
	w, e = s.Execute(ctx, "sourcing-worker", f.project, packageID, quote)
	if e != nil || w.Case.Version != 4 || w.Case.Offers[0].UnitPrice != "12.3456" {
		t.Fatal("native received quotation", e)
	}
	if denied, e := s.Execute(ctx, "sourcing-worker", f.project, packageID, purchaseDomain.SourcingCommand{Version: 4, Action: "propose", Reason: "No financial evidence fabricated", OfferID: offer, LOITerms: "Attempt without approval"}); e == nil || denied.Case != nil {
		t.Fatal("unapproved material financially proposed")
	}
	// Later central audit failure must undo both owned state and local history.
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	if result, e := failed.Execute(ctx, "sourcing-worker", f.project, packageID, purchaseDomain.SourcingCommand{Version: 4, Action: "no_bid", Reason: "Failure after owned writes", OfferID: offer}); e == nil || result.Case != nil {
		t.Fatal("audit rollback exposed sourcing")
	}
	actual, e := s.Workspace(ctx, "sourcing-manager", f.project, packageID)
	if e != nil || actual.Case.Version != 4 || actual.Case.Offers[0].State != "quoted" || len(actual.History) != 4 {
		t.Fatal("sourcing late audit committed", e)
	}
	if _, e = s.Workspace(ctx, "sourcing-worker", f.otherProject, packageID); e == nil {
		t.Fatal("cross-project sourcing exposed")
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_sourcing.write')", role)
	if _, e = s.Execute(ctx, "sourcing-worker", f.project, packageID, purchaseDomain.SourcingCommand{Version: 4, Action: "no_bid", Reason: "Revoked writer", OfferID: offer}); !errors.Is(e, purchaseDomain.SourcingForbidden) {
		t.Fatal("revoked sourcing writer accepted", e)
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_sourcing.write'", role)
	for _, sql := range []string{"UPDATE purchase_sourcing_cases SET source_snapshot='{}',version=version+1 WHERE id=$1", "DELETE FROM purchase_sourcing_cases WHERE id=$1", "UPDATE purchase_sourcing_events SET reason='rewritten' WHERE subject_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, sourceID); e == nil {
			t.Fatal("mutable sourcing provenance", sql)
		}
	}
	// An otherwise consistent write cannot bypass the application-owned audit transaction.
	if _, e = f.pool.Exec(ctx, `UPDATE purchase_sourcing_cases SET version=version+1,data=jsonb_set(data,'{version}',to_jsonb(version+1)) WHERE id=$1`, sourceID); e == nil {
		t.Fatal("Unaudited sourcing write committed")
	}
	// Missing JSON is SQL NULL, not JSON null; neither is selection evidence.
	if _, e = f.pool.Exec(ctx, `UPDATE purchase_sourcing_cases SET state='pending',version=version+1,data=jsonb_set(jsonb_set(jsonb_set(jsonb_set(data-'decision_snapshot','{version}',to_jsonb(version+1)),'{state}','"pending"'),'{selected_offer_id}',to_jsonb($2::text)),'{proposed_by_id}',to_jsonb($3::text)) WHERE id=$1`, sourceID, offer, f.outsider); e == nil {
		t.Fatal("Missing selection evidence committed")
	}
	router := chi.NewRouter()
	purchaseHTTP.SourcingRegister(router, s)
	url := "/api/v1/projects/" + f.project + "/purchase-workflow/" + packageID + "/sourcing"
	for _, check := range []struct {
		token, path string
		status      int
	}{{"sourcing-worker", url, 200}, {"", url, 401}, {"sourcing-worker", url + "?actor_id=forged", 422}} {
		r := httptest.NewRequest("GET", check.path, nil)
		r.Header.Set("Authorization", "Bearer "+check.token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		if response.Code != check.status {
			t.Fatal("native sourcing HTTP", check.status, response.Code, strings.TrimSpace(response.Body.String()))
		}
	}
	return assigned.Package.Version
}

// Uses actual completed BO, released indent, saved quotation, vendor verification,
// approved budget and delegated Finance decision; no financial source is fabricated.
func verifyNativePurchaseSourcingLOI(t *testing.T, f purchaseFixture, ids projectIdentities, boID, lineID, vendor string, approved purchaseContracts.QuotationApprovalsAttempt, financeFactory purchaseRepository.QuotationApprovalsFactory, direct bool, quantity string, hooks ...func(nativeSourcingPrepared)) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	var demand string
	if e := f.pool.QueryRow(ctx, "SELECT demand_id::text FROM purchase_bo_lines WHERE bo_id=$1 AND id=$2", boID, lineID).Scan(&demand); e != nil {
		t.Fatal(e)
	}
	role := v.ID()
	codes := []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", "projects.purchase_workflow.write", "projects.purchase_workflow.execute", "projects.purchase_workflow.resolve", "projects.purchase_sourcing.read", "projects.purchase_sourcing.write", "projects.purchase_sourcing.approve"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'fixture',true)", role, "native LOI scope "+v.ID())
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	ids["loi-writer"] = auth.Identity{ID: f.actor, Active: true, Permissions: codes}
	// Existing recovery administrator remains an independent approver, never the proposer.
	ids["loi-manager"] = ids["budget-manager"]
	rules := purchaseApp.NewClientRules(purchaseRepository.NewClientRulesUnitOfWork(f.pool, f.providers, purchaseJournal), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	ruleRows, err := rules.List(ctx, "loi-manager", f.client)
	if err != nil {
		t.Fatal(err)
	}
	hasRule := false
	for _, r := range ruleRows {
		hasRule = hasRule || r.Active && r.Data.ScopeCode == "CAB"
	}
	p := func(c database.Connection) purchaseRepository.IndentsProviders {
		x := f.materialProviders(c, purchaseRepository.NewIndentsReader(c))
		return purchaseRepository.IndentsProviders{Scope: x.Scope, Items: x.Items, Rules: x.Rules, Projects: x.Projects, Clients: f.providers(c).Clients, Demands: materialout.NewDemandPort(c, x, purchaseJournal(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Holds: purchaseRepository.NewHoldsEvaluator(c)}
	}
	if quantity == "10" {
		// Reproduce the live approved-vendor path without catalogue assignments.
		// The exact current Finance decision establishes the project item link.
		execFixture(t, f.pool, `UPDATE suppliers SET data=jsonb_set(jsonb_set(data,'{item_ids}','[]'),'{category_ids}','[]') WHERE id=$1`, vendor)
		awardWork := func(c database.Connection) purchaseRepository.WorkflowProviders {
			x := p(c)
			return purchaseRepository.WorkflowProviders{Projects: x.Projects, Purchase: x.Scope, Access: accessout.NewGate(c, accessProviders(c)), Users: x.Users, Indents: purchaseRepository.NewIndentsReader(c), Preparation: purchaseRepository.NewIndentsPreparationReader(c, x)}
		}
		verifyNativeAwardHandoff(t, f, ids, p, awardWork, financeFactory, approved.ID)
	}
	// The approved-group handoff above must work before any make policy exists.
	// The older sourcing lifecycle below also exercises an explicit approved list.
	if !hasRule {
		if _, e := rules.Save(ctx, "loi-manager", f.client, "", purchaseDomain.ClientRulesSave{Version: 1, Active: true, Reason: "Native approved make schedule", Data: purchaseDomain.ClientRulesData{ScopeCode: "CAB", Mode: "approved_only", Allowed: []string{f.make}, Excluded: []string{}, EffectiveFrom: "2026-10-01", Evidence: "Reviewed client schedule"}}); e != nil {
			t.Fatal(e)
		}
	}
	indents := purchaseApp.NewIndents(purchaseRepository.NewIndentsUnitOfWork(f.pool, p, purchaseJournal), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	ir, e := indents.Save(ctx, "loi-manager", f.project, "", purchaseDomain.IndentsSave{Version: 1, Reason: "Release actual approved requirement", Data: purchaseDomain.IndentsInput{Currency: "INR", DoerID: f.manager, ResolverID: f.actor, Documents: "Completed BO", Lines: []purchaseDomain.IndentsLine{{DemandID: demand, Quantity: quantity, Budget: "200.00", FullKit: "Reviewed specification"}}}})
	if e != nil {
		t.Fatal("LOI indent", e)
	}
	for _, action := range []string{"submit", "review", "release"} {
		ir, e = indents.Transition(ctx, "loi-manager", f.project, ir.ID, purchaseDomain.IndentsAction{Version: ir.Version, Action: action, Reason: "Actual native transition", Evidence: "Reviewed exact requirement", LineIDs: []string{demand}})
		if e != nil {
			t.Fatal(action, e)
		}
	}
	wp := func(c database.Connection) purchaseRepository.WorkflowProviders {
		x := p(c)
		return purchaseRepository.WorkflowProviders{Projects: x.Projects, Purchase: x.Scope, Access: accessout.NewGate(c, accessProviders(c)), Users: x.Users, Indents: purchaseRepository.NewIndentsReader(c), Preparation: purchaseRepository.NewIndentsPreparationReader(c, x)}
	}
	ws := purchaseApp.NewWorkflow(purchaseRepository.NewWorkflowUnitOfWork(f.pool, wp, purchaseJournal), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	w, e := ws.Create(ctx, "loi-manager", f.project, purchaseDomain.WorkflowCreate{IndentID: ir.ID, DemandID: demand, Reason: "Native financially approved package"})
	if e != nil {
		t.Fatal(e)
	}
	pkg := w.Package.ID
	today := purchaseDomain.SourcingToday(v.Now())
	rows := []purchaseDomain.WorkflowAssignment{}
	for _, n := range []int{3, 4, 5} {
		rows = append(rows, purchaseDomain.WorkflowAssignment{Number: n, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: today})
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: 1, Reason: "Assign actual purchasing owner", Rows: rows}); e != nil {
		t.Fatal(e)
	}
	factory := func(c database.Connection) purchasePorts.SourcingContext {
		x := p(c)
		return purchaseRepository.NewSourcingContext(purchaseRepository.SourcingProviders{Projects: x.Projects, Purchase: x.Scope, Access: accessout.NewGate(c, accessProviders(c)), Users: x.Users, Work: purchaseRepository.NewWorkflowSourceReader(c, wp(c)), Indents: purchaseRepository.NewIndentsReader(c), Items: x.Items, Suppliers: masterRepository.NewVendorDirectory(c), BO: purchaseRepository.NewBODemandReader(c), Quotes: purchaseRepository.NewQuotationApprovalsApprovedReader(c, financeFactory(c))})
	}
	service := purchaseApp.NewSourcing(purchaseRepository.NewSourcingUnitOfWork(f.pool, factory, purchaseJournal), purchaseRepository.NewAuthBridge(ids), v)
	sw, e := service.Execute(ctx, "loi-manager", f.project, pkg, purchaseDomain.SourcingCommand{Action: "configure", Reason: "Independent selection authority", ApproverID: f.manager})
	if e != nil {
		t.Fatal(e)
	}
	run := func(token string, q purchaseDomain.SourcingCommand) {
		t.Helper()
		q.Version = sw.Case.Version
		sw, e = service.Execute(ctx, token, f.project, pkg, q)
		if e != nil {
			t.Fatal(q.Action, e)
		}
	}
	wantQuoteTotal, wantQuoteTax := "145.74", "22.23"
	if direct {
		wantQuoteTotal, wantQuoteTax = "145.69", "22.22"
	}
	if len(sw.Evidence) != 1 || sw.Evidence[0].ID != approved.ID || sw.Evidence[0].VendorID != vendor || sw.Evidence[0].QuoteReference != approved.Reference || sw.Evidence[0].EvidenceReference != approved.QuoteKey || sw.Evidence[0].Total != wantQuoteTotal || sw.Evidence[0].Price == nil || (!direct && sw.Evidence[0].Price.Rate != "12.35") || (quantity == "10" && (sw.Evidence[0].Price.Tax != wantQuoteTax || sw.Evidence[0].Price.Charges != "0.01")) {
		t.Fatal("Current exact approved-evidence selector", sw.Evidence)
	}
	run("loi-writer", purchaseDomain.SourcingCommand{Action: "rfq", Reason: "Actual vendor RFQ", SupplierID: vendor, DueDate: today, Terms: "Released requirement"})
	offer := sw.Case.Offers[0].ID
	run("loi-writer", purchaseDomain.SourcingCommand{Action: "dispatch", Reason: "Record manual dispatch", OfferID: offer, SentDate: today, Channel: "manual", DispatchReference: "Native RFQ dispatch"})
	yes := true
	// New unit quotation: 10 × 12.35 = 123.50; tax 22.23; freight 0.01.
	// A same gross total does not authorize changing the saved commercial terms.
	original := purchaseDomain.SourcingCommand{Action: "quote", Reason: "Restore approved commercial terms", OfferID: offer, QuoteReference: approved.Reference, EvidenceReference: approved.QuoteKey, ValidUntil: today, UnitPrice: "12.35", Discount: "0", Tax: wantQuoteTax, Freight: "0.01", Other: "0", DeliveryDays: 10, PaymentTerms: "Against receipt", Warranty: "24 months", Compliant: &yes, ComplianceNotes: "Reviewed exact specified make"}
	if quantity != "10" {
		original.Tax = sw.Evidence[0].Price.Tax
		original.Discount = sw.Evidence[0].Price.Discount
		original.Freight = sw.Evidence[0].Price.Charges
	}
	if direct {
		original.PricingMode = "line_total"
		original.LineTotal = sw.Evidence[0].Price.Base
		original.UnitPrice = ""
		if sw.Evidence[0].Price.Mode != "line_total" || (quantity == "10" && sw.Evidence[0].Price.Base != "123.46") {
			t.Fatal("actual approved direct pricing lost", sw.Evidence[0].Price)
		}
	}
	run("loi-writer", original)
	altered := original
	if direct {
		altered.LineTotal = map[string]string{"10": "123.45", "6": "74.07", "4": "49.37", "5": "61.72"}[quantity]
		altered.Other = "0.01"
	} else {
		altered.UnitPrice = "12.3499"
	}
	altered.Reason = "Changed rate with identical rounded gross"
	run("loi-writer", altered)
	if result, e := service.Execute(ctx, "loi-writer", f.project, pkg, purchaseDomain.SourcingCommand{Version: sw.Case.Version, Action: "propose", Reason: "Try altered rate", OfferID: offer, LOITerms: "Same total"}); e == nil || result.Case != nil {
		t.Fatal("Different approved rate accepted despite identical gross")
	}
	run("loi-writer", original)
	altered = original
	altered.Tax = map[string]string{"10": "22.23", "6": "13.34", "4": "8.88", "5": "11.12"}[quantity]
	if !direct {
		if quantity == "10" {
			altered.Tax = "22.24"
		}
		if quantity == "6" {
			altered.Tax = "13.35"
		}
	}
	altered.Freight = map[string]string{"10": "0", "6": "0", "4": "0.01", "5": "0"}[quantity]
	altered.Reason = "Compensating tax and freight at unchanged gross"
	run("loi-writer", altered)
	if result, e := service.Execute(ctx, "loi-writer", f.project, pkg, purchaseDomain.SourcingCommand{Version: sw.Case.Version, Action: "propose", Reason: "Try compensating charges", OfferID: offer, LOITerms: "Same total"}); e == nil || result.Case != nil {
		t.Fatal("Different approved tax accepted despite identical gross")
	}
	run("loi-writer", original)

	if len(hooks) > 0 {
		failed := purchaseApp.NewSourcing(purchaseRepository.NewSourcingUnitOfWork(f.pool, factory, func(database.Connection) events.Writer { return purchaseRejectJournal{} }), purchaseRepository.NewAuthBridge(ids), v)
		hooks[0](nativeSourcingPrepared{service: service, failed: failed, workspace: sw, packageID: pkg, offerID: offer})
		return
	}
	run("loi-writer", purchaseDomain.SourcingCommand{Action: "propose", Reason: "Select approved vendor quotation", OfferID: offer, LOITerms: "Approved commercial terms"})
	if sw.Case.State != "pending" || sw.Case.Decision == nil {
		t.Fatal("Missing actual pending evidence")
	}
	if _, e = service.Execute(ctx, "loi-writer", f.project, pkg, purchaseDomain.SourcingCommand{Version: sw.Case.Version, Action: "decide", Reason: "Self approval", Approve: &yes}); e == nil {
		t.Fatal("Self approval accepted")
	}
	run("loi-manager", purchaseDomain.SourcingCommand{Action: "decide", Reason: "Independent selection review", Approve: &yes})
	failed := purchaseApp.NewSourcing(purchaseRepository.NewSourcingUnitOfWork(f.pool, factory, func(database.Connection) events.Writer { return purchaseRejectJournal{} }), purchaseRepository.NewAuthBridge(ids), v)
	release := purchaseDomain.SourcingCommand{Version: sw.Case.Version, Action: "release", Reason: "Actual manual LOI release", SentDate: today, Channel: "manual", DispatchReference: "Native LOI dispatch"}
	if result, e := failed.Execute(ctx, "loi-writer", f.project, pkg, release); e == nil || result.Case != nil {
		t.Fatal("LOI late audit failure exposed a release")
	}
	run("loi-writer", release)
	if sw.Case.State != "released" || sw.Case.LOIReference == "" || sw.Case.ReleasedAt == nil || len(sw.History) != 11 {
		t.Fatal("Missing durable LOI evidence")
	}
	if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		reader := purchaseRepository.NewSourcingReleasedReader(c)
		fact, err := reader.Released(txctx, f.project, pkg)
		if err != nil {
			return err
		}
		total, err := purchaseDomain.SourcingTotal(sw.Case.Decision.Offer, sw.Case.Source.Quantity)
		if err != nil {
			return err
		}
		if fact == nil || fact.CaseID != sw.Case.ID || fact.LOIReference != sw.Case.LOIReference || fact.Total != total || fact.FinancialReference != sw.Case.Decision.FinancialReference {
			t.Fatal("released downstream authority changed")
		}
		progress, err := purchaseRepository.NewSourcingProgressReader(c).Progress(txctx, f.project, pkg, sw.Source, true)
		if err != nil || !progress.Current || progress.Steps[2].State != "completed" || progress.Snapshot == nil || progress.Snapshot.CaseID != fact.CaseID {
			t.Fatal("actual released LOI progress", err, progress)
		}
		changed := sw.Source
		changed.DemandVersion++
		stale, err := purchaseRepository.NewSourcingProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
		if err != nil || stale.Current || stale.Steps[2].State != "blocked" {
			t.Fatal("obsolete demand earned LOI progress", err, stale)
		}
		foreignProgress, err := purchaseRepository.NewSourcingProgressReader(c).Progress(txctx, f.otherProject, pkg, sw.Source, true)
		if err != nil || foreignProgress.Current || foreignProgress.Snapshot != nil || foreignProgress.Steps[2].State != "not_started" {
			t.Fatal("foreign project leaked LOI progress", err, foreignProgress)
		}
		foreign, err := reader.Released(txctx, f.otherProject, pkg)
		if err != nil {
			return err
		}
		if foreign != nil {
			t.Fatal("released authority crossed projects")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := purchaseRepository.NewSourcingReleasedReader(f.pool).Released(ctx, f.project, pkg); err == nil {
		t.Fatal("unbound released authority accepted")
	}
	if _, err := purchaseRepository.NewSourcingProgressReader(f.pool).Progress(ctx, f.project, pkg, sw.Source, true); err == nil {
		t.Fatal("unbound LOI progress reader accepted pool")
	}
	if !direct && quantity == "10" {
		verifyNativeTechnicalKit(t, f, ids, p, wp, ws, pkg, financeFactory)
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	for _, token := range []string{"loi-manager", "loi-writer"} {
		blocked, err := service.Workspace(ctx, token, f.project, pkg)
		if err != nil || blocked.Case == nil || len(blocked.Actions) != 0 || blocked.Source.Available {
			t.Fatal("blocked project hides sourcing evidence", token, err)
		}
		if token == "loi-manager" && len(blocked.History) != 11 {
			t.Fatal("blocked scoped history disappeared")
		}
	}
	if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		reader := purchaseRepository.NewWorkflowSourceReader(c, wp(c))
		if _, _, err := reader.Source(txctx, f.project, pkg); err == nil {
			t.Fatal("strict mutation source accepted disabled project")
		}
		if _, _, err := reader.Inspect(txctx, f.project, pkg); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := purchaseRepository.NewWorkflowSourceReader(f.pool, wp(f.pool)).Inspect(ctx, f.project, pkg); err == nil {
		t.Fatal("unbound historical projection accepted")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	if _, e = service.Execute(ctx, "loi-manager", f.project, pkg, purchaseDomain.SourcingCommand{Version: sw.Case.Version, Action: "reopen", Reason: "Rewrite released LOI"}); e == nil {
		t.Fatal("Released LOI reopened")
	}
	if _, e = f.pool.Exec(ctx, "UPDATE purchase_sourcing_cases SET version=version+1 WHERE id=$1", sw.Case.ID); e == nil {
		t.Fatal("Released LOI mutated")
	}
	if e = database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		rows, e := purchaseRepository.NewQuotationApprovalsApprovedReader(c, financeFactory(c)).Approved(txctx, f.project, boID)
		if e == nil && len(rows) == 0 {
			t.Fatal("Actual Finance decision disappeared")
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
}

type nativeSourcingPrepared struct {
	service, failed    *purchaseApp.SourcingService
	workspace          purchaseDomain.SourcingWorkspace
	packageID, offerID string
}

func verifyNativePurchaseSourcingCapacity(t *testing.T, f purchaseFixture, ids projectIdentities, boID, lineID, vendor string, approved purchaseContracts.QuotationApprovalsAttempt, factory purchaseRepository.QuotationApprovalsFactory, direct bool) {
	expectedReservation := "72.88"
	if direct {
		expectedReservation = "72.85"
	}
	t.Helper()
	prepared := []nativeSourcingPrepared{}
	capture := func(p nativeSourcingPrepared) { prepared = append(prepared, p) }
	for range 2 {
		verifyNativePurchaseSourcingLOI(t, f, ids, boID, lineID, vendor, approved, factory, direct, "5", capture)
	}
	type result struct {
		index     int
		workspace purchaseDomain.SourcingWorkspace
		err       error
	}
	results := make(chan result, 2)
	proposal := func(p nativeSourcingPrepared) purchaseDomain.SourcingCommand {
		return purchaseDomain.SourcingCommand{Version: p.workspace.Case.Version, Action: "propose", Reason: "Concurrent approved capacity request", OfferID: p.offerID, LOITerms: "Exact scoped approved terms"}
	}
	for i, p := range prepared {
		go func() {
			w, e := p.service.Execute(t.Context(), "loi-writer", f.project, p.packageID, proposal(p))
			results <- result{i, w, e}
		}()
	}
	winner, loser := -1, -1
	for range 2 {
		r := <-results
		if r.err == nil {
			if winner != -1 {
				t.Fatal("concurrent amount approval reused")
			}
			winner = r.index
			prepared[winner].workspace = r.workspace
		} else {
			var invalid *purchaseDomain.SourcingError
			if !errors.As(r.err, &invalid) || invalid.Status != 422 || r.workspace.Case != nil {
				t.Fatal("unexpected capacity failure", r.err)
			}
			loser = r.index
		}
	}
	if winner < 0 || loser < 0 {
		t.Fatal("capacity did not admit exactly one proposal")
	}
	check := func(active int, amount string) {
		t.Helper()
		var n int
		var spent string
		if e := f.pool.QueryRow(t.Context(), "SELECT count(*),coalesce(sum(amount),0)::text FROM purchase_sourcing_allocations WHERE project_id=$1 AND is_active", f.project).Scan(&n, &spent); e != nil || n != active || spent != amount {
			t.Fatal("owning reservation balance", n, spent, e)
		}
	}
	check(1, expectedReservation)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	p := prepared[winner]
	reopen := purchaseDomain.SourcingCommand{Version: p.workspace.Case.Version, Action: "reopen", Reason: "Release pending capacity"}
	if w, e := p.failed.Execute(t.Context(), "loi-writer", f.project, p.packageID, reopen); e == nil || w.Case != nil {
		t.Fatal("failed audit leaked cancelled reservation")
	}
	check(1, expectedReservation)
	if w, e := p.service.Execute(t.Context(), "loi-writer", f.project, p.packageID, reopen); e != nil || w.Case.State != "draft" {
		t.Fatal("owning reopen did not cancel", e)
	}
	check(0, "0")
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	p = prepared[loser]
	w, e := p.service.Execute(t.Context(), "loi-writer", f.project, p.packageID, proposal(p))
	if e != nil {
		t.Fatal("freed capacity unavailable", e)
	}
	check(1, expectedReservation)
	no := false
	w, e = p.service.Execute(t.Context(), "loi-manager", f.project, p.packageID, purchaseDomain.SourcingCommand{Version: w.Case.Version, Action: "decide", Approve: &no, Reason: "Independent rejection releases reservation"})
	if e != nil || w.Case.State != "draft" {
		t.Fatal("rejection failed", e)
	}
	check(0, "0")
	if _, e = f.pool.Exec(t.Context(), "DELETE FROM purchase_sourcing_allocations WHERE project_id=$1", f.project); e == nil {
		t.Fatal("reservation provenance deleted")
	}
	if _, e = f.pool.Exec(t.Context(), "UPDATE purchase_sourcing_allocations SET case_version=case_version+1 WHERE project_id=$1", f.project); e == nil {
		t.Fatal("unaudited reservation update accepted")
	}
}
