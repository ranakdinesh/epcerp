package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/sql/migrations"

	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
)

// This fixture enters from actual Finance-reviewed quotations and independently
// released LOI, then uses real owned encrypted PNG uploads and native transactions.
func verifyNativeTechnicalKit(t *testing.T, f purchaseFixture, ids projectIdentities, p func(database.Connection) purchaseRepository.IndentsProviders, wp func(database.Connection) purchaseRepository.WorkflowProviders, ws *purchaseApp.WorkflowService, pkg string, financeFactory purchaseRepository.QuotationApprovalsFactory) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_technical.read", "projects.purchase_technical.write", "projects.purchase_technical.approve"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native technical',true)", role, "Native technical "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native technical')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	writer := ids["loi-writer"]
	writer.Permissions = append(append([]string{}, writer.Permissions...), codes...)
	ids["technical-writer"] = writer
	ids["technical-reviewer"] = ids["loi-manager"]
	packageView, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	rows := []purchaseDomain.WorkflowAssignment{}
	for _, n := range []int{6, 7, 8} {
		rows = append(rows, purchaseDomain.WorkflowAssignment{Number: n, Team: "Technical", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.TechnicalToday(v.Now())})
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: packageView.Package.Version, Reason: "Assign real technical steps", Rows: rows}); e != nil {
		t.Fatal(e)
	}
	factory := func(c database.Connection) purchasePorts.TechnicalContext {
		x := p(c)
		return purchaseRepository.NewTechnicalContext(purchaseRepository.TechnicalProviders{Projects: x.Projects, Purchase: x.Scope, Access: accessout.NewGate(c, accessProviders(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Work: purchaseRepository.NewWorkflowSourceReader(c, wp(c)), Released: purchaseRepository.NewSourcingReleasedReader(c)})
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.TechnicalService {
		return purchaseApp.NewTechnical(purchaseRepository.NewTechnicalUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	w, e := service.Execute(ctx, "technical-writer", f.project, pkg, purchaseDomain.TechnicalCommand{Action: "configure", Reason: "Actual released LOI checklist"})
	if e != nil || w.Case == nil || len(w.Case.Documents) != 6 || w.Ready {
		t.Fatal("native technical kit", e)
	}
	var key [32]byte
	if _, e = rand.Read(key[:]); e != nil {
		t.Fatal(e)
	}
	cipher, e := documentInfra.NewDocumentCipher(hex.EncodeToString(key[:]))
	if e != nil {
		t.Fatal(e)
	}
	evidence := purchaseApp.NewTechnicalEvidence(service, cipher, purchaseRepository.TechnicalInspector{})
	api := chi.NewRouter()
	api.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" && purchaseHTTP.TechnicalIsUploadPath(r.URL.Path) {
				purchaseHTTP.TechnicalUploadGuard(evidence)(httpmiddleware.PayloadLimit(11<<20)(next)).ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	})
	purchaseHTTP.TechnicalRegister(api, service, evidence)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/technical"
	request := func(method, path, token string, body io.Reader, contentType string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, body)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	probe := &technicalReadProbe{}
	denied := request("POST", base+"/uploads/document/"+strconv.Itoa(int(w.Case.Version)), "", probe, "multipart/form-data; boundary=test")
	if denied.Code != 401 || probe.read {
		t.Fatal("unauthorized technical body buffered", denied.Code, probe.read)
	}

	var b bytes.Buffer
	if e = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
		t.Fatal(e)
	}
	bytesIn := b.Bytes()
	today := purchaseDomain.TechnicalToday(v.Now())
	yes, no := true, false
	run := func(token string, q purchaseDomain.TechnicalCommand) {
		t.Helper()
		q.Version = w.Case.Version

		raw, err := json.Marshal(q)
		if err != nil {
			t.Fatal(err)
		}
		response := request("POST", base, token, bytes.NewReader(raw), "application/json")
		if response.Code != 200 {
			t.Fatal(q.Action, response.Code, response.Body.String())
		}
		if e = json.Unmarshal(response.Body.Bytes(), &w); e != nil {
			t.Fatal(e)
		}
	}
	upload := func(purpose string) purchaseDomain.TechnicalFile {
		t.Helper()

		path := base + "/uploads/" + purpose + "/" + strconv.Itoa(int(w.Case.Version))
		authority := request("GET", path, "technical-writer", nil, "")
		if authority.Code != 204 {
			t.Fatal("upload preflight", authority.Code, authority.Body.String())
		}
		var body bytes.Buffer
		multi := multipart.NewWriter(&body)
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="file"; filename="actual-evidence.png"`)
		header.Set("Content-Type", "image/png")
		part, err := multi.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(bytesIn); err != nil {
			t.Fatal(err)
		}
		if err = multi.WriteField("reason", "Native actual source evidence"); err != nil {
			t.Fatal(err)
		}
		if err = multi.Close(); err != nil {
			t.Fatal(err)
		}
		response := request("POST", path, "technical-writer", &body, multi.FormDataContentType())
		if response.Code != 201 {
			t.Fatal("native multipart upload", purpose, response.Code, response.Body.String())
		}
		var file purchaseDomain.TechnicalFile
		if err = json.Unmarshal(response.Body.Bytes(), &file); err != nil {
			t.Fatal(err)
		}
		return file
	}
	for _, doc := range append([]purchaseDomain.TechnicalDocument{}, w.Case.Documents...) {
		if !doc.Required {
			continue
		}
		file := upload("document")
		run("technical-writer", purchaseDomain.TechnicalCommand{Action: "document", Reason: "Receive actual vendor revision", Document: &purchaseDomain.TechnicalDocumentInput{ID: doc.ID, Kind: doc.Kind, Title: doc.Title, Category: doc.Category, Standard: doc.Standard, Required: &yes, Active: &yes, Status: "received", Revision: "Vendor Rev 1", ReceivedDate: today, FileID: file.ID}})
	}
	verifyTechnicalProgress(t, f, factory, pkg, [3]string{"completed", "not_started", "not_started"})
	transmittal := upload("transmittal")
	run("technical-writer", purchaseDomain.TechnicalCommand{Action: "submit", Reason: "Submit exact full kit", Submission: &purchaseDomain.TechnicalSubmissionInput{Reference: "Native transmittal", Date: today, TargetDate: purchaseDomain.TechnicalToday(v.Now().AddDate(0, 0, 7)), Authority: "Recorded external client", FileID: transmittal.ID}})
	verifyTechnicalProgress(t, f, factory, pkg, [3]string{"completed", "completed", "in_progress"})
	if _, e = service.Execute(ctx, "technical-writer", f.project, pkg, purchaseDomain.TechnicalCommand{Version: w.Case.Version, Action: "team_review", Reason: "Try self review", Approve: &yes, Remarks: "Self review"}); e == nil {
		t.Fatal("native self review accepted")
	}
	run("technical-reviewer", purchaseDomain.TechnicalCommand{Action: "team_review", Reason: "Actual independent review", Approve: &yes, Remarks: "Exact submitted revision reviewed"})
	letter := upload("client_approval")
	client := &purchaseDomain.TechnicalApprovalInput{Status: "approved_with_comments", Reference: "Native actual client letter", Date: today, Name: "Recorded client reviewer", Designation: "Client engineer", FileID: letter.ID, Remarks: "Actual annotated conditions"}
	run("technical-writer", purchaseDomain.TechnicalCommand{Action: "client_approval", Reason: "Record actual conditional client decision", Approval: client, QAPApproved: &yes, ConditionsClosed: &no})
	if w.Ready {
		t.Fatal("open client conditions grant PO readiness")
	}
	closure := upload("conditions")
	client.ClosureFileID = closure.ID
	run("technical-writer", purchaseDomain.TechnicalCommand{Action: "client_approval", Reason: "Record actual conditions closure", Approval: client, QAPApproved: &yes, ConditionsClosed: &yes})
	if !w.Ready || len(w.History) != 10 {
		t.Fatal("native cleared technical kit", len(w.History))
	}

	verifyTechnicalProgress(t, f, factory, pkg, [3]string{"completed", "completed", "completed"})
	verifyNativePurchaseOrder(t, f, ids, p, wp, ws, pkg, financeFactory)

	httpFile := request("GET", base+"/files/"+letter.ID, "technical-writer", nil, "")
	if httpFile.Code != 200 || httpFile.Header().Get("Content-Type") != "image/png" || !bytes.Equal(httpFile.Body.Bytes(), bytesIn) {
		t.Fatal("native technical HTTP view", httpFile.Code)
	}
	strict := request("POST", base, "technical-writer", strings.NewReader(`{"action":"revise","version":1,"reason":"Reject forged field","recorded_by_id":"forged"}`), "application/json")
	if strict.Code != 422 {
		t.Fatal("forged actor HTTP accepted", strict.Code)
	}
	_, plain, e := evidence.Download(ctx, "technical-writer", f.project, pkg, letter.ID)
	if e != nil || !bytes.Equal(plain, bytesIn) {
		t.Fatal("native authenticated file read", e)
	}
	if _, plain, e = evidence.Download(ctx, "technical-writer", f.otherProject, pkg, letter.ID); e == nil || plain != nil {
		t.Fatal("cross-project file exposed")
	}
	var stored []byte
	if e = f.pool.QueryRow(ctx, "SELECT ciphertext FROM purchase_technical_files WHERE id=$1", letter.ID).Scan(&stored); e != nil || bytes.Equal(stored, bytesIn) {
		t.Fatal("unencrypted evidence", e)
	}
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	failedFiles := purchaseApp.NewTechnicalEvidence(failed, cipher, purchaseRepository.TechnicalInspector{})
	if file, e := failedFiles.Upload(ctx, "technical-writer", f.project, pkg, purchaseDomain.TechnicalUpload{Version: w.Case.Version, Purpose: "conditions", Name: "actual-evidence.png", MediaType: "image/png", Reason: "Fail late file journal", Data: bytesIn}); e == nil || file.ID != "" {
		t.Fatal("late upload audit failure exposed file")
	}
	if _, plain, e = failedFiles.Download(ctx, "technical-writer", f.project, pkg, letter.ID); e == nil || plain != nil {
		t.Fatal("late read audit failure exposed bytes")
	}
	if result, e := failed.Execute(ctx, "technical-writer", f.project, pkg, purchaseDomain.TechnicalCommand{Version: w.Case.Version, Action: "revise", Reason: "Fail late kit journal"}); e == nil || result.Case != nil {
		t.Fatal("failed kit commit exposed revision")
	}
	unchanged, e := service.Workspace(ctx, "technical-reviewer", f.project, pkg)
	if e != nil || !unchanged.Ready || unchanged.Case.Version != w.Case.Version {
		t.Fatal("failed audit changed actual readiness", e)
	}

	if _, e = f.pool.Exec(ctx, "UPDATE purchase_technical_cases SET version=version+1,data=jsonb_set(data,'{version}',to_jsonb(version+1)) WHERE id=$1", w.Case.ID); e == nil {
		t.Fatal("unaudited kit write accepted")
	}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_technical.read')", role)
	if _, plain, e = evidence.Download(ctx, "technical-writer", f.project, pkg, letter.ID); e == nil || plain != nil {
		t.Fatal("revoked actual permission exposed file")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_technical.read'", role)
	if _, e = f.pool.Exec(ctx, "DELETE FROM purchase_technical_files WHERE id=$1", letter.ID); e == nil {
		t.Fatal("immutable source evidence deleted")
	}
	if _, e = f.pool.Exec(ctx, "INSERT INTO purchase_technical_files SELECT $1,case_id,purpose,name,media_type,byte_size,sha256,ciphertext,is_active,created_at,updated_at,created_by_id,updated_by_id FROM purchase_technical_files WHERE id=$2", v.ID(), letter.ID); e == nil {
		t.Fatal("unaudited file insert accepted")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "technical-reviewer", f.project, pkg)
	if e != nil || held.Case == nil || held.Ready || len(held.Actions) != 0 {
		t.Fatal("held kit lost evidence or retained authority", e)
	}
	verifyTechnicalProgress(t, f, factory, pkg, [3]string{"completed", "completed", "completed"})
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	run("technical-writer", purchaseDomain.TechnicalCommand{Action: "revise", Reason: "Actual correction invalidates previous client approval"})
	if w.Ready || w.Case.Approval != nil || w.Case.Submission != nil {
		t.Fatal("revised kit retained obsolete approval")
	}
	verifyTechnicalProgress(t, f, factory, pkg, [3]string{"completed", "not_started", "not_started"})
	// Test the owning down guard in a rolled-back transaction. This fixture
	// continues exercising current services afterward; a failed multi-migration
	// rollback must not leave its newer notice schema partially downgraded.
	rollbackSQL, e := migrations.Files.ReadFile("000057_finance_accounts_acceptance.down.sql")
	if e != nil {
		t.Fatal(e)
	}
	tx, e := f.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(ctx, strings.TrimSuffix(strings.TrimPrefix(string(rollbackSQL), "BEGIN;"), "COMMIT;\n"))
	_ = tx.Rollback(ctx)
	if e == nil || !strings.Contains(e.Error(), "populated Accounts acceptance") {
		t.Fatal("populated Accounts acceptance rollback allowed", e)
	}

	var retained int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_technical_files WHERE case_id=$1", w.Case.ID).Scan(&retained); e != nil || retained != 8 {
		t.Fatal("rollback lost actual technical evidence", retained, e)
	}

}

// A rejected upload must never invoke Read, even when its body size is unknown.
type technicalReadProbe struct{ read bool }

func (p *technicalReadProbe) Read([]byte) (int, error) { p.read = true; return 0, io.EOF }

func verifyTechnicalProgress(t *testing.T, f purchaseFixture, factory func(database.Connection) purchasePorts.TechnicalContext, pkg string, want [3]string) {
	t.Helper()
	ctx := t.Context()
	manager := database.NewManager(f.pool)
	if e := manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		r := purchaseRepository.NewTechnicalProgressReader(c, factory(c))
		result, e := r.Progress(ctx, f.project, pkg, true)
		if e != nil {
			return e
		}
		if !result.Current || len(result.Steps) != 3 {
			t.Fatal("current technical reporting projection", result)
		}
		for i, state := range want {
			if result.Steps[i].State != state {
				t.Fatal("technical report stage", i+6, result.Steps[i].State, state)
			}
		}
		unavailable, e := r.Progress(ctx, f.project, pkg, false)
		if e != nil {
			return e
		}
		if unavailable.Current || unavailable.Steps[0].State != "blocked" {
			t.Fatal("unverified report parent credited")
		}
		return nil
	}); e != nil {
		t.Fatal("native technical progress", e)
	}
	if _, e := purchaseRepository.NewTechnicalProgressReader(f.pool, factory(f.pool)).Progress(ctx, f.project, pkg, true); e == nil {
		t.Fatal("unbound technical progress")
	}
}
