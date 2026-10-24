package infrastructure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
)

var ErrDocumentIntegrity = errors.New("document integrity verification failed")

type DocumentCipher struct{ aead cipher.AEAD }

func NewDocumentCipher(key string) (*DocumentCipher, error) {
	raw, e := hex.DecodeString(key)
	if e != nil || len(raw) != 32 {
		return nil, errors.New("Document encryption key must be a 32-byte hex value")
	}
	block, e := aes.NewCipher(raw)
	if e != nil {
		return nil, e
	}
	aead, e := cipher.NewGCMWithRandomNonce(block)
	if e != nil {
		return nil, e
	}
	return &DocumentCipher{aead}, nil
}
func (c *DocumentCipher) Hash(data []byte) string {
	v := sha256.Sum256(data)
	return hex.EncodeToString(v[:])
}
func (c *DocumentCipher) Seal(data []byte) ([]byte, string, error) {
	return c.aead.Seal(nil, nil, data, nil), c.Hash(data), nil
}
func (c *DocumentCipher) Open(data []byte, hash string) ([]byte, error) {
	plain, e := c.aead.Open(nil, nil, data, nil)
	if e != nil {
		return nil, ErrDocumentIntegrity
	}
	if subtle.ConstantTimeCompare([]byte(c.Hash(plain)), []byte(hash)) != 1 {
		return nil, ErrDocumentIntegrity
	}
	return plain, nil
}
