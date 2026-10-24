package outbound

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestAESGCMEnvelopeAndAuthentication(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	cipher, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("private-document-fixture")
	a, _ := cipher.Encrypt(data)
	b, _ := cipher.Encrypt(data)
	if len(a) != len(data)+28 || string(a) == string(b) || strings.Contains(string(a), string(data)) {
		t.Fatal("nonce/envelope confidentiality failed")
	}
	plain, err := cipher.Decrypt(a)
	if err != nil || string(plain) != string(data) {
		t.Fatal("roundtrip failed")
	}
	a[15] ^= 255
	if _, err = cipher.Decrypt(a); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	if _, err = cipher.Decrypt([]byte("short")); err == nil {
		t.Fatal("invalid envelope accepted")
	}
	if _, err = NewCipher(make([]byte, 16)); err == nil {
		t.Fatal("non AES-256 key accepted")
	}
}
func TestStorageConfigurationKeyParityAndPrivacy(t *testing.T) {
	values := map[string]string{"VTPL_STORAGE_SECRET_KEY": "private-storage-canary", "VTPL_JWT_SECRET_KEY": strings.Repeat("x", 32)}
	lookup := func(key string) (string, bool) { v, ok := values[key]; return v, ok }
	config, err := LoadConfig(lookup)
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte(values["VTPL_JWT_SECRET_KEY"] + ":vtpl_secure_doc_salt"))
	if config.key != expected || config.Bucket != "vtpl-documents" || config.Secure {
		t.Fatal("default key derivation drift")
	}
	values["VTPL_DOCUMENT_ENCRYPTION_KEY"] = ""
	config, err = LoadConfig(lookup)
	if err != nil || config.key != expected {
		t.Fatal("empty legacy encryption setting must use JWT fallback")
	}
	values["VTPL_DOCUMENT_ENCRYPTION_KEY"] = "explicit-fixture-key"
	config, err = LoadConfig(lookup)
	if err != nil || config.key != sha256.Sum256([]byte("explicit-fixture-key")) {
		t.Fatal("explicit key derivation drift")
	}
	if strings.Contains(fmt.Sprintf("%v %+v %#v", config, config, config), "private-storage-canary") {
		t.Fatal("secret formatted")
	}
	for key, value := range map[string]string{"STORAGE_ENDPOINT": "http://wrong/path", "STORAGE_BUCKET": "public/invalid", "STORAGE_SECURE": "invalid", "STORAGE_SECRET_KEY": "", "STORAGE_ACCESS_KEY": ""} {
		full := "VTPL_" + key
		old, present := values[full]
		values[full] = value
		if _, err := LoadConfig(lookup); err == nil || err.Error() != "invalid "+full {
			t.Fatalf("invalid %s accepted", key)
		}
		if present {
			values[full] = old
		} else {
			delete(values, full)
		}
	}
}
