package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inboxout "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	financeout "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/adapters/outbound"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"

	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	govout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	govapp "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/application"
	gov "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	accessapp "github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	budgetout "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/outbound"
	budgetapp "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/application"
	budget "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	budgetports "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/ports"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Reuses the native BO → grouping → dispatch → quotation receipt fixture. No
// purchasing/financial workflow snapshots are inserted directly to simulate readiness.
func verifyNativeQuotationApproval(t *testing.T, f purchaseFixture, ids projectIdentities, boID, document, lineID string, quotesFactory purchaseRepository.QuotationsFactory, direct bool, allocationMode string) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.manager)
	execFixture(t, f.pool, "UPDATE projects SET contract_value=1000 WHERE id=$1", f.project)
	ids["budget-manager"] = auth.Identity{ID: f.manager, Active: true, Superuser: true}
	purchaser := ids["quote-writer"]
	purchaser.Permissions = append(purchaser.Permissions, "projects.group_comparisons.write")
	ids["approval-submitter"] = purchaser
	role := v.ID()
	codes := []string{"accounts.quotation_approvals.view", "accounts.quotation_approvals.read", "accounts.quotation_approvals.write"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'quotation Accounts fixture','fixture',true)", role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture') ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.outsider, role)
	ids["approval-accounts"] = auth.Identity{ID: f.outsider, Active: true, Permissions: codes}
	team := accessapp.New(accessout.NewUnitOfWork(f.pool, accessProviders, purchaseJournal), accessout.NewAuthBridge(ids), accessout.Values{})
	if _, e := team.Save(ctx, "budget-manager", f.project, "", access.Input{UserID: f.outsider, BundleID: role, Responsibility: "Accounts quotation approval", Permissions: codes, Active: true, Reason: "Delegated item review"}); e != nil {
		t.Fatal(e)
	}
	cipher, e := documentInfra.NewDocumentCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	evidence := func(c database.Connection) masterContracts.VendorDocumentReader {
		return masterRepository.NewVendorDocumentEvidenceReader(c, cipher, documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)).WithObjects(quotationDocumentObjects)
	}
	directory := func(c database.Connection) masterContracts.VendorDirectory {
		return masterRepository.NewVendorDirectory(c)
	}
	factory := func(c database.Connection) purchaseRepository.QuotationApprovalsProviders {
		return purchaseRepository.QuotationApprovalsProviders{Users: directorUsers(c), Projects: projectout.NewContextReader(c), Purchase: quotesFactory(c).Scope, Accounts: accessout.NewGate(c, accessProviders(c)), Quotes: purchaseRepository.NewQuotationsApprovalReader(c, quotesFactory(c), purchaseRepository.NewIndentGroupsReader(c), purchaseRepository.Values{}), Budgets: budgetout.NewApprovedReader(c, projectout.NewContextReader(c)), Verification: masterRepository.NewVendorVerificationGate(c, directory(c), evidence(c))}
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.QuotationApprovalsService {
		finance := func(c database.Connection) purchasePorts.QuotationApprovalsFinance {
			users := directorUsers(c)
			notices := inboxout.NewWriter(c, govout.NewDirectory(c, users), j(c), inboxout.Values{})
			return purchaseRepository.NewQuotationApprovalsFinanceBridge(financeout.NewQuotationReviewer(c, users, accessout.NewGate(c, accessProviders(c)), j(c), notices, financeout.Values{}))
		}
		return purchaseApp.NewQuotationApprovals(purchaseRepository.NewQuotationApprovalsUnitOfWork(f.pool, factory, j, finance).WithNotices(func(c database.Connection) inbox.AddressedWriter {
			return inboxout.NewAddressedWriter(c, j(c), inboxout.Values{})
		}), purchaseRepository.NewAuthBridge(ids), v).WithPrinter(purchaseRepository.QuotationApprovalPrinter{})
	}
	service := build(purchaseJournal)
	submit := purchaseDomain.QuotationApprovalsCommand{Version: 0, Reason: "Finalise saved quotation", QuoteKey: document + ":2"}
	shortlistMode := direct && (allocationMode == "whole" || allocationMode == "no_budget" || allocationMode == "project_budget_only")
	if shortlistMode {
		submit.Proposals = []purchaseDomain.QuotationApprovalsNomination{{QuoteKey: submit.QuoteKey, BestPrice: true, BestPriceComment: "Lowest cost"}, {QuoteKey: document + ":1", Preferred: true, PreferredComment: "Earlier offer also meets delivery terms"}}
		submit.QuoteKey = ""
	}
	if w, e := service.Workspace(ctx, "approval-accounts", f.project, boID); e != nil || !w.CanReview || w.CanSubmit || w.Ready || w.CanReadBudget || w.CanUpdateBudget {
		t.Fatal("delegated Accounts source", e)
	}
	if w, e := service.Workspace(ctx, "budget-manager", f.project, boID); e != nil || !w.CanReadBudget || !w.CanUpdateBudget {
		t.Fatal("recovery budget permission missing from review", w, e)
	}
	if _, e := service.Workspace(ctx, "approval-accounts", f.otherProject, boID); e == nil {
		t.Fatal("Accounts crossed project scope")
	}

	if shortlistMode {
		var groupID string
		if e = f.pool.QueryRow(ctx, "SELECT group_id FROM purchase_indent_documents WHERE id=$1", document).Scan(&groupID); e != nil {
			t.Fatal(e)
		}
		draft := purchaseDomain.QuotationShortlistDraft{GroupID: groupID, Proposals: submit.Proposals, Comment: "Saved QA submission comment"}
		w, e := service.SaveDraft(ctx, "approval-submitter", f.project, boID, draft)
		if e != nil || len(w.Drafts) != 1 || w.Drafts[0].Version != 1 {
			t.Fatal("save pre-budget draft", e)
		}
		reloaded, e := service.Workspace(ctx, "approval-submitter", f.project, boID)
		if e != nil || len(reloaded.Drafts) != 1 || reloaded.Drafts[0].Comment != draft.Comment {
			t.Fatal("durable draft", e)
		}
		if _, e = service.SaveDraft(ctx, "approval-submitter", f.project, boID, draft); e == nil {
			t.Fatal("stale draft accepted")
		}
		if _, e = service.SaveDraft(ctx, "approval-accounts", f.project, boID, draft); e == nil {
			t.Fatal("Finance edited procurement draft")
		}
		if _, e = service.SaveDraft(ctx, "approval-submitter", f.otherProject, boID, draft); e == nil {
			t.Fatal("draft crossed project")
		}
	}
	if _, e := service.Execute(ctx, "approval-submitter", f.project, boID, "submit", submit); e == nil {
		t.Fatal("unverified vendor submitted")
	}
	var vendor string
	if e = f.pool.QueryRow(ctx, "SELECT vendor_id FROM purchase_indent_documents WHERE id=$1", document).Scan(&vendor); e != nil {
		t.Fatal(e)
	}
	// Complete only the native master fixture before supplier evidence is verified.
	master, _ := masterDTO.MarshalJSON(map[string]any{"qualification": "approved", "category_ids": []string{f.category}, "registration_reference": "Native REG", "contact": "Fixture contact", "location": "Fixture site"})
	execFixture(t, f.pool, "UPDATE suppliers SET data=$2,version=version+1 WHERE id=$1", vendor, master)
	docs := masterApplication.NewVendorDocumentService(masterRepository.NewVendorDocumentUnitOfWork(f.pool, directory, purchaseJournal), documentInfra.NewAuthBridge(ids), cipher, masterRepository.VendorDocumentValues{}).WithObjects(quotationDocumentObjects, "fixture-documents").WithInspector(documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes))
	var img bytes.Buffer
	if e = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
		t.Fatal(e)
	}
	files := []masterDomain.VendorDocumentDocument{}
	for _, kind := range []string{"pan", "cheque", "gst"} {
		file, e := docs.Upload(ctx, "budget-manager", vendor, masterDomain.VendorDocumentInput{DocType: kind, Filename: "evidence.png", Content: img.Bytes()}, "Native evidence")
		if e != nil {
			t.Fatal(e)
		}
		files = append(files, *file)
	}
	verification := masterApplication.NewVendorVerificationService(masterRepository.NewVendorVerificationUnitOfWork(f.pool, directory, evidence, purchaseJournal, directorUsers), documentInfra.NewAuthBridge(ids), masterRepository.VendorVerificationValues{})
	verify := func() {
		t.Helper()
		status, e := verification.Status(ctx, "budget-manager", vendor)
		if e != nil {
			t.Fatal(e)
		}
		q := masterDomain.VendorVerificationCommand{Version: 0, Fingerprint: status.Fingerprint, Reason: "Inspected current vendor evidence", Decisions: []masterDomain.VendorVerificationDecision{}}
		if status.Review != nil {
			q.Version = status.Review.Version
		}
		for _, file := range files {
			q.Decisions = append(q.Decisions, masterDomain.VendorVerificationDecision{DocumentID: file.ID, Decision: "accept", Comment: "Inspected"})
		}
		if _, e := verification.Review(ctx, "budget-manager", vendor, q); e != nil {
			t.Fatal(e)
		}
	}
	verify()

	governance := govapp.New(govout.NewUnitOfWork(f.pool, directorUsers, purchaseJournal), govout.NewAuthBridge(ids), govout.Values{})
	if _, e := governance.Save(ctx, "budget-manager", gov.Input{UserID: f.manager, Active: true, Reason: "Director notification recipient"}); e != nil {
		t.Fatal(e)
	}
	financial := func(c database.Connection) budgetports.Approvals {
		users := directorUsers(c)
		n := inboxout.NewWriter(c, govout.NewDirectory(c, users), purchaseJournal(c), inboxout.Values{})
		return budgetout.NewFinanceBridge(financeout.NewBudgetApprover(c, users, accessout.NewGate(c, accessProviders(c)), purchaseJournal(c), n, financeout.Values{}))
	}
	budgets := budgetapp.New(budgetout.NewFinanceUnitOfWork(f.pool, budgetProviders, purchaseJournal, financial), budgetout.NewAuthBridge(ids), budgetout.Values{})
	head := v.ID()
	data := budget.Data{Total: "1000.00", Currency: "INR", OwnerID: f.manager, Heads: []budget.Head{{ID: head, Name: "Purchase", Kind: "purchase", Amount: "500.00", OwnerID: f.manager, Active: true}}, Allocations: []budget.Allocation{{ID: v.ID(), BOID: boID, LineID: lineID, HeadID: head, Amount: "200.00", OwnerID: f.manager, Active: true}}}

	router := chi.NewRouter()
	purchaseHTTP.QuotationApprovalsRegister(router, service)
	base := "/api/v1/projects/" + f.project + "/purchase-bo/" + boID + "/quotation-approvals"
	unauth := httptest.NewRecorder()
	router.ServeHTTP(unauth, httptest.NewRequest("GET", base, nil))
	if unauth.Code != 401 {
		t.Fatal("unauthenticated approval workspace", unauth.Code)
	}
	body, err := masterDTO.MarshalJSON(submit)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", base+"/submit", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer approval-submitter")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	var pending purchaseDomain.QuotationApprovalsWorkspace
	e = masterDTO.UnmarshalJSON(response.Body.Bytes(), &pending)
	if response.Code != 200 {
		t.Fatal("native submission HTTP", response.Code, response.Body.String())
	}
	if e != nil || pending.Version != 1 || len(pending.Attempts) != 1 || pending.Attempts[0].Lines[0].Budget != nil {
		t.Fatal("submission or financial redaction", e)
	}
	if _, e = service.Execute(ctx, "approval-submitter", f.project, boID, "submit", purchaseDomain.QuotationApprovalsCommand{Version: 1, Reason: "Duplicate pending", QuoteKey: submit.QuoteKey}); e == nil {
		t.Fatal("duplicate pending submission")
	}
	queued, e := service.Queue(ctx, "approval-accounts", 0)
	if e != nil || len(queued.Rows) != 1 || queued.Rows[0].ProjectID != f.project || queued.Rows[0].ProjectName == "" {
		t.Fatal("Finance scoped request queue", queued, e)
	}
	if _, e = service.Queue(ctx, "approval-submitter", 0); e == nil {
		t.Fatal("Procurement read Accounts queue")
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='accounts.quotation_approvals.read')", role)
	queued, e = service.Queue(ctx, "approval-accounts", 0)
	if e != nil || len(queued.Rows) != 0 {
		t.Fatal("revoked live Accounts read exposed request", queued, e)
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='accounts.quotation_approvals.read'", role)
	if shortlistMode {
		var count int
		if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_quotation_work_tasks WHERE bo_id=$1 AND state IN('pending','waiting')", boID).Scan(&count); e != nil || count != 3 {
			t.Fatal("durable Finance/executive tasks", count, e)
		}
		if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM communication_notices WHERE notice_kind='quotation' AND project_id=$1", f.project).Scan(&count); e != nil || count != 2 {
			t.Fatal("submission addressed notice", count, e)
		}
		e = database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			r := purchaseRepository.NewQuotationWorkReader(c, directorUsers(c), quotesFactory(c).Scope, accessout.NewGate(c, accessProviders(c)), projectout.NewContextReader(c))
			rows, e := r.Own(txctx, purchaseDomain.Principal{ID: f.outsider, Active: true, Permissions: codes})
			if e != nil || len(rows) != 1 {
				return errors.New("Finance task source missing")
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
	}
	request := pending.Attempts[0]
	if shortlistMode {
		for _, token := range []string{"approval-submitter", "approval-accounts"} {
			pdf, e := service.Print(ctx, token, f.project, boID, request.GroupID, request.ID, nil, nil, nil)
			if e != nil || len(pdf) < 2000 || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
				t.Fatal("frozen shortlist PDF", e)
			}
		}
	}
	decision := purchaseDomain.QuotationApprovalsCommand{Version: 1, AttemptID: request.ID, Reason: "Accounts checked each item", Decisions: []purchaseDomain.QuotationApprovalsDecision{{LineID: lineID, Decision: "reject", Comment: ""}}}
	if shortlistMode {
		decision.SelectedQuoteKey = request.Proposals[1].QuoteKey
		decision.ReturnAction = "negotiate"
	}
	// Even a real recovery administrator cannot review their own submission.
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.actor)
	ids["submitter-superadmin"] = auth.Identity{ID: f.actor, Active: true, Superuser: true}
	if _, e = service.Execute(ctx, "submitter-superadmin", f.project, boID, "review", decision); e == nil {
		t.Fatal("self-review recovery bypass")
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=false WHERE id=$1", f.actor)
	// Finance receives pre-budget submissions with advisory budget information.
	financePending, e := service.Workspace(ctx, "approval-accounts", f.project, boID)
	if e != nil || financePending.Attempts[0].EffectiveState != "pending_accounts" || financePending.Attempts[0].BudgetIssue == "" {
		t.Fatal("pre-budget request did not remain pending for Finance", e)
	}
	acceptWithoutBudget := decision
	acceptWithoutBudget.ReturnAction = ""
	acceptWithoutBudget.Decisions = []purchaseDomain.QuotationApprovalsDecision{{LineID: lineID, Decision: "accept", Comment: ""}}
	if allocationMode == "no_budget" || allocationMode == "project_budget_only" {
		if _, err := service.Execute(ctx, "approval-accounts", f.project, boID, "review", acceptWithoutBudget); err == nil {
			t.Fatal("Finance approval accepted without an approved budget")
		}
		if allocationMode == "project_budget_only" {
			data.PricingMode = "unit_rates"
			data.Allocations = nil
		}
		if _, err := budgets.Execute(ctx, "budget-manager", f.project, budget.Command{Action: "save", Reason: "Budget prepared from Finance review", Data: &data}); err != nil {
			t.Fatal(err)
		}
		if _, err := budgets.Execute(ctx, "budget-manager", f.project, budget.Command{Action: "approve", Version: 1, Reason: "Budget approved before vendor decision"}); err != nil {
			t.Fatal(err)
		}
		approved, err := service.Execute(ctx, "approval-accounts", f.project, boID, "review", acceptWithoutBudget)
		if err != nil || approved.Version != 2 || !approved.Ready || approved.Attempts[0].State != "approved" || approved.Attempts[0].QuoteKey != acceptWithoutBudget.SelectedQuoteKey || approved.Attempts[0].BudgetRevision != 1 || (allocationMode != "project_budget_only" && approved.Attempts[0].Lines[0].Budget == nil) || (allocationMode == "project_budget_only" && (approved.Attempts[0].Lines[0].Budget != nil || approved.Attempts[0].Lines[0].Difference != nil)) || approved.Attempts[0].BudgetIssue != "" || approved.Attempts[0].Proposals[0].Snapshot.Lines[0].Budget != nil {
			t.Fatal("native single-vendor approval after in-review budget resolution", approved, err)
		}
		var count int
		if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM finance_quotation_decisions WHERE project_id=$1", f.project).Scan(&count); err != nil || count != 1 {
			t.Fatal("budget-resolved decision not persisted", count, err)
		}
		if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM communication_notices WHERE decision_id IN(SELECT id FROM finance_quotation_decisions WHERE project_id=$1)", f.project).Scan(&count); err != nil || count != 1 {
			t.Fatal("budget-resolved Director notice missing", count, err)
		}
		if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_quotation_work_tasks WHERE bo_id=$1 AND state<>'completed'", boID).Scan(&count); err != nil || count != 0 {
			t.Fatal("budget-resolved tasks were not completed", count, err)
		}
		queued, err := service.Queue(ctx, "approval-accounts", 0)
		if err != nil || len(queued.Rows) != 1 || queued.Rows[0].State != "approved" {
			t.Fatal("budget-resolved decision not in completed queue", queued, err)
		}
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			rows, err := purchaseRepository.NewQuotationApprovalsApprovedReader(c, factory(c)).Approved(txctx, f.project, boID)
			if err != nil || len(rows) != 1 {
				t.Fatalf("budget reintroduced at downstream quotation gate: %v", err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, "UPDATE quotation_approval_sets SET version=version+1 WHERE bo_id=$1", boID); err != nil {
			t.Fatal("unchanged decided attempt blocked a later set version", err)
		}
		for _, sql := range []string{
			"UPDATE quotation_approval_sets SET attempts=jsonb_set(attempts,'{0,lines,0,budget}','\"999.00\"'),version=version+1",
			"UPDATE quotation_approval_sets SET attempts=jsonb_set(attempts,'{0,proposals,0,snapshot,total}','\"1.00\"'),version=version+1",
		} {
			if _, err := f.pool.Exec(ctx, sql); err == nil {
				t.Fatal("completed review or frozen proposal was mutable")
			}
		}
		return
	}
	if _, e := budgets.Execute(ctx, "budget-manager", f.project, budget.Command{Action: "save", Reason: "Item budget draft", Data: &data}); e != nil {
		t.Fatal(e)
	}
	if _, e := budgets.Execute(ctx, "budget-manager", f.project, budget.Command{Action: "approve", Version: 1, Reason: "Approve item budget"}); e != nil {
		t.Fatal(e)
	}
	financePending, e = service.Workspace(ctx, "approval-accounts", f.project, boID)
	if e != nil || financePending.Attempts[0].EffectiveState != "pending_accounts" || financePending.Attempts[0].BudgetIssue != "" || financePending.Attempts[0].Lines[0].Budget == nil {
		t.Fatal("Finance budget approval required purchaser resubmission", e)
	}
	wantDifference := "-54.26"
	if direct {
		wantDifference = "-54.31"
	}
	preview, e := service.Preview(ctx, "approval-accounts", f.project, boID, request.QuoteKey)
	if e != nil || len(preview.Lines) != 1 || *preview.Lines[0].Budget != "200.00" || *preview.Lines[0].Difference != wantDifference {
		t.Fatal("native budget comparison", e)
	}
	if _, e = service.Preview(ctx, "approval-submitter", f.project, boID, request.QuoteKey); !errors.Is(e, purchaseDomain.QuotationApprovalsForbidden) {
		t.Fatal("Procurement read Finance preview", e)
	}
	budgetNotices := 1
	checkCounts := func(n int) {
		t.Helper()
		var actual int
		if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM finance_quotation_decisions").Scan(&actual); e != nil || actual != n {
			t.Fatal("Finance quote decisions", actual, n, e)
		}
		if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM communication_notices WHERE decision_id IN(SELECT id FROM finance_quotation_decisions UNION ALL SELECT id FROM finance_budget_decisions)").Scan(&actual); e != nil || actual != n+budgetNotices {
			t.Fatal("atomic notices", actual, n+budgetNotices, e)
		}
	}
	checkCounts(0)
	if _, e := governance.Save(ctx, "budget-manager", gov.Input{UserID: f.manager, Version: 1, Active: false, Reason: "Temporarily revoke Director oversight"}); e != nil {
		t.Fatal(e)
	}
	if w, e := service.Execute(ctx, "approval-accounts", f.project, boID, "review", decision); e == nil || w.Version != 0 {
		t.Fatal("missing Director review escaped rollback", e)
	}
	checkCounts(0)
	if _, e := governance.Save(ctx, "budget-manager", gov.Input{UserID: f.manager, Version: 2, Active: true, Reason: "Restore Director oversight"}); e != nil {
		t.Fatal(e)
	}
	for _, module := range []string{"finance_approvals", "communication_inbox", "quotation_approvals"} {
		broken := build(func(c database.Connection) events.Writer {
			return financeBudgetRejectModule{purchaseJournal(c), module}
		})
		if w, e := broken.Execute(ctx, "approval-accounts", f.project, boID, "review", decision); e == nil || w.Version != 0 {
			t.Fatal("failed review escaped transaction", module, e)
		}
		checkCounts(0)
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id IN(SELECT id FROM auth.permissions WHERE code='accounts.quotation_approvals.write')", role)
	if _, e = service.Execute(ctx, "approval-accounts", f.project, boID, "review", decision); e == nil {
		t.Fatal("revoked Accounts authority reviewed")
	}
	checkCounts(0)
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='accounts.quotation_approvals.write'", role)
	rejected, e := service.Execute(ctx, "approval-accounts", f.project, boID, "review", decision)
	if e != nil || rejected.Version != 2 || rejected.Attempts[0].State != "rejected" || rejected.Attempts[0].DecisionID == nil {
		t.Fatal("native item rejection", e)
	}
	checkCounts(1)
	submit.Version = 2
	pending, e = service.Execute(ctx, "approval-submitter", f.project, boID, "submit", submit)
	if e != nil || pending.Version != 3 || len(pending.Attempts) != 2 {
		t.Fatal("resubmission", e)
	}
	if shortlistMode {
		var active int
		if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_quotation_work_tasks WHERE bo_id=$1 AND state<>'completed'", boID).Scan(&active); e != nil || active != 3 {
			t.Fatal("resubmission retained obsolete return tasks", active, e)
		}
	}
	decision.Version = 3
	decision.AttemptID = pending.Attempts[1].ID
	decision.ReturnAction = ""
	if shortlistMode {
		decision.SelectedQuoteKey = pending.Attempts[1].QuoteKey
	}
	decision.Decisions[0].Decision = "accept"
	decision.Decisions[0].Comment = ""
	type result struct {
		w purchaseDomain.QuotationApprovalsWorkspace
		e error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			w, e := service.Execute(ctx, "approval-accounts", f.project, boID, "review", decision)
			results <- result{w, e}
		}()
	}
	var approved purchaseDomain.QuotationApprovalsWorkspace
	successes := 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.e == nil {
			approved = r.w
			successes++
		} else if !errors.Is(r.e, purchaseDomain.QuotationApprovalsStale) {
			t.Fatal("concurrent item approval", r.e)
		}
	}
	if successes != 1 {
		t.Fatal("concurrent decision count", successes)
	}
	e = nil
	if e != nil || approved.Version != 4 || !approved.Ready || approved.Attempts[1].EffectiveState != "approved" {
		t.Fatal("native approval", e)
	}
	checkCounts(2)
	if allocationMode == "capacity" {
		verifyNativePurchaseSourcingCapacity(t, f, ids, boID, lineID, vendor, approved.Attempts[1], factory, direct)
	} else if allocationMode == "partial" {
		verifyNativePurchaseSourcingLOI(t, f, ids, boID, lineID, vendor, approved.Attempts[1], factory, direct, "6")
		verifyNativePurchaseSourcingLOI(t, f, ids, boID, lineID, vendor, approved.Attempts[1], factory, direct, "4")
		var quantity, amount string
		if e := f.pool.QueryRow(ctx, "SELECT sum(quantity)::text,sum(amount)::text FROM purchase_sourcing_allocations WHERE is_active").Scan(&quantity, &amount); e != nil || quantity != "10.0000" || (direct && amount != "145.69") || (!direct && amount != "145.74") {
			t.Fatal("partial committed totals", quantity, amount, e)
		}
	} else {
		verifyNativePurchaseSourcingLOI(t, f, ids, boID, lineID, vendor, approved.Attempts[1], factory, direct, "10")
	}
	checkCounts(2)
	read, e := service.Workspace(ctx, "approval-submitter", f.project, boID)
	if e != nil {
		t.Fatal(e)
	}
	for _, event := range read.History {
		snapshots := []purchaseDomain.QuotationApprovalsRecord{event.After}
		if event.Before != nil {
			snapshots = append(snapshots, *event.Before)
		}
		for _, snapshot := range snapshots {
			for _, a := range snapshot.Attempts {
				for _, line := range a.Lines {
					if line.Budget != nil || line.Difference != nil {
						t.Fatal("Finance amounts leaked in Procurement history")
					}
				}
			}
		}
	}
	checkProgress := func(wantReady bool) {
		t.Helper()
		providers := func(c database.Connection) purchaseRepository.ProgressGroupedProviders {
			return purchaseRepository.ProgressGroupedProviders{Projects: projectout.NewContextReader(c), BOs: purchaseRepository.NewBOReader(c), Groups: purchaseRepository.NewIndentGroupsReader(c), Documents: purchaseRepository.NewIndentDocumentsReader(c, purchaseRepository.NewIndentGroupsReader(c)), Quotes: purchaseRepository.NewQuotationsProgressReader(c), Approvals: purchaseRepository.NewQuotationApprovalsProgressReader(c, factory(c))}
		}
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			finance, err := purchaseRepository.NewQuotationApprovalsProgressReader(c, factory(c)).Progress(txctx, f.project, boID)
			if err != nil || finance.Ready != wantReady || !finance.Started {
				t.Fatal("actual Finance status progress", err, finance)
			}
			facts, err := purchaseRepository.NewProgressGroupedReader(c, providers(c)).Read(txctx, f.project, boID)
			if err != nil || len(facts) != 3 || facts[2].Number != 4 || (facts[2].State == "completed") != wantReady {
				t.Fatal("actual grouped progress", err, facts)
			}
			groups, err := purchaseRepository.NewIndentGroupsReader(c).Get(txctx, f.project, boID)
			if err != nil || groups == nil {
				t.Fatal("actual grouped scope", err)
			}
			docs, err := purchaseRepository.NewIndentDocumentsReader(c, purchaseRepository.NewIndentGroupsReader(c)).List(txctx, f.project, boID)
			if err != nil {
				t.Fatal(err)
			}
			expected := 0
			for _, group := range groups.Groups {
				expected += len(group.VendorIDs)
			}
			if len(docs) < expected && facts[0].State == "completed" {
				t.Fatal("ungenerated vendor documents earned grouped completion", facts)
			}
			quotes, err := purchaseRepository.NewQuotationsProgressReader(c).Progress(txctx, f.project, boID)
			if err != nil || len(quotes.Quotes) == 0 {
				t.Fatal("actual saved quotation progress", err, quotes)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := purchaseRepository.NewQuotationApprovalsProgressReader(f.pool, factory(f.pool)).Progress(ctx, f.project, boID); err == nil {
			t.Fatal("unbound Finance progress reader accepted pool")
		}
		if _, err := purchaseRepository.NewQuotationsProgressReader(f.pool).Progress(ctx, f.project, boID); err == nil {
			t.Fatal("unbound quote progress reader accepted pool")
		}
		if _, err := purchaseRepository.NewProgressGroupedReader(f.pool, providers(f.pool)).Read(ctx, f.project, boID); err == nil {
			t.Fatal("unbound grouped progress reader accepted pool")
		}
	}
	checkProgress(approved.Ready)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	checkProgress(approved.Ready)
	if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		if _, err := purchaseRepository.NewQuotationApprovalsApprovedReader(c, factory(c)).Approved(txctx, f.project, boID); err == nil {
			t.Fatal("reporting relaxed held-project commitment gate")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	// The gate rechecks the real saved sources, and rejects outside transaction reads.
	reader := purchaseRepository.NewQuotationApprovalsApprovedReader(f.pool, factory(f.pool))
	if _, e = reader.Approved(ctx, f.project, boID); e == nil {
		t.Fatal("pool approval gate accepted")
	}
	e = database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		rows, e := purchaseRepository.NewQuotationApprovalsApprovedReader(c, factory(c)).Approved(txctx, f.project, boID)
		if approved.Ready && (e != nil || len(rows) == 0) {
			t.Fatal("ready downstream gate", e)
		}
		if !approved.Ready && e == nil {
			t.Fatal("unquoted groups considered ready")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	// Budget-only revisions retain the audited vendor decision and its original budget snapshot.
	if _, e := budgets.Execute(ctx, "budget-manager", f.project, budget.Command{Action: "revise", Version: 2, Reason: "Prepare updated item budget"}); e != nil {
		t.Fatal(e)
	}
	retained, e := service.Workspace(ctx, "approval-accounts", f.project, boID)
	if e != nil || retained.Attempts[len(retained.Attempts)-1].EffectiveState != "approved" {
		t.Fatal("draft budget invalidated effective approved baseline", e)
	}
	data.Allocations[0].Amount = "201.00"
	if _, e := budgets.Execute(ctx, "budget-manager", f.project, budget.Command{Action: "save", Version: 3, Reason: "Update draft allocation", Data: &data}); e != nil {
		t.Fatal(e)
	}
	if _, e := budgets.Execute(ctx, "budget-manager", f.project, budget.Command{Action: "approve", Version: 4, Reason: "Approve revised item budget"}); e != nil {
		t.Fatal(e)
	}
	budgetNotices++
	changed, e := service.Workspace(ctx, "approval-accounts", f.project, boID)
	if e != nil || changed.Ready != approved.Ready || changed.Attempts[len(changed.Attempts)-1].EffectiveState != "approved" || changed.Attempts[len(changed.Attempts)-1].BudgetRevision != 1 {
		t.Fatal("budget-only revision invalidated or rewrote quotation approval", e)
	}
	checkProgress(approved.Ready)
	checkCounts(2)
	e = database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
		p := factory(c)
		future := quoteSourceClock{now: purchaseRepository.Values{}.Now().Add(24 * time.Hour)}
		p.Quotes = purchaseRepository.NewQuotationsApprovalReader(c, quotesFactory(c), purchaseRepository.NewIndentGroupsReader(c), future)
		if _, e := purchaseRepository.NewQuotationApprovalsApprovedReader(c, p).Approved(txctx, f.project, boID); e == nil {
			t.Fatal("expired quote remained ready downstream")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	// A supplier master revision invalidates approval; verifying the new evidence
	// still changes its fingerprint and requires resubmission.
	execFixture(t, f.pool, "UPDATE suppliers SET version=version+1 WHERE id=$1", vendor)
	stale, e := service.Workspace(ctx, "approval-accounts", f.project, boID)
	if e != nil || stale.Ready || stale.Attempts[len(stale.Attempts)-1].EffectiveState != "stale" {
		t.Fatal("vendor change did not invalidate approval", e)
	}
	checkProgress(false)
	verify()
	stale, e = service.Workspace(ctx, "approval-accounts", f.project, boID)
	if e != nil || stale.Attempts[len(stale.Attempts)-1].EffectiveState != "stale" {
		t.Fatal("reverification silently reused old approval", e)
	}
	for _, sql := range []string{"UPDATE finance_quotation_decisions SET reason='rewrite'", "DELETE FROM finance_quotation_decisions", "UPDATE quotation_approval_events SET reason='rewrite'", "DELETE FROM quotation_approval_sets", "UPDATE quotation_approval_sets SET attempts=jsonb_set(attempts,'{0,total}','\"1.00\"'),version=version+1", "UPDATE quotation_approval_sets SET attempts='[]',version=version+1"} {
		if _, e = f.pool.Exec(ctx, sql); e == nil {
			t.Fatal("quotation approval evidence mutable")
		}
	}
}

var quotationDocumentObjects = &objects{data: map[string][]byte{}}
