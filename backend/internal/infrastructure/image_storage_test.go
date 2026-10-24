package infrastructure

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
)

type imageObjectsFixture struct {
	data   []byte
	reject bool
	writes int
}

func (o *imageObjectsFixture) Put(_ context.Context, _, _ string, data []byte) error {
	if o.reject {
		return errors.New("fixture storage failure")
	}
	o.data = bytes.Clone(data)
	o.writes++
	return nil
}
func (o *imageObjectsFixture) Get(context.Context, string, string) ([]byte, error) {
	return bytes.Clone(o.data), nil
}
func imageCipher(t *testing.T) *DocumentCipher {
	t.Helper()
	var key [32]byte
	_, _ = rand.Read(key[:])
	c, e := NewDocumentCipher(hex.EncodeToString(key[:]))
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestImageStorageEncryptionIntegrityAndBounds(t *testing.T) {
	ctx := context.Background()
	o := &imageObjectsFixture{}
	s, e := NewImageStorage(o, imageCipher(t), "fixture-images", 64)
	if e != nil {
		t.Fatal(e)
	}
	plain := []byte("canonical-image-fixture")
	hash, e := s.Store(ctx, "images/fixture.enc", plain)
	if e != nil || len(hash) != 64 || bytes.Equal(o.data, plain) {
		t.Fatal("unencrypted object", e)
	}
	got, e := s.Retrieve(ctx, "images/fixture.enc", hash)
	if e != nil || !bytes.Equal(got, plain) {
		t.Fatal("roundtrip", e)
	}
	if _, e = s.Retrieve(ctx, "images/fixture.enc", string(bytes.Repeat([]byte("0"), 64))); e == nil {
		t.Fatal("wrong digest accepted")
	}
	o.data[0] ^= 1
	if _, e = s.Retrieve(ctx, "images/fixture.enc", hash); e == nil {
		t.Fatal("tampering accepted")
	}
	o.data[0] ^= 1
	for _, key := range []string{"../outside", "images/../outside", "images/a\\b", "images/a\x00b"} {
		if _, e = s.Store(ctx, key, plain); e == nil {
			t.Fatal("invalid key accepted")
		}
	}
	for _, data := range [][]byte{nil, make([]byte, 65)} {
		if _, e = s.Store(ctx, "images/fixture.enc", data); e == nil {
			t.Fatal("size bound ignored")
		}
	}
	if o.writes != 1 {
		t.Fatal("invalid uploads reached storage")
	}
	o.reject = true
	if _, e = s.Store(ctx, "images/new.enc", plain); e == nil {
		t.Fatal("storage failure lost")
	}
	o.data = make([]byte, 93)
	if _, e = s.Retrieve(ctx, "images/fixture.enc", hash); e == nil {
		t.Fatal("read bound ignored")
	}
}

// Optional real S3 transport gate runs against a disposable MinIO instance.
func TestMinIOEncryptedImageRoundTrip(t *testing.T) {
	if os.Getenv("EPCERP_GO_MINIO_INTEGRATION") != "1" {
		t.Skip("disposable MinIO gate not enabled")
	}
	ctx := context.Background()
	o, e := NewObjectStore(ObjectStoreConfig{Endpoint: os.Getenv("EPCERP_MINIO_TEST_ENDPOINT"), AccessKey: os.Getenv("EPCERP_MINIO_TEST_ACCESS_KEY"), SecretKey: os.Getenv("EPCERP_MINIO_TEST_SECRET_KEY")}, 1052)
	if e != nil {
		t.Fatal("MinIO client creation failed")
	}
	s, e := NewImageStorage(o, imageCipher(t), "image-fixture-"+hex.EncodeToString([]byte(rand.Text()[:8])), 1024)
	if e != nil {
		t.Fatal(e)
	}
	key := "images/fixture.enc"
	plain := []byte("private-image-roundtrip")
	hash, e := s.Store(ctx, key, plain)
	if e != nil {
		t.Fatal("MinIO upload failed")
	}
	defer func() {
		if e := o.Remove(ctx, s.bucket, key); e != nil {
			t.Error("MinIO fixture cleanup failed")
		}
	}()
	encrypted, e := o.Get(ctx, s.bucket, key)
	if e != nil || bytes.Equal(encrypted, plain) {
		t.Fatal("encrypted MinIO object unavailable")
	}
	got, e := s.Retrieve(ctx, key, hash)
	if e != nil || !bytes.Equal(got, plain) {
		t.Fatal("MinIO decryption failed")
	}
}
