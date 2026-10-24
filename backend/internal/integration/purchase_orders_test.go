package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"

	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func verifyNativePurchaseOrder(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.IndentsFactory, wp purchaseRepository.WorkflowFactory, ws *purchaseApp.WorkflowService, pkg string, financeFactory purchaseRepository.QuotationApprovalsFactory) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_orders.read", "projects.purchase_orders.write", "projects.purchase_orders.release", "projects.purchase_orders.client_approval"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native PO',true)", role, "Native PO "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native PO')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	actor := ids["loi-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), codes...)
	ids["po-writer"] = actor
	view, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: view.Package.Version, Reason: "Assign actual PO doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 9, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.OrdersToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	factory := func(c database.Connection) purchasePorts.OrdersContext {
		x := p(c)
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		scope := accessout.NewGate(c, accessProviders(c))
		workSource := purchaseRepository.NewWorkflowSourceReader(c, wp(c))
		suppliers := masterRepository.NewVendorDirectory(c)
		sp := purchaseRepository.SourcingProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Indents: purchaseRepository.NewIndentsReader(c), Items: x.Items, Suppliers: suppliers, BO: purchaseRepository.NewBODemandReader(c), Quotes: purchaseRepository.NewQuotationApprovalsApprovedReader(c, financeFactory(c))}
		tp := purchaseRepository.TechnicalProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Released: purchaseRepository.NewSourcingReleasedReader(c)}
		verified := financeFactory(c).Verification
		gate, ok := verified.(masterContracts.VendorVerificationGate)
		if !ok {
			t.Fatal("fixture must use real verification gate")
		}
		return purchaseRepository.NewOrdersContext(purchaseRepository.OrdersProviders{Projects: x.Projects, Purchase: x.Scope, Access: scope, Users: users, Work: workSource, Authority: purchaseRepository.NewSourcingAuthorityReader(c, sp), Technical: purchaseRepository.NewTechnicalClearanceReader(c, tp), Verification: gate, Verified: verified, Suppliers: suppliers, Indents: purchaseRepository.NewIndentsReader(c)})
	}
	objects := &nativePOObjects{data: map[string][]byte{}}
	cipher, err := documentInfra.NewDocumentCipher(strings.Repeat("12", 32))
	if err != nil {
		t.Fatal(err)
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.OrdersService {
		return purchaseApp.NewOrders(purchaseRepository.NewOrdersUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v).WithEvidence(cipher, purchaseRepository.TechnicalInspector{}, objects, "private-native-po")
	}
	service := build(purchaseJournal)
	api := chi.NewRouter()
	purchaseHTTP.OrdersRegister(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/purchase-orders"
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	w, e := service.Workspace(ctx, "po-writer", f.project, pkg)
	if e != nil || w.Source.Guard() != nil {
		t.Fatal("current source gates", e, w.Source.Blockers, w.Source.Guard())
	}
	if len(w.History) != 0 || w.Order != nil {
		t.Fatal("fake PO seeded")
	}
	if response := request("GET", base, "", ""); response.Code != 401 {
		t.Fatal("unauthenticated PO read", response.Code)
	}
	if response := request("POST", base, "po-writer", `{"version":0,"action":"prepare","reason":"r","total":"999"}`); response.Code != 422 {
		t.Fatal("forged price accepted", response.Code)
	}
	details := purchaseDomain.OrdersDetails{Date: purchaseDomain.OrdersToday(v.Now()), IssuerName: "Native actual issuer", IssuerAddress: "Native actual office", BillTo: "Native actual invoice address", DeliveryAddress: "Native actual site"}
	run := func(q purchaseDomain.OrdersCommand) {
		t.Helper()
		raw, e := json.Marshal(q)
		if e != nil {
			t.Fatal(e)
		}
		response := request("POST", base, "po-writer", string(raw))
		if response.Code != 200 {
			t.Fatal("PO command", q.Action, response.Code, response.Body.String())
		}
		if e = json.Unmarshal(response.Body.Bytes(), &w); e != nil {
			t.Fatal(e)
		}
	}
	run(purchaseDomain.OrdersCommand{Action: "prepare", Reason: "Prepare from actual cleared kit", Details: &details})
	if w.Order == nil || w.Order.Source.Total != "145.74" || w.Order.Source.UnitPrice != "12.3500" {
		t.Fatal("exact PO source", w.Order)
	}
	// This new gate must reject an unsigned draft before allocating a PO number.
	if _, err := service.Execute(ctx, "po-writer", f.project, pkg, purchaseDomain.OrdersCommand{Version: w.Order.Version, Action: "release", Reason: "Unsigned draft"}); err == nil {
		t.Fatal("unsigned PO released")
	}
	run(purchaseDomain.OrdersCommand{Version: w.Order.Version, Action: "submit_client", Reason: "Actual client submission", Submission: &purchaseDomain.OrdersClientSubmission{Reference: "PO-TRANSMITTAL", Date: details.Date}})
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_orders.client_approval')", role)
	if err := service.AuthorizeClientUpload(ctx, "po-writer", f.project, pkg, w.Order.Version); err == nil {
		t.Fatal("revoked client evidence authority accepted")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_orders.client_approval'", role)
	if _, err := service.UploadClientFile(ctx, "po-writer", f.project, pkg, w.Order.Version, "bad.pdf", "application/pdf", []byte("unreadable")); err == nil {
		t.Fatal("unreadable signed PO accepted")
	}
	evidence, err := service.UploadClientFile(ctx, "po-writer", f.project, pkg, w.Order.Version, "signed-po.pdf", "application/pdf", []byte("%PDF-1.4\nNative actual signed PO fixture\n%%EOF"))
	if err != nil {
		t.Fatal("signed upload", err)
	}
	if len(objects.data) != 1 {
		t.Fatal("signed file not stored through private object port")
	}
	decision := &purchaseDomain.OrdersClientDecision{Status: "approved", Reference: "CLIENT-SIGNED-PO", Date: details.Date, Signatory: "Client representative", Designation: "Project engineer", FileID: evidence.ID}
	foreign := *decision
	foreign.FileID = v.ID()
	if _, err := service.Execute(ctx, "po-writer", f.project, pkg, purchaseDomain.OrdersCommand{Version: w.Order.Version, Action: "client_decision", Reason: "Foreign file", ClientDecision: &foreign}); err == nil {
		t.Fatal("unowned sign-off file accepted")
	}
	run(purchaseDomain.OrdersCommand{Version: w.Order.Version, Action: "client_decision", Reason: "Actual client approval", ClientDecision: decision})
	if !purchaseDomain.OrdersClientCleared(*w.Order) {
		t.Fatal("client signoff not retained")
	}
	draft := purchaseDomain.OrdersCopy(*w.Order)
	if _, err := purchaseRepository.NewOrdersReleasedReader(f.pool).Released(ctx, f.project, pkg); err == nil {
		t.Fatal("unbound released reader accepted pool connection")
	}
	if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, conn database.Connection) error {
		fact, err := purchaseRepository.NewOrdersReleasedReader(conn).Released(txctx, f.project, pkg)
		if err == nil || fact != nil {
			t.Fatal("draft PO published as released evidence")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	if leaked, e := failed.Execute(ctx, "po-writer", f.project, pkg, purchaseDomain.OrdersCommand{Version: draft.Version, Action: "release", Reason: "Late PO journal failure"}); e == nil || leaked.Order != nil {
		t.Fatal("failed release exposed order")
	}
	var sequenceCount int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_order_sequences").Scan(&sequenceCount); e != nil || sequenceCount != 0 {
		t.Fatal("failed release consumed number", sequenceCount, e)
	}
	heldDetails := details
	heldDetails.DeliveryAddress = "changed without audit"
	if _, e = f.pool.Exec(ctx, "UPDATE purchase_orders SET data=jsonb_set(data,'{details,delivery_address}',to_jsonb($2::text)),version=version+1 WHERE id=$1", draft.ID, heldDetails.DeliveryAddress); e == nil {
		t.Fatal("unaudited PO changed")
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_orders.release')", role)
	if _, e = service.Execute(ctx, "po-writer", f.project, pkg, purchaseDomain.OrdersCommand{Version: draft.Version, Action: "release", Reason: "Revoked live permission"}); e == nil {
		t.Fatal("claimed permission bypassed live revocation")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_orders.release'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "po-writer", f.project, pkg)
	if e != nil || held.Order == nil || len(held.Actions) != 0 {
		t.Fatal("held order retained release authority", e)
	}
	if _, e = service.Execute(ctx, "po-writer", f.project, pkg, purchaseDomain.OrdersCommand{Version: draft.Version, Action: "release", Reason: "Held source"}); e == nil {
		t.Fatal("held project released order")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	// Two actual transactions race the same expected version. Exactly one commits.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := service.Execute(ctx, "po-writer", f.project, pkg, purchaseDomain.OrdersCommand{Version: draft.Version, Action: "release", Reason: "Concurrent valid release"})
			results <- e
		}()
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
		t.Fatal("duplicate concurrent releases", success)
	}
	w, e = service.Workspace(ctx, "po-writer", f.project, pkg)
	if e != nil || w.Order.State != "released" || w.Order.Number == "" || w.Order.ReleasedBy != f.actor || w.Order.Version != 4 || len(w.History) != 4 {
		t.Fatal("frozen release", e, w)
	}

	if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, conn database.Connection) error {
		reader := purchaseRepository.NewOrdersReleasedReader(conn)
		progress, err := purchaseRepository.NewOrdersProgressReader(conn).Progress(txctx, f.project, pkg)
		if err != nil || progress.State != "completed" || !progress.Current || progress.Snapshot == nil || progress.Snapshot.ID != w.Order.ID {
			t.Fatal("actual issued order progress", progress, err)
		}
		foreignProgress, err := purchaseRepository.NewOrdersProgressReader(conn).Progress(txctx, f.otherProject, pkg)
		if err != nil || foreignProgress.State != "not_started" || foreignProgress.Snapshot != nil {
			t.Fatal("cross-project order progress leak", err)
		}
		fact, err := reader.Released(txctx, f.project, pkg)
		if err != nil || fact == nil || fact.ID != w.Order.ID || fact.Number != w.Order.Number || fact.Source.Total != "145.74" {
			t.Fatal("actual immutable released PO fact", err)
		}
		fact.Source.Total = "forged"
		*fact.ReleasedAt = purchaseDomain.OrdersTimestamp{}
		again, err := reader.Released(txctx, f.project, pkg)
		if err != nil || again.Source.Total != "145.74" || again.ReleasedAt.IsZero() {
			t.Fatal("released fact aliases saved order", err)
		}
		if foreign, err := reader.Released(txctx, f.otherProject, pkg); err == nil || foreign != nil {
			t.Fatal("foreign PO fact leaked")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	verifyNativePurchaseAcceptance(t, f, ids, p, wp, ws, pkg, financeFactory)
	var releases, eventsCount int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_orders WHERE state='released'").Scan(&releases); e != nil || releases != 1 {
		t.Fatal("release count", e, releases)
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_orders' AND entity_id=$1", draft.ID).Scan(&eventsCount); e != nil || eventsCount != 4 {
		t.Fatal("atomic project history", e, eventsCount)
	}
	if _, e = f.pool.Exec(ctx, "UPDATE purchase_orders SET version=version+1 WHERE id=$1", draft.ID); e == nil {
		t.Fatal("released order mutable")
	}
	if _, e = f.pool.Exec(ctx, "DELETE FROM purchase_orders WHERE id=$1", draft.ID); e == nil {
		t.Fatal("released order erased")
	}
	if _, e = f.pool.Exec(ctx, "DELETE FROM purchase_order_events WHERE subject_id=$1", draft.ID); e == nil {
		t.Fatal("PO history erased")
	}
	if leaked, e := service.Workspace(ctx, "po-writer", f.otherProject, pkg); e == nil || leaked.Order != nil {
		t.Fatal("foreign project PO exposed")
	}
	if leaked, e := failed.Document(ctx, "po-writer", f.project, pkg); e == nil || leaked.ID != "" {
		t.Fatal("failed export audit exposed PO")
	}
	for range 2 {
		response := request("GET", base+"/document", "po-writer", "")
		if response.Code != 200 {
			t.Fatal("audited repeat PO export", response.Code, response.Body.String())
		}
	}
	var prints int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_order_document_events WHERE order_id=$1", draft.ID).Scan(&prints); e != nil || prints != 2 {
		t.Fatal("repeat PO export attribution", prints, e)
	}

	raw, _ := json.Marshal(w.Order)
	again := request("GET", base, "po-writer", "")
	var actual purchaseDomain.OrdersWorkspace
	if again.Code != 200 || json.Unmarshal(again.Body.Bytes(), &actual) != nil {
		t.Fatal("PO HTTP view")
	}
	frozen, _ := json.Marshal(actual.Order)
	if !bytes.Equal(raw, frozen) {
		t.Fatal("frozen snapshot changed on read")
	}
}

// The database adapter is real; only the external object transport is isolated.
type nativePOObjects struct{ data map[string][]byte }

func (o *nativePOObjects) Put(_ context.Context, b, k string, data []byte) error {
	o.data[b+"/"+k] = append([]byte{}, data...)
	return nil
}
func (o *nativePOObjects) Get(_ context.Context, b, k string) ([]byte, error) {
	d, ok := o.data[b+"/"+k]
	if !ok {
		return nil, fmt.Errorf("missing native private object")
	}
	return append([]byte{}, d...), nil
}
