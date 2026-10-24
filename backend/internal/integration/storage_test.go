package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"

	authIdentity "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/identity"
	authOut "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authOutSecurity "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	authApp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	authDomain "github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	storageOut "github.com/ranakdinesh/epcERP/internal/modules/storage/adapters/outbound"
	storageApp "github.com/ranakdinesh/epcERP/internal/modules/storage/application"
	storageDomain "github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
	"sync"
	"testing"
	"time"
)

// Objects is the outbound object port fixture. Real PostgreSQL and real AES-GCM
// are used; this test does not claim S3/MinIO network integration.
type objects struct {
	mu     sync.Mutex
	data   map[string][]byte
	reject bool
}

func (o *objects) Put(_ context.Context, bucket, key string, data []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.reject {
		return errors.New("fixture PUT denied")
	}
	o.data[bucket+"/"+key] = bytes.Clone(data)
	return nil
}
func (o *objects) Get(_ context.Context, bucket, key string) ([]byte, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	data, ok := o.data[bucket+"/"+key]
	if !ok {
		return nil, errors.New("missing fixture object")
	}
	return bytes.Clone(data), nil
}
func TestNativeStorageTransactions(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	passwords, err := authOutSecurity.NewPasswords()
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := authOutSecurity.NewTokens(authOutSecurity.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "epcERP", Audience: "epcERP-api", AccessLifetime: 15 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	auth := authApp.NewSessions(authOut.NewSessionUnitOfWork(pool), passwords, tokens, authDomain.Policy{SessionLifetime: time.Hour, PresenceTimeout: time.Minute})
	password := "Aa1!" + rand.Text()
	if _, err = authApp.NewBootstrap(authOut.NewBootstrapUnitOfWork(pool), passwords, tokens).Bootstrap(ctx, "storage_admin", "storage@example.invalid", password); err != nil {
		t.Fatal(err)
	}
	pair, err := auth.Login(ctx, "storage_admin", password)
	if err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	rand.Read(key)
	cipher, err := storageOut.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	store := &objects{data: map[string][]byte{}}
	service := storageApp.New(storageOut.NewUnitOfWork(pool), storageOut.NewAuthBridge(authIdentity.NewIdentityResolver(auth)), store, cipher, storageOut.SystemValues{}, "epcerp-private")
	input := storageDomain.Upload{EntityType: "project", EntityID: "00000000-0000-4000-8000-000000000001", DocType: "project_document", Title: "Fixture evidence", Filename: "fixture.txt", ContentType: "application/pdf", Data: []byte("private native fixture evidence")}
	doc, err := service.Upload(ctx, pair.AccessToken, input)
	if err != nil {
		t.Fatal(err)
	}
	encrypted := bytes.Clone(store.data[doc.Bucket+"/"+doc.Key])
	if bytes.Contains(encrypted, input.Data) {
		t.Fatal("object fixture contains plaintext")
	}
	_, payload, err := service.Download(ctx, pair.AccessToken, doc.ID)
	if err != nil || !bytes.Equal(payload, input.Data) {
		t.Fatal("native download integrity failed", err)
	}
	// Audit failure must prevent releasing bytes, and rollback lifecycle metadata.
	_, err = pool.Exec(ctx, `CREATE FUNCTION reject_storage_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture audit rejected'; END; $$; CREATE CONSTRAINT TRIGGER reject_storage_audit AFTER INSERT ON stored_document_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_storage_audit()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, payload, err = service.Download(ctx, pair.AccessToken, doc.ID); err == nil || payload != nil {
		t.Fatal("download released bytes before audit commit")
	}
	if err = service.Deactivate(ctx, pair.AccessToken, doc.ID); err == nil {
		t.Fatal("failed audit committed deactivation")
	}
	if result, err := service.Upload(ctx, pair.AccessToken, input); err == nil || result.ID != "" {
		t.Fatal("failed audit returned successful upload")
	}
	var active bool
	pool.QueryRow(ctx, "SELECT is_active FROM stored_documents WHERE id=$1", doc.ID).Scan(&active)
	if !active {
		t.Fatal("failed lifecycle audit retained metadata change")
	}
	pool.Exec(ctx, "DROP TRIGGER reject_storage_audit ON stored_document_events; DROP FUNCTION reject_storage_audit()")
	store.reject = true
	if _, err = service.Upload(ctx, pair.AccessToken, input); err == nil {
		t.Fatal("failed object PUT accepted")
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM stored_documents").Scan(&count)
	if count != 1 {
		t.Fatal("failed PUT/audit retained metadata")
	}
	store.reject = false
	store.data[doc.Bucket+"/"+doc.Key][0] ^= 1
	if _, payload, err = service.Download(ctx, pair.AccessToken, doc.ID); err == nil || payload != nil {
		t.Fatal("corrupt ciphertext released payload")
	}
	store.data[doc.Bucket+"/"+doc.Key] = encrypted
	if err = service.Deactivate(ctx, pair.AccessToken, doc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Metadata(ctx, pair.AccessToken, doc.ID); err == nil {
		t.Fatal("inactive metadata accessible")
	}
	if _, ok := store.data[doc.Bucket+"/"+doc.Key]; !ok {
		t.Fatal("deactivation deleted evidence")
	}
	t.Log("Go-owned storage persistence, encryption, deferred audit/PUT rollback and no pre-audit download payload passed; object-store network adapter not exercised")
}
