// Package config validates the environment without exposing secret values in errors.
package config

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TenderManagerBundleIDs []string
	HTTPAddress            string
	Environment            string
	LogLevel               string
	DatabasePoolSize       int
	DatabaseMaxOverflow    int
	AllowedHosts           []string
	CORSOrigins            []string
	ForceHTTPS             bool
	MaxRequestBodyBytes    int64
	RateLimitRequests      int
	RateLimitWindow        time.Duration
	databaseURL            string
}

func (s Config) DatabaseURL() string { return s.databaseURL }
func (s Config) String() string      { return "Config{configuration values omitted}" }
func (s Config) GoString() string    { return s.String() }

type Lookup func(string) (string, bool)

// Load reads epcERP's independent database URL and retained VTPL_ platform settings.
// Environment files from the Python project are never loaded.
func Load(lookup Lookup) (Config, error) {
	s := Config{HTTPAddress: "127.0.0.1:8001"}
	if raw, ok := lookup("EPCERP_TENDER_MANAGER_BUNDLE_IDS"); ok && strings.TrimSpace(raw) != "" {
		seen := map[string]bool{}
		for _, id := range strings.Split(raw, ",") {
			id = strings.ToLower(strings.TrimSpace(id))
			if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(id) || seen[id] || len(s.TenderManagerBundleIDs) >= 20 {
				return s, errors.New("invalid EPCERP_TENDER_MANAGER_BUNDLE_IDS")
			}
			seen[id] = true
			s.TenderManagerBundleIDs = append(s.TenderManagerBundleIDs, id)
		}
	}
	if address, exists := lookup("EPCERP_HTTP_ADDRESS"); exists {
		host, port, err := net.SplitHostPort(address)
		number, numberError := strconv.Atoi(port)
		if err != nil || net.ParseIP(host) == nil || numberError != nil || number < 1 || number > 65535 {
			return s, errors.New("invalid EPCERP_HTTP_ADDRESS")
		}
		s.HTTPAddress = address
	}
	value := func(key, fallback string) string {
		v, ok := lookup("VTPL_" + key)
		if !ok {
			return fallback
		}
		return v
	}
	s.Environment = value("ENVIRONMENT", "development")
	switch s.Environment {
	case "development", "test", "staging", "production":
	default:
		return s, invalid("ENVIRONMENT")
	}
	s.LogLevel = strings.ToLower(strings.TrimSpace(value("LOG_LEVEL", "info")))
	switch s.LogLevel {
	case "trace", "debug", "info", "warn", "error":
	default:
		return s, invalid("LOG_LEVEL")
	}
	dsn, _ := lookup("EPCERP_DATABASE_URL")
	u, err := url.Parse(dsn)
	if err != nil || u == nil || (u.Scheme != "postgresql" && u.Scheme != "postgres") || u.Hostname() == "" || u.Fragment != "" {
		return s, invalid("DATABASE_URL")
	}
	if u.Port() != "" {
		p, err := strconv.Atoi(u.Port())
		if err != nil || p < 1 || p > 65535 {
			return s, invalid("DATABASE_URL")
		}
	}
	s.databaseURL = dsn
	for _, field := range []struct {
		key                string
		target             *int
		fallback, min, max int
	}{
		{"DATABASE_POOL_SIZE", &s.DatabasePoolSize, 5, 1, 50},
		{"DATABASE_MAX_OVERFLOW", &s.DatabaseMaxOverflow, 5, 0, 50},
		{"RATE_LIMIT_REQUESTS", &s.RateLimitRequests, 120, 1, 10000},
	} {
		n, err := integer(value(field.key, strconv.Itoa(field.fallback)), field.min, field.max)
		if err != nil {
			return s, invalid(field.key)
		}
		*field.target = n
	}
	maximum, err := integer(value("MAX_REQUEST_BODY_BYTES", "1048576"), 1024, 52428800)
	if err != nil {
		return s, invalid("MAX_REQUEST_BODY_BYTES")
	}
	s.MaxRequestBodyBytes = int64(maximum)
	window, err := integer(value("RATE_LIMIT_WINDOW_SECONDS", "60"), 1, 3600)
	if err != nil {
		return s, invalid("RATE_LIMIT_WINDOW_SECONDS")
	}
	s.RateLimitWindow = time.Duration(window) * time.Second
	switch strings.ToLower(strings.TrimSpace(value("FORCE_HTTPS", "false"))) {
	case "true", "1", "yes", "on":
		s.ForceHTTPS = true
	case "false", "0", "no", "off":
	default:
		return s, invalid("FORCE_HTTPS")
	}
	s.AllowedHosts = csv(value("ALLOWED_HOSTS", "localhost,127.0.0.1,testserver"))
	if len(s.AllowedHosts) == 0 {
		return s, invalid("ALLOWED_HOSTS")
	}
	for i, host := range s.AllowedHosts {
		host = strings.ToLower(host)
		check := strings.TrimPrefix(host, "*.")
		if !validHost(check) || host == "*" {
			return s, invalid("ALLOWED_HOSTS")
		}
		if strings.HasPrefix(host, "*.") && net.ParseIP(check) != nil {
			return s, invalid("ALLOWED_HOSTS")
		}
		s.AllowedHosts[i] = host
	}
	s.CORSOrigins = csv(value("CORS_ORIGINS", "http://localhost:3000"))
	for i, origin := range s.CORSOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || !validHost(parsed.Hostname()) || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return s, invalid("CORS_ORIGINS")
		}
		if parsed.Port() != "" {
			p, err := strconv.Atoi(parsed.Port())
			if err != nil || p < 1 || p > 65535 {
				return s, invalid("CORS_ORIGINS")
			}
		}
		s.CORSOrigins[i] = strings.TrimSuffix(origin, "/")
	}
	return s, nil
}

func invalid(key string) error { return errors.New("invalid VTPL_" + key) }
func integer(value string, min, max int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < min || n > max {
		return 0, errors.New("out of range")
	}
	return n, nil
}
func csv(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}
