package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	masterHTTP "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"

	infra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authrepo "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inboxrepo "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	inboxapp "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/application"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	inboxdomain "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
	eventrepo "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	repo "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	app "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
	taskrepo "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/adapters/outbound"
	taskapp "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/application"
	taskdomain "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/domain"
	purchase "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

type noWorkflowTasks struct{}

func (noWorkflowTasks) Assigned(context.Context, purchase.WorkflowActor, purchase.WorkflowTaskQuery) (purchase.WorkflowTaskPage, error) {
	return purchase.WorkflowTaskPage{}, nil
}
func TestSupplierNamedReviewAssignmentsAndPersonalQueue(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := t.Context()
	values := repo.VendorDocumentValues{}
	admin, a, b, outsider := values.ID(), values.ID(), values.ID(), values.ID()
	for i, id := range []string{admin, a, b, outsider} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,'fixture',true,$4)", id, []string{"assignment_admin", "finance_a", "finance_b", "outsider"}[i], []string{"a@fixture.invalid", "b@fixture.invalid", "c@fixture.invalid", "d@fixture.invalid"}[i], i == 0)
	}
	codes := []string{"settings.supplier_documents.read", "settings.supplier_documents.write", "accounts.vendor_verification.view", "accounts.vendor_verification.read", "accounts.vendor_verification.write", "work.tasks.view", "work.tasks.read", "communication.inbox.read", "communication.inbox.write"}
	role := values.ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,'supplier Finance fixture','fixture',true)", role)
	for _, code := range codes {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description) VALUES($1,$2,'fixture')", values.ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id) SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	for _, id := range []string{a, b} {
		execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", id, role)
	}
	ids := projectIdentities{"admin": auth.Identity{ID: admin, Active: true, Superuser: true}, "a": auth.Identity{ID: a, Active: true, Permissions: codes}, "b": auth.Identity{ID: b, Active: true, Permissions: codes}, "outsider": auth.Identity{ID: outsider, Active: true}}
	users := func(c database.Connection) auth.AssignmentDirectory {
		return authapp.NewAssignmentDirectory(authrepo.NewAssignmentReader(c))
	}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventrepo.NewJournal(c)) }
	notices := func(c database.Connection) inbox.SupplierReviewWriter {
		return inboxrepo.NewSupplierReviewWriter(c, journal(c), inboxrepo.Values{})
	}
	assignments := app.NewVendorReviewAssignments(repo.NewVendorReviewAssignmentUnitOfWork(pool, users, journal, notices), infra.NewAuthBridge(ids), values)
	vendors := app.NewVendorService(repo.NewVendorUnitOfWork(pool, journal), infra.NewAuthBridge(ids), repo.VendorValues{})
	vendor, err := vendors.Create(ctx, "admin", domain.VendorCreate{Code: "REVIEW", Name: "Review fixture", Data: map[string]any{}, Reason: "Fixture supplier"})
	if err != nil {
		t.Fatal(err)
	}
	input := domain.VendorReviewAssignmentInput{UserID: a, Active: true, Reason: "Assign Finance owner"}
	if _, err = assignments.Save(ctx, "a", vendor.ID, input); !errors.Is(err, domain.VendorVerificationForbidden) {
		t.Fatal("ordinary reviewer granted assignment authority", err)
	}
	input.UserID = outsider
	if _, err = assignments.Save(ctx, "admin", vendor.ID, input); err == nil {
		t.Fatal("unqualified reviewer assigned")
	}
	input.UserID = a
	// Any notice failure must roll back assignment, local history and source journal.
	failNotices := func(c database.Connection) inbox.SupplierReviewWriter { return rejectedSupplierNotice{} }
	failing := app.NewVendorReviewAssignments(repo.NewVendorReviewAssignmentUnitOfWork(pool, users, journal, failNotices), infra.NewAuthBridge(ids), values)
	if _, err = failing.Save(ctx, "admin", vendor.ID, input); err == nil {
		t.Fatal("notice failure committed assignment")
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM supplier_review_assignments").Scan(&count)
	if count != 0 {
		t.Fatal("failed notice leaked assignment")
	}
	current, err := assignments.Save(ctx, "admin", vendor.ID, input)
	if err != nil || current.Version != 1 || current.UserID != a || *current.UpdatedBy != admin {
		t.Fatal("assignment failed", err)
	}
	mux := chi.NewRouter()
	masterHTTP.RegisterVendorReviewAssignments(mux, assignments)
	for _, test := range []struct {
		method, path, body, token string
		status                    int
	}{
		{"GET", "/api/v1/suppliers/" + vendor.ID + "/verification/assignment", "", "outsider", 403},
		{"GET", "/api/v1/suppliers/" + vendor.ID + "/verification/assignment/options", "", "a", 403},
		{"GET", "/api/v1/suppliers/" + vendor.ID + "/verification/assignment", "", "a", 200},
		{"PUT", "/api/v1/suppliers/" + vendor.ID + "/verification/assignment", `{"user_id":"` + a + `","version":1,"reason":"fixture","is_active":true,"created_by_id":"` + outsider + `"}`, "admin", 422},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		request.Header.Set("Authorization", "Bearer "+test.token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatal("assignment HTTP authority/strict payload", response.Code, test.status)
		}
	}
	if _, err = assignments.Save(ctx, "admin", vendor.ID, input); !errors.Is(err, domain.VendorVerificationStale) {
		t.Fatal("stale assignment accepted", err)
	}
	cipher, err := infra.NewDocumentCipher(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	objects := &objects{data: map[string][]byte{}}
	directory := func(c database.Connection) contracts.VendorDirectory { return repo.NewVendorDirectory(c) }
	evidence := func(c database.Connection) contracts.VendorDocumentReader {
		return repo.NewVendorDocumentEvidenceReader(c, cipher, infra.NewDocumentInspector(domain.VendorDocumentMaximumBytes)).WithObjects(objects)
	}
	source := func(c database.Connection) contracts.VendorReviewTasks {
		return repo.NewVendorReviewTasks(c, users(c), directory(c), evidence(c))
	}
	tasks := taskapp.New(taskrepo.NewUnitOfWork(pool, func(c database.Connection) taskrepo.Providers {
		return taskrepo.Providers{Users: users(c), Purchase: noWorkflowTasks{}, SupplierReviews: source(c)}
	}), taskrepo.NewAuthBridge(ids), taskrepo.Values{})
	page, err := tasks.Page(ctx, "a", taskdomain.Query{})
	if err != nil || pendingSupplierTasks(page) != 0 {
		t.Fatal("assignment alone created a document review without an upload", err)
	}
	if other, err := tasks.Page(ctx, "b", taskdomain.Query{}); err != nil || len(other.SupplierReviews) != 0 {
		t.Fatal("other recipient saw assigned task", err)
	}
	inboxService := inboxapp.New(inboxrepo.NewUnitOfWork(pool, users, journal), inboxrepo.NewAuthBridge(ids), inboxrepo.Values{})
	listing, err := inboxService.List(ctx, "a", inboxdomain.Query{Page: 1, Size: 20})
	if err != nil || len(listing.Items) != 1 || listing.Items[0].Kind != "supplier_review" || listing.Items[0].ProjectID != "" {
		t.Fatal("durable supplier assignment notice missing", err)
	}
	if _, err = inboxService.Receipt(ctx, "a", listing.Items[0].ID, 1, "read"); err != nil {
		t.Fatal("supplier notice receipt failed", err)
	}
	lifecycle := func(c database.Connection) ports.DocumentApprovalLifecycle {
		return app.NewDocumentApprovalService(repo.NewDocumentApprovalRepository(c, journal(c)), repo.NewApprovalUserDirectory(users(c)))
	}
	docs := app.NewVendorDocumentService(repo.NewVendorDocumentUnitOfWork(pool, directory, journal).WithApprovalLifecycle(lifecycle), infra.NewAuthBridge(ids), cipher, values).WithInspector(infra.NewDocumentInspector(domain.VendorDocumentMaximumBytes)).WithObjects(objects, "fixture")
	var img bytes.Buffer
	_ = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	files := []domain.VendorDocumentDocument{}
	for _, kind := range []string{"pan", "cheque"} {
		doc, err := docs.Upload(ctx, "a", vendor.ID, domain.VendorDocumentInput{DocType: kind, Filename: "fixture.png", Content: img.Bytes()}, "")
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, *doc)
	}
	finance := app.NewVendorVerificationService(repo.NewVendorVerificationUnitOfWork(pool, directory, evidence, journal, users).WithApprovalLifecycle(lifecycle), infra.NewAuthBridge(ids), repo.VendorVerificationValues{})
	status, err := finance.Status(ctx, "admin", vendor.ID)
	if err != nil {
		t.Fatal(err)
	}
	decisions := []domain.VendorVerificationDecision{}
	for _, doc := range files {
		decisions = append(decisions, domain.VendorVerificationDecision{DocumentID: doc.ID, Decision: "accept", Comment: "Fixture inspected"})
	}
	status, err = finance.Review(ctx, "admin", vendor.ID, domain.VendorVerificationCommand{Fingerprint: status.Fingerprint, Reason: "Approve first document", Decisions: decisions[:1]})
	if err != nil || status.CanOrder || status.Review == nil {
		t.Fatal("partial row approval failed", err)
	}
	page, err = tasks.Page(ctx, "a", taskdomain.Query{})
	if err != nil || pendingSupplierTasks(page) != 1 {
		t.Fatal("partial approval prematurely closed assigned review task", err)
	}
	decisions[0].Decision = "reject"
	decisions[0].Comment = "this is approved"
	status, err = finance.Review(ctx, "admin", vendor.ID, domain.VendorVerificationCommand{Version: status.Review.Version, Fingerprint: status.Fingerprint, Reason: "Fixture review", Decisions: decisions})
	if err != nil || status.State != "rejected" {
		t.Fatal("superuser or comment silently approved saved reject", err)
	}
	decisions[0].Decision = "accept"
	status, err = finance.Review(ctx, "admin", vendor.ID, domain.VendorVerificationCommand{Version: status.Review.Version, Fingerprint: status.Fingerprint, Reason: "Explicit corrected review", Decisions: decisions})
	if err != nil || !status.CanOrder {
		t.Fatal("superuser could not explicitly correct review", err)
	}
	page, err = tasks.Page(ctx, "a", taskdomain.Query{})
	if err != nil || pendingSupplierTasks(page) != 0 {
		t.Fatal("verified supplier task remained", err)
	}
	if _, err = docs.Upload(ctx, "a", vendor.ID, domain.VendorDocumentInput{DocType: "other", Filename: "changed.png", Content: img.Bytes()}, ""); err != nil {
		t.Fatal(err)
	}
	page, err = tasks.Page(ctx, "a", taskdomain.Query{})
	if err != nil || pendingSupplierTasks(page) != 1 {
		t.Fatal("changed evidence did not reopen assigned task", err)
	}
	input.Version = current.Version
	input.UserID = b
	input.Reason = "Finance handover"
	current, err = assignments.Save(ctx, "admin", vendor.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	page, err = tasks.Page(ctx, "a", taskdomain.Query{})
	if err != nil || pendingSupplierTasks(page) != 0 {
		t.Fatal("handover retained old actionable task", err)
	}
	page, err = tasks.Page(ctx, "b", taskdomain.Query{})
	if err != nil || pendingSupplierTasks(page) != 1 {
		t.Fatal("handover did not route task", err)
	}
	// Live role revocation defeats the earlier resolved caller grants.
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='accounts.vendor_verification.write')", role)
	page, err = tasks.Page(ctx, "b", taskdomain.Query{})
	if err != nil || pendingSupplierTasks(page) != 0 {
		t.Fatal("revoked Finance authority retained actionable task", err)
	}
	if _, err = finance.Review(ctx, "b", vendor.ID, domain.VendorVerificationCommand{}); !errors.Is(err, domain.VendorVerificationForbidden) {
		t.Fatal("revoked reviewer could write", err)
	}
	input.Version = current.Version
	input.Active = false
	input.Reason = "Revoke review assignment"
	if _, err = assignments.Save(ctx, "admin", vendor.ID, input); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE supplier_review_assignment_changes SET reason='rewrite'"); err == nil {
		t.Fatal("assignment audit mutable")
	}
}

type rejectedSupplierNotice struct{}

func (rejectedSupplierNotice) SupplierReviewNotice(context.Context, string, string, string, string) error {
	return errors.New("fixture notice rejection")
}

func pendingSupplierTasks(page taskdomain.Page) int {
	n := 0
	for _, task := range page.SupplierReviews {
		if !task.Completed {
			n++
		}
	}
	return n
}
