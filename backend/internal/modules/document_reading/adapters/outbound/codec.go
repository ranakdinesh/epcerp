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
	evidence := domain.BuildEvidence(r)
	fmt.Fprintf(&out, "# Derived reading (%s)\n\nOriginal reading and source remain authoritative. %d segments; %d empty; %d repeated furniture blocks retained in the appendix.\n\n", evidence.Version, evidence.Segments, evidence.EmptySegments, evidence.Suppressed)
	for _, b := range evidence.Blocks {
		if b.Suppressed {
			continue
		}
		fmt.Fprintf(&out, "## %s (%s; %s; bytes %d-%d)\n\n%s\n\n", b.Label, b.Method, b.Kind, b.Start, b.End, b.Text)
	}
	if evidence.Suppressed > 0 {
		out.WriteString("# Repeated page furniture (retained, lower search priority)\n\n")
		for _, b := range evidence.Blocks {
			if b.Suppressed {
				fmt.Fprintf(&out, "## %s / bytes %d-%d\n\n%s\n\n", b.Label, b.Start, b.End, b.Text)
			}
		}
	}
	return []byte(out.String()), nil
}
