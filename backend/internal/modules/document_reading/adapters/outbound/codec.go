package outbound

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"strings"
)

type Cipher struct{ a cipher.AEAD }

func NewCipher(key string) (*Cipher, error) {
	b, e := hex.DecodeString(key)
	if e != nil || len(b) != 32 {
		return nil, fmt.Errorf("document encryption requires a persistent 32-byte hex key")
	}
	block, e := aes.NewCipher(b)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCMWithRandomNonce(block)
	return &Cipher{a}, e
}
func (c *Cipher) Hash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func (c *Cipher) Seal(b []byte) ([]byte, string, error) {
	return c.a.Seal(nil, nil, b, nil), c.Hash(b), nil
}
func (c *Cipher) Open(b []byte, hash string) ([]byte, error) {
	plain, e := c.a.Open(nil, nil, b, nil)
	if e != nil {
		return nil, domain.Invalid("Stored document integrity check failed")
	}
	if subtle.ConstantTimeCompare([]byte(c.Hash(plain)), []byte(hash)) != 1 {
		return nil, domain.Invalid("Stored document integrity check failed")
	}
	return plain, nil
}

type Codec struct{}

func (Codec) Encode(r domain.Result) ([]byte, error) {
	if e := domain.CheckResult(r); e != nil {
		return nil, e
	}
	b, e := json.Marshal(r)
	if len(b) > 8<<20 {
		return nil, domain.Invalid("Reading exceeds the 8 MiB result limit; split the source document")
	}
	return b, e
}
func (Codec) Text(b []byte) ([]byte, error) {
	var r domain.Result
	if e := json.Unmarshal(b, &r); e != nil {
		return nil, e
	}
	var out strings.Builder
	for _, s := range r.Segments {
		fmt.Fprintf(&out, "## %s (%s)\n\n%s\n\n", s.Label, s.Method, s.Text)
	}
	return []byte(out.String()), nil
}
