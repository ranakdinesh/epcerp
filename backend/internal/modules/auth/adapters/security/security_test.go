package security

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func tokenFixture(t *testing.T) *Tokens {
	t.Helper()
	tokens, err := NewTokens(TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "epcERP", Audience: "epcERP-api", AccessLifetime: 15 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return tokens
}
func TestJWTRequiredClaimsAlgorithmAndSessionCeiling(t *testing.T) {
	tokens := tokenFixture(t)
	now := time.Now().Truncate(time.Second)
	tokens.clock = func() time.Time { return now }
	subject := tokens.ID()
	raw, lifetime, err := tokens.Issue(subject, now.Unix()+20, 0)
	if err != nil || lifetime != 20 {
		t.Fatal("session ceiling not applied")
	}
	if got, err := tokens.Subject(raw); err != nil || got != subject {
		t.Fatal("valid identity token rejected")
	}
	for _, invalid := range []string{"missing-exp", "missing-iat", "missing-nbf", "missing-jti", "null-nbf", "wrong-type", "wrong-aud", "wrong-issuer", "future", "expired", "hs512"} {
		claims := jwt.MapClaims{"sub": subject, "iss": "epcERP", "aud": "epcERP-api", "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Unix() + 60, "jti": tokens.ID(), "typ": "access"}
		switch invalid {
		case "missing-exp":
			delete(claims, "exp")
		case "missing-iat":
			delete(claims, "iat")
		case "missing-nbf":
			delete(claims, "nbf")
		case "missing-jti":
			delete(claims, "jti")
		case "null-nbf":
			claims["nbf"] = nil
		case "wrong-type":
			claims["typ"] = "refresh"
		case "wrong-aud":
			claims["aud"] = "other"
		case "wrong-issuer":
			claims["iss"] = "other"
		case "future":
			claims["nbf"] = now.Unix() + 30
		case "expired":
			claims["exp"] = now.Unix() - 1
		}
		var method jwt.SigningMethod = jwt.SigningMethodHS256
		if invalid == "hs512" {
			method = jwt.SigningMethodHS512
		}
		bad, _ := jwt.NewWithClaims(method, claims).SignedString([]byte(tokens.config.Secret))
		if _, err := tokens.Subject(bad); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
	rawRefresh, hash := tokens.Refresh()
	if len(rawRefresh) != 64 || len(hash) != 64 || tokens.HashRefresh(rawRefresh) != hash || strings.Contains(hash, rawRefresh) {
		t.Fatal("opaque refresh format drift")
	}
}
func TestPHCCompatibilityAndMalformedBounds(t *testing.T) {
	passwords, err := NewPasswords()
	if err != nil {
		t.Fatal(err)
	}
	secret := rand.Text()
	encoded, err := passwords.Hash(context.Background(), secret)
	if err != nil || !passwords.Verify(context.Background(), secret, encoded) || passwords.Verify(context.Background(), secret+"changed", encoded) {
		t.Fatal("password hash roundtrip failed")
	}
	for _, bad := range []string{"not-a-hash", strings.Replace(encoded, "m=65536", "m=4294967295", 1), strings.Replace(encoded, "p=4", "p=0", 1), strings.Replace(encoded, "t=3", "t=999", 1), strings.Replace(encoded, "v=19", "v=16", 1)} {
		if passwords.Verify(context.Background(), secret, bad) {
			t.Fatal("malformed PHC accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Saturate slots so canceled work cannot queue indefinitely.
	for range cap(passwords.slots) {
		passwords.slots <- struct{}{}
	}
	if _, err := passwords.Hash(ctx, secret); err == nil {
		t.Fatal("canceled hashing queued")
	}
	for range cap(passwords.slots) {
		<-passwords.slots
	}
}
