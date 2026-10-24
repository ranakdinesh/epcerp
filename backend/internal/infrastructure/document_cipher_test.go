package infrastructure

import (
	"errors"
	"strings"
	"testing"
)

func TestDocumentEncryptionAndIntegrity(t *testing.T) {
	c, e := NewDocumentCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	input := []byte("private fixture")
	a, hash, e := c.Seal(input)
	if e != nil {
		t.Fatal(e)
	}
	b, _, _ := c.Seal(input)
	if string(a) == string(b) || string(a) == string(input) {
		t.Fatal("nonce or encryption missing")
	}
	data, e := c.Open(a, hash)
	if e != nil || string(data) != string(input) {
		t.Fatal("round trip failed")
	}
	a[len(a)-1] ^= 1
	if _, e = c.Open(a, hash); !errors.Is(e, ErrDocumentIntegrity) {
		t.Fatal("tampering accepted")
	}
	if _, e = c.Open(b, strings.Repeat("0", 64)); !errors.Is(e, ErrDocumentIntegrity) {
		t.Fatal("hash mismatch accepted")
	}
	if _, e = NewDocumentCipher("bad"); e == nil {
		t.Fatal("bad key accepted")
	}
}
