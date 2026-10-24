package security

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type AuthConfig struct {
	Tokens                                               TokenConfig
	Policy                                               domain.Policy
	BootstrapUsername, BootstrapEmail, BootstrapPassword string
}

func (c AuthConfig) String() string   { return "AuthConfig{values omitted}" }
func (c AuthConfig) GoString() string { return c.String() }
func LoadAuthConfig(lookup func(string) (string, bool)) (AuthConfig, error) {
	config := AuthConfig{}
	value := func(key, fallback string) string {
		v, exists := lookup("VTPL_" + key)
		if !exists {
			return fallback
		}
		return v
	}
	invalid := func(key string) error { return errors.New("invalid VTPL_" + key) }
	config.Tokens.Secret = value("JWT_SECRET_KEY", "")
	if utf8.RuneCountInString(config.Tokens.Secret) < 32 {
		return config, invalid("JWT_SECRET_KEY")
	}
	config.Tokens.Issuer = value("JWT_ISSUER", "epcERP")
	config.Tokens.Audience = value("JWT_AUDIENCE", "epcERP-api")
	for _, entry := range []struct{ key, value string }{{"JWT_ISSUER", config.Tokens.Issuer}, {"JWT_AUDIENCE", config.Tokens.Audience}} {
		if len(entry.value) < 3 || len(entry.value) > 255 {
			return config, invalid(entry.key)
		}
	}
	for _, entry := range []struct {
		key                string
		fallback, min, max int
		scale              time.Duration
		target             *time.Duration
	}{
		{"ACCESS_TOKEN_MINUTES", 15, 1, 60, time.Minute, &config.Tokens.AccessLifetime},
		{"SESSION_HOURS", 12, 1, 12, time.Hour, &config.Policy.SessionLifetime},
		{"USER_PRESENCE_TIMEOUT_SECONDS", 300, 30, 3600, time.Second, &config.Policy.PresenceTimeout},
	} {
		n, err := strconv.Atoi(value(entry.key, strconv.Itoa(entry.fallback)))
		if err != nil || n < entry.min || n > entry.max {
			return config, invalid(entry.key)
		}
		*entry.target = time.Duration(n) * entry.scale
	}
	config.BootstrapUsername = value("BOOTSTRAP_SUPERADMIN_USERNAME", "superadmin")
	config.BootstrapEmail = strings.ToLower(value("BOOTSTRAP_SUPERADMIN_EMAIL", ""))
	config.BootstrapPassword = value("BOOTSTRAP_SUPERADMIN_PASSWORD", "")
	if n := utf8.RuneCountInString(config.BootstrapUsername); n < 3 || n > 64 {
		return config, invalid("BOOTSTRAP_SUPERADMIN_USERNAME")
	}
	if utf8.RuneCountInString(config.BootstrapEmail) < 5 {
		return config, invalid("BOOTSTRAP_SUPERADMIN_EMAIL")
	}
	if utf8.RuneCountInString(config.BootstrapPassword) < 10 {
		return config, invalid("BOOTSTRAP_SUPERADMIN_PASSWORD")
	}
	return config, nil
}
