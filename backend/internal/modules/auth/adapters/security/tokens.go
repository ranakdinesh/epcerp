package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/ranakdinesh/epcERP/helpers"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

// TokenConfig holds Auth-specific signing settings and omits secrets when formatted.
type TokenConfig struct {
	Secret, Issuer, Audience string
	AccessLifetime           time.Duration
}

func (c TokenConfig) String() string { return "TokenConfig{values omitted}" }

func (c TokenConfig) GoString() string { return c.String() }

type Tokens struct {
	config TokenConfig
	clock  func() time.Time
}

func NewTokens(config TokenConfig) (*Tokens, error) {
	if len(config.Secret) < 32 || len(config.Issuer) < 3 || len(config.Issuer) > 255 || len(config.Audience) < 3 || len(config.Audience) > 255 || config.AccessLifetime < time.Minute || config.AccessLifetime > time.Hour {
		return nil, errors.New("invalid token configuration")
	}
	return &Tokens{config: config, clock: time.Now}, nil
}

func (t *Tokens) Now() int64 { return t.clock().Unix() }

func (t *Tokens) ID() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	value[6] = (value[6] & 15) | 64
	value[8] = (value[8] & 63) | 128
	s := hex.EncodeToString(value[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func (t *Tokens) Issue(userID string, sessionExpiry, version int64) (string, int, error) {
	now := t.Now()
	expiry := now + int64(t.config.AccessLifetime/time.Second)
	if sessionExpiry < expiry {
		expiry = sessionExpiry
	}
	if expiry <= now {
		return "", 0, domain.ErrUnauthorized
	}
	claims := jwt.MapClaims{"sub": userID, "iss": t.config.Issuer, "aud": t.config.Audience, "iat": now, "nbf": now, "exp": expiry, "jti": t.ID(), "typ": "access", "cv": version}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(t.config.Secret))
	return token, int(expiry - now), err
}

func (t *Tokens) verified(raw string) (jwt.MapClaims, error) {
	if len(raw) > 8192 {
		return nil, domain.ErrUnauthorized
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrUnauthorized
		}
		return []byte(t.config.Secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(t.config.Issuer), jwt.WithAudience(t.config.Audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(t.clock))
	if err != nil || !token.Valid || claims["typ"] != "access" {
		return nil, domain.ErrUnauthorized
	}
	for _, key := range []string{"sub", "exp", "iat", "nbf", "jti", "typ"} {
		if _, exists := claims[key]; !exists {
			return nil, domain.ErrUnauthorized
		}
	}
	for _, key := range []string{"exp", "iat", "nbf"} {
		if _, numeric := claims[key].(float64); !numeric {
			return nil, domain.ErrUnauthorized
		}
	}
	jti, ok := claims["jti"].(string)
	if !ok || jti == "" {
		return nil, domain.ErrUnauthorized
	}
	subject, ok := claims["sub"].(string)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	_, err = helpers.CanonicalID(subject)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

func (t *Tokens) Subject(raw string) (string, error) {
	claims, e := t.verified(raw)
	if e != nil {
		return "", e
	}
	return helpers.CanonicalID(claims["sub"].(string))
}
func (t *Tokens) Version(raw string) (int64, error) {
	claims, e := t.verified(raw)
	if e != nil {
		return 0, e
	}
	if _, exists := claims["cv"]; !exists {
		return 0, nil
	} // pre-migration identities are version zero only
	value, ok := claims["cv"].(float64)
	if !ok || value < 0 || value > 9007199254740991 || float64(int64(value)) != value {
		return 0, domain.ErrUnauthorized
	}
	return int64(value), nil
}

func (t *Tokens) Refresh() (string, string) {
	value := make([]byte, 48)
	_, _ = rand.Read(value)
	raw := base64.RawURLEncoding.EncodeToString(value)
	return raw, t.HashRefresh(raw)
}

func (t *Tokens) HashRefresh(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
