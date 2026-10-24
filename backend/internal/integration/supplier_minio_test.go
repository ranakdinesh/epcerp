package integration

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	infrastructure "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	master "github.com/ranakdinesh/epcERP/internal/modules/master"
	repository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	application "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type trackedSupplierObjects struct {
	ports.VendorDocumentObjects
	keys    []string
	reject  bool
	corrupt bool
}

func (o *trackedSupplierObjects) Put(ctx context.Context, bucket, key string, data []byte) error {
	if o.reject {
		return errors.New("fixture object upload rejected")
	}
	if err := o.VendorDocumentObjects.Put(ctx, bucket, key, data); err != nil {
		return err
	}
	o.keys = append(o.keys, key)
	return nil
}
func (o *trackedSupplierObjects) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	if o.corrupt {
		return []byte("corrupt ciphertext"), nil
	}
	return o.VendorDocumentObjects.Get(ctx, bucket, key)
}
func TestSupplierObjectStorageAndRelocation(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	bucket := "epcerp-supplier-tests"
	var backing ports.VendorDocumentObjects = &objects{data: map[string][]byte{}}
	if os.Getenv("EPCERP_SUPPLIER_MINIO_TEST") == "1" {
		remote, configured, err := master.DocumentObjects()
		if err != nil {
			t.Fatal("MinIO fixture configuration failed")
		}
		backing = remote
		bucket = configured
	}
	store := &trackedSupplierObjects{VendorDocumentObjects: backing}
	if remote, ok := backing.(*infrastructure.ObjectStore); ok {
		t.Cleanup(func() {
			for _, key := range store.keys {
				if err := remote.Remove(context.Background(), bucket, key); err != nil {
					t.Error("MinIO fixture cleanup failed")
				}
			}
		})
	}
	actor := "997bcaf6-e4ec-4809-b7e9-42b94c332891"
	supplier := (repository.VendorDocumentValues{}).ID()
	legacy := (repository.VendorDocumentValues{}).ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'object_fixture','object@example.invalid','fixture',true,true)", actor)
	execFixture(t, pool, "INSERT INTO suppliers(id,code,name,data,is_active) VALUES($1,'OBJECT','Object fixture','{}',true)", supplier)
	cipher, err := infrastructure.NewDocumentCipher(strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	var imageData bytes.Buffer
	_ = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	encrypted, hash, err := cipher.Seal(imageData.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	execFixture(t, pool, `INSERT INTO supplier_documents(id,supplier_id,doc_type,title,original_filename,content_type,byte_size,encrypted_content,sha256,is_active,created_by_id,updated_by_id,created_at,updated_at) VALUES($1,$2,'pan','Legacy','fixture.png','image/png',$3,$4,$5,true,$6,$6,$7,$7)`, legacy, supplier, len(imageData.Bytes()), encrypted, hash, actor, at)
	journal := func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }
	relocate := application.NewVendorDocumentRelocation(repository.NewVendorDocumentRelocationUnitOfWork(pool, journal), store, cipher, repository.VendorDocumentValues{}, bucket)
	// A failed PUT and a failed audit commit must both retain the only DB copy.
	store.reject = true
	if _, err = relocate.Run(ctx); err == nil {
		t.Fatal("rejected object upload migrated metadata")
	}
	store.reject = false
	execFixture(t, pool, `CREATE FUNCTION reject_relocation_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END; $$; CREATE CONSTRAINT TRIGGER reject_relocation_event AFTER INSERT ON business_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_relocation_event();`)
	if _, err = relocate.Run(ctx); err == nil {
		t.Fatal("failed audit relocation committed")
	}
	var inline []byte
	var key string
	if err = pool.QueryRow(ctx, "SELECT encrypted_content FROM supplier_documents WHERE id=$1", legacy).Scan(&inline); err != nil || !bytes.Equal(inline, encrypted) {
		t.Fatal("failed migration lost source")
	}
	execFixture(t, pool, "DROP TRIGGER reject_relocation_event ON business_events")
	if n, err := relocate.Run(ctx); err != nil || n != 1 {
		t.Fatal("relocation failed", n, err)
	}
	if n, err := relocate.Run(ctx); err != nil || n != 0 {
		t.Fatal("relocation not idempotent", n, err)
	}
	var version int
	var updated time.Time
	var updatedBy string
	if err = pool.QueryRow(ctx, "SELECT encrypted_content,object_key,version,updated_at,updated_by_id::text FROM supplier_documents WHERE id=$1", legacy).Scan(&inline, &key, &version, &updated, &updatedBy); err != nil || inline != nil || version != 1 || !updated.Equal(at) || updatedBy != actor {
		t.Fatal("relocation changed evidence identity or retained DB blob", err)
	}
	raw, err := store.Get(ctx, bucket, key)
	if err != nil || !bytes.Equal(raw, encrypted) || bytes.Contains(raw, imageData.Bytes()) {
		t.Fatal("object ciphertext mismatch")
	}
	ids := projectIdentities{"admin": auth.Identity{ID: actor, Active: true, Superuser: true}, "denied": auth.Identity{ID: actor, Active: true}}
	directory := func(c database.Connection) contracts.VendorDirectory { return repository.NewVendorDirectory(c) }
	service := application.NewVendorDocumentService(repository.NewVendorDocumentUnitOfWork(pool, directory, journal), infrastructure.NewAuthBridge(ids), cipher, repository.VendorDocumentValues{}).WithInspector(infrastructure.NewDocumentInspector(domain.VendorDocumentMaximumBytes)).WithObjects(store, bucket)
	_, payload, err := service.Preview(ctx, "admin", supplier, legacy)
	if err != nil || !bytes.Equal(payload, imageData.Bytes()) {
		t.Fatal("relocated preview failed", err)
	}
	reader := repository.NewVendorDocumentEvidenceReader(pool, cipher, infrastructure.NewDocumentInspector(domain.VendorDocumentMaximumBytes)).WithObjects(store)
	evidence, err := reader.Active(ctx, supplier)
	if err != nil || len(evidence) != 1 || !evidence[0].Previewable || evidence[0].SHA256 != hash {
		t.Fatal("Finance evidence reader failed", err)
	}
	store.corrupt = true
	if _, payload, err = service.Download(ctx, "admin", supplier, legacy); err == nil || payload != nil {
		t.Fatal("corrupt object released bytes")
	}
	if _, err = reader.Active(ctx, supplier); err == nil {
		t.Fatal("corrupt object passed Finance evidence gate")
	}
	store.corrupt = false
	if _, payload, err = service.Download(ctx, "denied", supplier, legacy); err == nil || payload != nil {
		t.Fatal("denied object download")
	}
	input := domain.VendorDocumentInput{DocType: "cheque", Filename: "fixture.png", Content: imageData.Bytes()}
	store.reject = true
	if _, err = service.Upload(ctx, "admin", supplier, input, "Fixture upload"); err == nil {
		t.Fatal("failed object upload accepted")
	}
	store.reject = false
	uploaded, err := service.Upload(ctx, "admin", supplier, input, "Fixture upload")
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.ObjectKey == "" || len(uploaded.Ciphertext) != 0 {
		t.Fatal("new document was not externalized")
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM supplier_documents WHERE encrypted_content IS NOT NULL").Scan(&count); err != nil || count != 0 {
		t.Fatal("new upload stored a database blob")
	}
	// Once external evidence exists the down migration must refuse to discard object references.
	if _, err = pool.Exec(ctx, "ALTER TABLE supplier_documents ALTER COLUMN encrypted_content SET NOT NULL"); err == nil {
		t.Fatal("unsafe rollback permitted")
	}
}
