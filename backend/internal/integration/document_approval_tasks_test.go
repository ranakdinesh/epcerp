package integration

import (
	"bytes"
	"context"
	infra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authrepo "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	authdomain "github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventrepo "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	repo "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	app "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestDocumentPermissionApprovalReviewAndCorrectionTasks(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := t.Context()
	values := repo.VendorDocumentValues{}
	admin, head, worker := values.ID(), values.ID(), values.ID()
	for i, id := range []string{admin, head, worker} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,'fixture',true,$4)", id, []string{"approval_admin", "not_a_role_name", "uploader"}[i], []string{"ad@fixture.invalid", "fh@fixture.invalid", "up@fixture.invalid"}[i], i == 0)
	}
	common := []string{"settings.suppliers.read", "settings.suppliers.write", "settings.supplier_documents.read", "settings.supplier_documents.write", "work.tasks.read", "work.tasks.view"}
	finance := append(append([]string{}, common...), "accounts.vendor_verification.view", "accounts.vendor_verification.read", "accounts.vendor_verification.write", domain.DocumentAutoApprovePermission)
	codes := append([]string{}, finance...)
	for _, code := range codes {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", values.ID(), code)
	}
	for i, user := range []string{head, worker} {
		role := values.ID()
		execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'fixture',true)", role, []string{"arbitrary_capability_bundle", "ordinary_upload_bundle"}[i])
		for _, code := range [][]string{finance, common}[i] {
			execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
		}
		execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", user, role)
	}
	ids := projectIdentities{"admin": {ID: admin, Active: true, Superuser: true}, "head": {ID: head, Active: true, Permissions: finance}, "worker": {ID: worker, Active: true, Permissions: common}}
	users := func(c database.Connection) auth.AssignmentDirectory {
		return authapp.NewAssignmentDirectory(authrepo.NewAssignmentReader(c))
	}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventrepo.NewJournal(c)) }
	lifecycle := func(c database.Connection) ports.DocumentApprovalLifecycle {
		return app.NewDocumentApprovalService(repo.NewDocumentApprovalRepository(c, journal(c)), repo.NewApprovalUserDirectory(users(c)))
	}
	directory := func(c database.Connection) contracts.VendorDirectory { return repo.NewVendorDirectory(c) }
	cipher, e := infra.NewDocumentCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	store := &objects{data: map[string][]byte{}}
	evidence := func(c database.Connection) contracts.VendorDocumentReader {
		return repo.NewVendorDocumentEvidenceReader(c, cipher, infra.NewDocumentInspector(domain.VendorDocumentMaximumBytes)).WithObjects(store)
	}
	docs := app.NewVendorDocumentService(repo.NewVendorDocumentUnitOfWork(pool, directory, journal).WithApprovalLifecycle(lifecycle), infra.NewAuthBridge(ids), cipher, values).WithInspector(infra.NewDocumentInspector(domain.VendorDocumentMaximumBytes)).WithObjects(store, "fixture-documents")
	verification := app.NewVendorVerificationService(repo.NewVendorVerificationUnitOfWork(pool, directory, evidence, journal, users).WithApprovalLifecycle(lifecycle), infra.NewAuthBridge(ids), repo.VendorVerificationValues{})
	masters := app.NewVendorService(repo.NewVendorUnitOfWork(pool, journal), infra.NewAuthBridge(ids), repo.VendorValues{})
	vendor, e := masters.Create(ctx, "admin", domain.VendorCreate{Code: "APPROVAL", Name: "Approval flow", Data: map[string]any{}, Reason: "Fixture"})
	if e != nil {
		t.Fatal(e)
	}
	var img bytes.Buffer
	_ = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	upload := func(token, kind string) *domain.VendorDocumentDocument {
		t.Helper()
		d, e := docs.Upload(ctx, token, vendor.ID, domain.VendorDocumentInput{DocType: kind, Filename: "fixture.png", Content: img.Bytes()}, "")
		if e != nil {
			t.Fatal(e)
		}
		return d
	}
	own := func(user string) []contracts.VendorReviewTask {
		t.Helper()
		var out []contracts.VendorReviewTask
		e := database.NewManager(pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
			var err error
			out, err = repo.NewVendorReviewTasks(c, users(c), directory(c), evidence(c)).Own(ctx, user)
			return err
		})
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	status := func() domain.VendorVerificationStatus {
		t.Helper()
		v, e := verification.Status(ctx, "head", vendor.ID)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	decide := func(id, decision string) {
		t.Helper()
		s := status()
		version := int32(0)
		if s.Review != nil {
			version = s.Review.Version
		}
		_, e := verification.Review(ctx, "head", vendor.ID, domain.VendorVerificationCommand{Version: version, Fingerprint: s.Fingerprint, Reason: "Fixture explicit decision", Decisions: []domain.VendorVerificationDecision{{DocumentID: id, Decision: decision, Comment: "Fixture reviewed"}}})
		if e != nil {
			t.Fatal(e)
		}
	}
	pan := upload("head", "pan")
	if rows := own(head); len(rows) != 0 {
		t.Fatal("auto-approval created review task", rows)
	}
	if s := status(); s.Review == nil || len(s.Review.Decisions) != 1 || s.Review.Decisions[0].DocumentID != pan.ID || s.CanOrder {
		t.Fatal("auto-approved PAN state or missing-cheque gate", s)
	}
	upload("head", "cheque")
	if s := status(); !s.CanOrder || s.State != "verified" || s.Review == nil || s.Review.Version != 0 {
		t.Fatal("fully auto-approved supplier requires an invented manual review", s)
	}
	cheque := upload("worker", "cheque")
	rows := own(head)
	if len(rows) != 1 || rows[0].Completed || rows[0].DocumentID != cheque.ID {
		t.Fatal("ordinary upload not routed to permission-based default reviewer", rows)
	}
	// Source verification requires both read and write; handover metadata must not
	// accept a write-only recipient who cannot even read the current verification.
	e = database.NewManager(pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
		work, e := repo.NewWorkInventory(c).Inventory(ctx, head)
		if e != nil {
			return e
		}
		for _, w := range work {
			if w.Kind == "supplier_document_review" && authdomain.EligibleReplacement(authdomain.User{ID: head, IsActive: true, Roles: []authdomain.Role{{IsActive: true, Permissions: []string{"accounts.vendor_verification.write"}}}}, worker, w) {
				t.Error("review handover accepts a recipient without required source read permission")
			}
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	decide(cheque.ID, "reject")
	rows = own(head)
	if len(rows) != 1 || !rows[0].Completed || rows[0].Outcome != "rejected" {
		t.Fatal("reject did not complete Finance review", rows)
	}
	correction := own(worker)
	if len(correction) != 1 || correction[0].Kind != "correction" || correction[0].Completed {
		t.Fatal("rejection did not create uploader correction", correction)
	}
	replacement := upload("worker", "cheque")
	correction = own(worker)
	if len(correction) != 1 || !correction[0].Completed || correction[0].Outcome != "resubmitted" {
		t.Fatal("resubmission did not complete correction", correction)
	}
	rows = own(head)
	if len(rows) != 2 {
		t.Fatal("resubmission lost completed review or new pending task", rows)
	}
	decide(replacement.ID, "accept")
	if s := status(); !s.CanOrder || len(s.Review.Decisions) != 2 {
		t.Fatal("unrelated approval was reset or complete vendor not green", s)
	}
	upload("head", "other")
	if s := status(); !s.CanOrder || len(s.Review.Decisions) != 3 {
		t.Fatal("auto upload reset unrelated approvals", s)
	}
	if rows := own(head); len(rows) != 2 || !rows[0].Completed || !rows[1].Completed {
		t.Fatal("approve did not complete task", rows)
	}
	upload("admin", "other")
	if !status().CanOrder {
		t.Fatal("superuser upload did not auto-approve")
	}
	execFixture(t, pool, "INSERT INTO supplier_review_assignments(id,supplier_id,user_id,version,is_active,created_by_id,updated_by_id)VALUES($1,$2,$3,1,true,$4,$4)", values.ID(), vendor.ID, head, admin)
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id IN(SELECT role_id FROM auth.user_roles WHERE user_id=$1) AND permission_id=(SELECT id FROM auth.permissions WHERE code=$2)", head, domain.DocumentAutoApprovePermission)
	pending := upload("head", "other")
	if s := status(); s.CanOrder {
		t.Fatal("revoked auto-approval token trusted")
	}
	if rows := own(head); len(rows) != 3 || rows[0].DocumentID != pending.ID || rows[0].Completed {
		t.Fatal("revoked permission upload did not create review", rows)
	}

	execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT ur.role_id,p.id FROM auth.user_roles ur CROSS JOIN auth.permissions p WHERE ur.user_id=$1 AND p.code=$2", worker, domain.DocumentAutoApprovePermission)
	arbitrary := upload("worker", "other")
	found := false
	for _, decision := range status().Review.Decisions {
		if decision.DocumentID == arbitrary.ID && decision.Decision == "accept" {
			found = true
		}
	}
	if !found {
		t.Fatal("permission assigned to arbitrary uploader did not auto-approve")
	}
	if rows := own(head); len(rows) != 3 {
		t.Fatal("arbitrary auto-approval created Finance task")
	}
	decide(pending.ID, "accept")
	legacy := app.NewVendorDocumentService(repo.NewVendorDocumentUnitOfWork(pool, directory, journal), infra.NewAuthBridge(ids), cipher, values).WithInspector(infra.NewDocumentInspector(domain.VendorDocumentMaximumBytes)).WithObjects(store, "fixture-documents")
	historical, e := legacy.Upload(ctx, "worker", vendor.ID, domain.VendorDocumentInput{DocType: "other", Filename: "historical.png", Content: img.Bytes()}, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = docs.ReconcileApprovals(ctx, "admin", vendor.ID); e != nil {
		t.Fatal("explicit historical reconciliation", e)
	}
	found = false
	for _, decision := range status().Review.Decisions {
		if decision.DocumentID == historical.ID && decision.Decision == "accept" {
			found = true
		}
	}
	if !found || !status().CanOrder {
		t.Fatal("historical permitted upload was not reconciled")
	}
	var auditBefore, auditAfter int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM supplier_document_approval_changes").Scan(&auditBefore); e != nil {
		t.Fatal(e)
	}
	if _, e = docs.ReconcileApprovals(ctx, "admin", vendor.ID); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM supplier_document_approval_changes").Scan(&auditAfter); e != nil || auditAfter != auditBefore {
		t.Fatal("reconciliation duplicated decisions", e)
	}

	decide(arbitrary.ID, "reject")
	corrected, e := docs.Upload(ctx, "worker", vendor.ID, domain.VendorDocumentInput{DocType: "other", ReplacesID: arbitrary.ID, Filename: "corrected-other.png", Content: img.Bytes()}, "")
	if e != nil || corrected == nil || !status().CanOrder {
		t.Fatal("explicit Other replacement did not resolve rejection", e)
	}
	resolved := false
	for _, task := range own(worker) {
		if task.DocumentID == arbitrary.ID && task.Kind == "correction" && task.Completed && task.Outcome == "resubmitted" {
			resolved = true
		}
	}
	if !resolved {
		t.Fatal("Other document correction did not complete")
	}
	execFixture(t, pool, `CREATE FUNCTION reject_document_task() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture task failure'; END; $$;CREATE TRIGGER reject_document_task BEFORE INSERT ON supplier_document_tasks FOR EACH ROW EXECUTE FUNCTION reject_document_task();`)
	var before, after int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM supplier_documents").Scan(&before); e != nil {
		t.Fatal(e)
	}
	if _, e = docs.Upload(ctx, "head", vendor.ID, domain.VendorDocumentInput{DocType: "pan", Filename: "rollback.png", Content: img.Bytes()}, ""); e == nil {
		t.Fatal("task failure did not abort upload")
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM supplier_documents").Scan(&after); e != nil || after != before {
		t.Fatal("failed task leaked document", e)
	}
	if !status().CanOrder {
		t.Fatal("failed replacement invalidated the approved original")
	}
	execFixture(t, pool, "DROP TRIGGER reject_document_task ON supplier_document_tasks;DROP FUNCTION reject_document_task()")
	if _, e = pool.Exec(ctx, "UPDATE supplier_document_tasks SET comment='rewrite' WHERE completed_at IS NOT NULL"); e == nil {
		t.Fatal("completed task was mutable")
	}
}
