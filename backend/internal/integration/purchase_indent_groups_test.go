package integration

import (
	"context"
	"errors"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"

	materialout "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"

	"testing"
)

func TestBOGroupVendorIssueAndInvitationSnapshots(t *testing.T) {
	verifyBOGroupVendorIssueAndInvitationSnapshots(t, false, "whole")
}

func TestBOGroupVendorIssueAndInvitationSnapshotsDirectLineTotal(t *testing.T) {
	verifyBOGroupVendorIssueAndInvitationSnapshots(t, true, "whole")
}

func TestBOGroupVendorIssueAndInvitationSnapshotsPartial(t *testing.T) {
	verifyBOGroupVendorIssueAndInvitationSnapshots(t, false, "partial")
}
func TestBOGroupVendorIssueAndInvitationSnapshotsPartialDirect(t *testing.T) {
	verifyBOGroupVendorIssueAndInvitationSnapshots(t, true, "partial")
}

func TestBOGroupVendorIssueAndInvitationSnapshotsCapacity(t *testing.T) {
	verifyBOGroupVendorIssueAndInvitationSnapshots(t, false, "capacity")
}
func TestBOGroupVendorIssueAndInvitationSnapshotsCapacityDirect(t *testing.T) {
	verifyBOGroupVendorIssueAndInvitationSnapshots(t, true, "capacity")
}

