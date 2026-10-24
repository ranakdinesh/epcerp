package integration

import (
	"bytes"
	"context"
	"github.com/go-chi/chi/v5"
	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"

	masterHTTP "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupplierHTTPUploadPreviewAndStrictReview(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actor := "135bcaf6-e4ec-4809-b7e9-42b94c332891"
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'supplier_http','fixture@example.invalid','fixture',true,false)", actor)

	role := (masterRepository.VendorVerificationValues{}).ID()
	execFixture(t, pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'supplier review fixture','test',true)", role)
	for _, code := range []string{"settings.suppliers.read", "accounts.vendor_verification.read", "accounts.vendor_verification.write"} {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", (masterRepository.VendorVerificationValues{}).ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", actor, role)
	identities := projectIdentities{"admin": auth.Identity{ID: actor, Active: true, Superuser: true}, "finance": auth.Identity{ID: actor, Active: true, Permissions: []string{"accounts.vendor_verification.read", "accounts.vendor_verification.write"}}}
	journal := func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }
	directory := func(c database.Connection) masterContracts.VendorDirectory {
		return masterRepository.NewVendorDirectory(c)
	}
	store := &objects{data: map[string][]byte{}}
	cipher, e := documentInfra.NewDocumentCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	master := masterApplication.NewVendorService(masterRepository.NewVendorUnitOfWork(pool, journal), documentInfra.NewAuthBridge(identities), masterRepository.VendorValues{})
	docs := masterApplication.NewVendorDocumentService(masterRepository.NewVendorDocumentUnitOfWork(pool, directory, journal), documentInfra.NewAuthBridge(identities), cipher, masterRepository.VendorDocumentValues{}).WithObjects(store, "fixture-documents").WithInspector(documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes))
	finance := masterApplication.NewVendorVerificationService(masterRepository.NewVendorVerificationUnitOfWork(pool, directory, func(c database.Connection) masterContracts.VendorDocumentReader {
		return masterRepository.NewVendorDocumentEvidenceReader(c, cipher, documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)).WithObjects(store)
	}, journal, directorUsers), documentInfra.NewAuthBridge(identities), masterRepository.VendorVerificationValues{})
	v, e := master.Create(ctx, "admin", masterDomain.VendorCreate{Code: "HTTP", Name: "HTTP fixture", Data: map[string]any{}, Reason: "Fixture setup"})
	if e != nil {
		t.Fatal(e)
	}
	mux := chi.NewRouter()
	masterHTTP.RegisterVendorRoutes(mux, master)
	masterHTTP.RegisterVendorDocumentRoutes(mux, docs)
	masterHTTP.RegisterVendorVerificationRoutes(mux, finance)
	var pngFile bytes.Buffer
	_ = png.Encode(&pngFile, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	upload := func(file []byte, extra bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("doc_type", "pan")
		part, _ := writer.CreateFormFile("file", "fixture.png")
		_, _ = part.Write(file)
		if extra {
			_ = writer.WriteField("unsafe", "unknown")
		}
		_ = writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/suppliers/"+v.ID+"/documents", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer admin")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	if response := upload([]byte("<html>renamed.png</html>"), false); response.Code != 422 {
		t.Fatal("renamed HTML accepted", response.Code)
	}
	if response := upload(pngFile.Bytes(), true); response.Code != 422 {
		t.Fatal("unknown multipart field accepted", response.Code)
	}
	response := upload(pngFile.Bytes(), false)
	if response.Code != 201 {
		t.Fatal("native upload failed", response.Code)
	}
	var stored masterDomain.VendorDocumentDocument
	if e = masterDTO.UnmarshalJSON(response.Body.Bytes(), &stored); e != nil {
		t.Fatal(e)
	}
	var auditReason, auditActor string
	if e = pool.QueryRow(ctx, "SELECT reason,created_by_id::text FROM supplier_document_changes WHERE document_id=$1 AND action='uploaded'", stored.ID).Scan(&auditReason, &auditActor); e != nil || auditReason != "Supplier document uploaded" || auditActor != actor {
		t.Fatal("upload without a reason lost its automatic actor audit", e)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/suppliers/"+v.ID+"/documents/"+stored.ID+"/preview", nil)
	req.Header.Set("Authorization", "Bearer finance")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != 200 || response.Header().Get("Content-Type") != "image/png" || !strings.HasPrefix(response.Header().Get("Content-Disposition"), "inline") || !bytes.Equal(response.Body.Bytes(), pngFile.Bytes()) {
		t.Fatal("inline preview failed", response.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/suppliers/"+v.ID+"/verification", strings.NewReader(`{"version":0,"unexpected":true}`))
	req.Header.Set("Authorization", "Bearer finance")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != 422 {
		t.Fatal("unknown review field accepted", response.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/suppliers/"+v.ID+"/documents/"+stored.ID+"/preview", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != 401 {
		t.Fatal("anonymous preview accepted", response.Code)
	}
	if e = masterRepository.NewVendorVerificationGate(pool, directory(pool), masterRepository.NewVendorDocumentEvidenceReader(pool, cipher, documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)).WithObjects(store)).RequirePurchaseOrder(ctx, v.ID); e == nil {
		t.Fatal("pool-only PO gate allowed")
	}
}
