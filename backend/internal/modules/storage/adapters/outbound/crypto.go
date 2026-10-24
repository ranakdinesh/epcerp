package outbound

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
	"time"
)

type Cipher struct{ aead cipher.AEAD }

func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, domain.Invalid("Invalid document encryption key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead}, nil
}
func CipherFromConfig(config Config) (*Cipher, error) { return NewCipher(config.key[:]) }
func (c *Cipher) Encrypt(data []byte) ([]byte, error) { return c.aead.Seal(nil, nil, data, nil), nil }
func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	if len(data) < 28 {
		return nil, domain.Invalid("Invalid encrypted payload length")
	}
	return c.aead.Open(nil, nil, data, nil)
}
func (c *Cipher) Hash(data []byte) string {
	value := sha256.Sum256(data)
	return hex.EncodeToString(value[:])
}
func (c *Cipher) Algorithm() string { return "AES-256-GCM" }

type SystemValues struct{}

func (SystemValues) Now() int64 { return time.Now().UnixNano() }
func (SystemValues) ID() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	value[6] = (value[6] & 15) | 64
	value[8] = (value[8] & 63) | 128
	s := hex.EncodeToString(value[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
