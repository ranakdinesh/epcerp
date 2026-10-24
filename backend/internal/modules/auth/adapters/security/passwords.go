package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Passwords accepts bounded Argon2id/i PHC parameters and creates standard
// recommended Argon2id hashes. The semaphore bounds memory pressure across requests.
type Passwords struct {
	slots chan struct{}
	dummy string
}

func NewPasswords() (*Passwords, error) {
	p := &Passwords{slots: make(chan struct{}, 4)}
	dummy, err := p.Hash(context.Background(), rand.Text())
	if err != nil {
		return nil, err
	}
	p.dummy = dummy
	return p, nil
}

func (p *Passwords) acquire(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case p.slots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *Passwords) Hash(ctx context.Context, password string) (string, error) {
	if !p.acquire(ctx) {
		return "", ctx.Err()
	}
	defer func() { <-p.slots }()
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	key := argon2.IDKey([]byte(password), salt, 3, 65536, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=4$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

var parameterPattern = regexp.MustCompile(`^m=([0-9]+),t=([0-9]+),p=([0-9]+)$`)

func (p *Passwords) Verify(ctx context.Context, password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(encoded) > 255 || len(parts) != 6 || parts[0] != "" || (parts[1] != "argon2id" && parts[1] != "argon2i") || parts[2] != "v=19" {
		return false
	}
	match := parameterPattern.FindStringSubmatch(parts[3])
	if match == nil {
		return false
	}
	m, err := strconv.ParseUint(match[1], 10, 32)
	if err != nil {
		return false
	}
	iterations, err := strconv.ParseUint(match[2], 10, 32)
	if err != nil {
		return false
	}
	parallel, err := strconv.ParseUint(match[3], 10, 8)
	if err != nil || parallel < 1 || parallel > 16 || m < 8*parallel || m > 262144 || iterations < 1 || iterations > 10 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false
	}
	if !p.acquire(ctx) {
		return false
	}
	defer func() { <-p.slots }()
	derive := argon2.IDKey
	if parts[1] == "argon2i" {
		derive = argon2.Key
	}
	actual := derive([]byte(password), salt, uint32(iterations), uint32(m), uint8(parallel), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (p *Passwords) VerifyAbsent(ctx context.Context, password string) {
	p.Verify(ctx, password, p.dummy)
}