func verifyBOGroupVendorIssueAndInvitationSnapshots(t *testing.T, direct bool, allocationMode string) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	scope := []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", "projects.purchase_bo.read", "projects.purchase_indent_groups.read", "projects.purchase_indent_groups.view"}
	all := append(append([]string{}, scope...), "settings.items.read", "settings.items.write", "projects.items.write", "projects.purchase_bo.write", "projects.purchase_bo.complete", "projects.purchase_indent_groups.write", "projects.purchase_group_vendors.write", "projects.purchase_indent_documents.generate", "projects.purchase_indent_documents.dispatch")
	ids := projectIdentities{"writer": {ID: f.actor, Active: true, Permissions: all}, "item-editor": {ID: f.actor, Active: true, Permissions: append(append([]string{}, scope...), "projects.purchase_indent_groups.write")}, "issuer": {ID: f.actor, Active: true, Permissions: append(append([]string{}, scope...), "projects.purchase_indent_documents.generate")}, "outsider": {ID: f.outsider, Active: true, Permissions: all}}
	item, e := f.itemService(ids).Create(ctx, "writer", f.template, f.make, map[string]string{"C": "3C"}, "MODEL")
	if e != nil {
		t.Fatal(e)
	}
	boFactory := func(c database.Connection) purchaseRepository.BOProviders {
		p := f.materialProviders(c, purchaseRepository.NewIndentsReader(c))
		return purchaseRepository.BOProviders{Scope: p.Scope, Items: p.Items, Rules: p.Rules, Projects: p.Projects, Demands: materialout.NewDemandPort(c, p, purchaseJournal(c))}
	}
	bos := purchaseApp.NewBO(purchaseRepository.NewBOUnitOfWork(f.pool, boFactory, purchaseJournal), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	boID, lineID := (purchaseRepository.Values{}).ID(), (purchaseRepository.Values{}).ID()
	q, date, name := "10", "2026-10-20", "BO cables"
	b, e := bos.Save(ctx, "writer", f.project, boID, purchaseDomain.BOSave{Name: &name, Version: 0, Reason: "Prepare BO", Lines: []purchaseDomain.BOInputLine{{ID: lineID, ItemID: &item.ID, Quantity: &q, RequiredDate: &date}}})
	if e != nil {
		t.Fatal(e)
	}
	vendors := []string{}
	for i := 0; i < 4; i++ {
		id := (purchaseRepository.Values{}).ID()
		vendors = append(vendors, id)
		execFixture(t, f.pool, `INSERT INTO suppliers(id,code,name,data,is_active)VALUES($1,$2,$3,'{"indent_notes":"Vendor-specific inspection note","indent_terms":"Agreed vendor conditions","delivery_terms":"Agreed delivery location"}',true)`, id, fmt.Sprintf("V%d", i), fmt.Sprintf("Vendor %d", i))
	}
	factory := func(c database.Connection) purchaseRepository.IndentGroupsProviders {
		p := f.materialProviders(c, purchaseRepository.NewIndentsReader(c))
		return purchaseRepository.IndentGroupsProviders{Scope: p.Scope, BO: purchaseRepository.NewBOReader(c), Suppliers: masterRepository.NewVendorDirectory(c), Projects: p.Projects, Clients: f.providers(c).Clients}
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.IndentGroupsService {
		return purchaseApp.NewIndentGroups(purchaseRepository.NewIndentGroupsUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	}
	service := build(purchaseJournal)
	groupID := (purchaseRepository.Values{}).ID()
	group := purchaseDomain.IndentGroupsGroupInput{ID: groupID, Name: "Cable group", LineIDs: []string{lineID}, VendorIDs: []string{}}
	command := purchaseDomain.IndentGroupsCommand{Version: 0, BOVersion: b.Version, Action: "create_group", Reason: "Group cable items", Groups: []purchaseDomain.IndentGroupsGroupInput{group}}
	if _, e = service.Execute(ctx, "outsider", f.project, boID, command); !errors.Is(e, purchaseDomain.IndentGroupsForbidden) {
		t.Fatal("outside group scope", e)
	}
	w, e := service.Execute(ctx, "item-editor", f.project, boID, command)
	if e != nil || w.Record == nil || w.Record.State != "grouped" {
		t.Fatal("draft BO grouping", e)
	}
	allocation := purchaseDomain.IndentGroupsCommand{Version: w.Record.Version, BOVersion: b.Version, Action: "set_vendors", Reason: "Allocate vendors", GroupID: &groupID, VendorIDs: vendors[:2]}
	if _, e = service.Execute(ctx, "item-editor", f.project, boID, allocation); !errors.Is(e, purchaseDomain.IndentGroupsForbidden) {
		t.Fatal("group edit changed vendors", e)
	}
	if _, e = service.Execute(ctx, "writer", f.project, boID, allocation); e == nil {
		t.Fatal("fewer than three vendors")
	}
	allocation.VendorIDs = vendors[:3]
	w, e = service.Execute(ctx, "writer", f.project, boID, allocation)
	if e != nil {
		t.Fatal(e)
	}
	group.VendorIDs = vendors[:3]
	issue := purchaseDomain.IndentGroupsCommand{Version: w.Record.Version, BOVersion: b.Version, Action: "create", Reason: "Issue reviewed grouped indent", Groups: []purchaseDomain.IndentGroupsGroupInput{group}}
	if _, e = service.Execute(ctx, "writer", f.project, boID, issue); e == nil {
		t.Fatal("incomplete BO issued")
	}
	b, e = bos.Change(ctx, "writer", f.project, boID, "complete", purchaseDomain.BOChange{Version: b.Version, Reason: "Complete BO requirements"})
	if e != nil {
		t.Fatal(e)
	}
	issue.BOVersion = b.Version
	if _, e = build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Execute(ctx, "writer", f.project, boID, issue); e == nil {
		t.Fatal("group audit failure committed")
	}
	w, e = service.Execute(ctx, "issuer", f.project, boID, issue)
	if e != nil || w.Record.State != "created" || len(w.Record.SourceLines) != 1 || len(w.Record.SourceVendors) != 3 {
		t.Fatal("immutable issued snapshot", e)
	}
	if _, e = service.Execute(ctx, "issuer", f.project, boID, issue); !errors.Is(e, purchaseDomain.IndentGroupsStale) {
		t.Fatal("stale issue", e)
	}
	invitation := purchaseDomain.IndentGroupsCommand{Version: w.Record.Version, BOVersion: b.Version, Action: "invite", Reason: "Record RFQ dispatch", GroupID: &groupID, VendorID: &vendors[0], SentDate: &date, Channel: "Email", DispatchReference: "RFQ evidence"}
	today := purchaseDomain.IndentGroupsToday((purchaseRepository.Values{}).Now())
	invitation.SentDate = &today
	w, e = service.Execute(ctx, "writer", f.project, boID, invitation)
	if e != nil || len(w.ItemReport[0].Invitations) != 1 || w.ItemReport[0].Invitations[0].ScopeRevision != 1 {
		t.Fatal("RFQ unverified vendor invitation", e)
	}
	invitation.Version = w.Record.Version
	if _, e = service.Execute(ctx, "writer", f.project, boID, invitation); e == nil {
		t.Fatal("duplicate invitation")
	}
	addition := purchaseDomain.IndentGroupsCommand{Version: w.Record.Version, BOVersion: b.Version, Action: "add_vendors", Reason: "Invite additional active vendor", GroupID: &groupID, VendorIDs: vendors[3:]}
	w, e = service.Execute(ctx, "writer", f.project, boID, addition)
	if e != nil || len(w.Record.SourceVendors) != 4 {
		t.Fatal("append vendor", e)
	}
	reset := purchaseDomain.IndentGroupsCommand{Version: w.Record.Version, BOVersion: b.Version, Action: "reset_item", Reason: "Attempt issued scope reset", LineID: &lineID}
	if _, e = service.Execute(ctx, "writer", f.project, boID, reset); e == nil {
		t.Fatal("issued item reset")
	}
	execFixture(t, f.pool, "UPDATE suppliers SET is_active=false WHERE id=$1", vendors[0])
	w, e = service.Workspace(ctx, "writer", f.project, boID)
	if e != nil || len(w.Vendors) != 4 {
		t.Fatal("retained archived vendor", e)
	}
	if w.ItemReport[0].SelectedVendors[0].Name != "Vendor 0" {
		t.Fatal("historical vendor name lost")
	}
	history := w.History
	if len(history) != 5 || history[0].Before == nil {
		t.Fatal("group history", len(history))
	}

	// Native document generation, notice deduplication and dispatch share the
	// same transaction with their owning grouping application service.
	writer := ids["writer"]
	writer.Permissions = append(writer.Permissions, "projects.purchase_indent_documents.view", "projects.purchase_indent_documents.read", "projects.purchase_indent_documents.print")
	ids["writer"] = writer
	ids["manager-notices"] = auth.Identity{ID: f.manager, Active: true, Permissions: []string{"projects.master.read"}}
	ids["outsider-notices"] = auth.Identity{ID: f.outsider, Active: true, Permissions: []string{"projects.master.read"}}
	docsFactory := func(c database.Connection) purchaseRepository.IndentDocumentsProviders {
		p := factory(c)
		return purchaseRepository.IndentDocumentsProviders{Scope: p.Scope, BO: p.BO, Groups: purchaseRepository.NewIndentGroupsReader(c), Commands: purchaseRepository.NewIndentGroupsCommander(c, p, purchaseJournal(c)), Suppliers: p.Suppliers, Projects: p.Projects, Clients: p.Clients, Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))}
	}
	docsBuild := func(j func(database.Connection) events.Writer) *purchaseApp.IndentDocumentsService {
		return purchaseApp.NewIndentDocuments(purchaseRepository.NewIndentDocumentsUnitOfWork(f.pool, docsFactory, j), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	}
	documents := docsBuild(purchaseJournal)
	generate := purchaseDomain.IndentDocumentsCommand{Action: "generate", GroupID: &groupID, VendorID: &vendors[1], GroupVersion: w.Record.Version, BOVersion: b.Version}
	execFixture(t, f.pool, `UPDATE suppliers SET data=jsonb_set(data,'{indent_terms}','""') WHERE id=$1`, vendors[1])
	if _, e = documents.Execute(ctx, "writer", f.project, boID, generate); e == nil {
		t.Fatal("generation accepted missing vendor conditions")
	}
	var unconfiguredCount int
	if e = f.pool.QueryRow(ctx, `SELECT count(*) FROM purchase_indent_documents WHERE project_id=$1 AND bo_id=$2`, f.project, boID).Scan(&unconfiguredCount); e != nil || unconfiguredCount != 0 {
		t.Fatal("unconfigured generation committed a document", e)
	}
	execFixture(t, f.pool, `UPDATE suppliers SET data=jsonb_set(data,'{indent_terms}','"Agreed vendor conditions"') WHERE id=$1`, vendors[1])
	if _, e = docsBuild(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Execute(ctx, "writer", f.project, boID, generate); e == nil {
		t.Fatal("document generation audit failure committed")
	}
	dw, e := documents.Workspace(ctx, "writer", f.project, boID)
	if e != nil || len(dw.Documents) != 0 {
		t.Fatal("generation rollback", e)
	}
	dw, e = documents.Execute(ctx, "writer", f.project, boID, generate)
	if e != nil || len(dw.Documents) != 1 || !dw.Documents[0].Data.IsCurrent || !dw.ManagerAssigned {
		t.Fatal("generate vendor indent", e)
	}

	published, e := purchaseRepository.NewIndentDocumentsReader(f.pool, purchaseRepository.NewIndentGroupsReader(f.pool)).List(ctx, f.project, boID)
	if e != nil || len(published) != 1 || !published[0].Data.IsCurrent {
		t.Fatal("published current document projection", e)
	}
	document := dw.Documents[0]

	if document.Data.Terms == nil || document.Data.Terms.Source != "vendor_master" || document.Data.Terms.Conditions[0] != "Agreed vendor conditions" {
		t.Fatal("vendor terms were not frozen")
	}
	execFixture(t, f.pool, `UPDATE suppliers SET data=jsonb_set(data,'{indent_terms}','"Later vendor terms"') WHERE id=$1`, vendors[1])
	if document.Data.Vendor.Name != "Vendor 1" || len(document.Data.Lines) != 1 || document.Data.Number != purchaseDomain.IndentDocumentsNumber(document.ID) {
		t.Fatal("document source snapshot")
	}
	dw, e = documents.Execute(ctx, "writer", f.project, boID, generate)
	if e != nil || len(dw.Documents) != 1 || len(dw.History) != 1 {
		t.Fatal("idempotent generation", e)
	}
	if dw.Documents[0].Data.Terms.Conditions[0] != "Agreed vendor conditions" {
		t.Fatal("vendor edit changed issued terms")
	}
	print := purchaseDomain.IndentDocumentsCommand{Action: "print", DocumentID: &document.ID, Version: document.Version}
	if _, e = docsBuild(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Execute(ctx, "writer", f.project, boID, print); e == nil {
		t.Fatal("print notice audit failure committed")
	}
	notices, e := documents.Notifications(ctx, "manager-notices")
	if e != nil || len(notices) != 0 {
		t.Fatal("notice rollback", e)
	}
	dw, e = documents.Execute(ctx, "writer", f.project, boID, print)
	if e != nil || len(dw.Documents[0].Data.PrintRequests) != 1 {
		t.Fatal("print request", e)
	}
	print.Version = dw.Documents[0].Version
	dw, e = documents.Execute(ctx, "writer", f.project, boID, print)
	if e != nil || len(dw.Documents[0].Data.PrintRequests) != 2 {
		t.Fatal("repeat print log", e)
	}
	notices, e = documents.Notifications(ctx, "manager-notices")
	if e != nil || len(notices) != 1 || notices[0].Number != document.Data.Number {
		t.Fatal("pending dispatch notice deduplication", e)
	}
	notices, e = documents.Notifications(ctx, "outsider-notices")
	if e != nil || len(notices) != 0 {
		t.Fatal("notice recipient isolation", e)
	}
	send := purchaseDomain.IndentDocumentsCommand{Action: "send", DocumentID: &document.ID, Version: dw.Documents[0].Version, SentDate: &today, Channel: "Email", DispatchReference: "Stored dispatch evidence"}
	if _, e = docsBuild(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Execute(ctx, "writer", f.project, boID, send); e == nil {
		t.Fatal("dispatch audit failure committed")
	}
	groupBefore, e := service.Workspace(ctx, "writer", f.project, boID)
	if e != nil || len(groupBefore.Record.Groups[0].Invitations) != 1 {
		t.Fatal("dispatch invitation rollback", e)
	}
	notices, e = documents.Notifications(ctx, "manager-notices")
	if e != nil || len(notices) != 1 {
		t.Fatal("notice resolution rollback", e)
	}
	dw, e = documents.Execute(ctx, "writer", f.project, boID, send)
	if e != nil || dw.Documents[0].Data.SentDate == nil {
		t.Fatal("dispatch commit", e)
	}
	notices, e = documents.Notifications(ctx, "manager-notices")
	if e != nil || len(notices) != 0 {
		t.Fatal("pending notice unresolved", e)
	}
	groupAfter, e := service.Workspace(ctx, "writer", f.project, boID)
	if e != nil || len(groupAfter.Record.Groups[0].Invitations) != 2 || len(groupAfter.Record.Groups[0].Invitations[1].LineIDs) != 1 {
		t.Fatal("dispatch frozen invitation", e)
	}
	if _, e = documents.Execute(ctx, "writer", f.project, boID, send); !errors.Is(e, purchaseDomain.IndentDocumentsStale) {
		t.Fatal("stale document dispatch", e)
	}
	send.Version = dw.Documents[0].Version
	if _, e = documents.Execute(ctx, "writer", f.project, boID, send); e == nil {
		t.Fatal("duplicate dispatch")
	}
	print.Version = dw.Documents[0].Version
	dw, e = documents.Execute(ctx, "writer", f.project, boID, print)
	if e != nil {
		t.Fatal("print dispatched document", e)
	}
	notices, e = documents.Notifications(ctx, "manager-notices")
	if e != nil || len(notices) != 0 {
		t.Fatal("sent indent reopened notice", e)
	}
	if !direct && allocationMode == "whole" {
		verifyLegacyIndentTerms(t, f, documents, document)
	}
	verifyNativeQuotationReceipt(t, f, ids, boID, document.ID, lineID, today, direct, allocationMode)
	if _, e = f.pool.Exec(ctx, "UPDATE purchase_indent_documents SET data=jsonb_set(data,'{group_name}','\"Rewritten\"')"); e == nil {
		t.Fatal("generated scope overwritten")
	}
	if _, e = f.pool.Exec(ctx, "UPDATE purchase_indent_document_events SET reason='rewrite'"); e == nil {
		t.Fatal("document event overwritten")
	}
	if _, e = f.pool.Exec(ctx, "DELETE FROM purchase_indent_dispatch_notices"); e == nil {
		t.Fatal("dispatch notice deleted")
	}
	if _, e = purchaseRepository.NewIndentGroupsCommander(f.pool, factory(f.pool), purchaseJournal(f.pool)).Execute(ctx, purchaseDomain.Principal{ID: f.actor, Active: true, Permissions: all}, f.project, boID, reset); e == nil {
		t.Fatal("unbound group commander accepted")
	}
	if _, e = f.pool.Exec(ctx, "DELETE FROM purchase_indent_group_entries"); e == nil {
		t.Fatal("group identities deleted")
	}
	if _, e = f.pool.Exec(ctx, "UPDATE purchase_indent_group_events SET reason='rewrite'"); e == nil {
		t.Fatal("group history rewritten")
	}
}

func TestFinanceResolvesBudgetBeforeVendorApproval(t *testing.T) {
	verifyBOGroupVendorIssueAndInvitationSnapshots(t, true, "no_budget")
}

func TestFinanceApprovesWithoutIndividualItemAllocations(t *testing.T) {
	verifyBOGroupVendorIssueAndInvitationSnapshots(t, true, "project_budget_only")
}
