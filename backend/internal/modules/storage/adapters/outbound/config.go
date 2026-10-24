package outbound

import (
	"crypto/sha256"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Config struct {
	Endpoint, AccessKey, SecretKey, Bucket string
	Secure                                 bool
	key                                    [32]byte
}

func (c Config) String() string   { return "StorageConfig{values omitted}" }
func (c Config) GoString() string { return c.String() }
func LoadConfig(lookup func(string) (string, bool)) (Config, error) {
	value := func(key, fallback string) string {
		v, ok := lookup("VTPL_" + key)
		if !ok {
			return fallback
		}
		return v
	}
	invalid := func(key string) error { return errors.New("invalid VTPL_" + key) }
	c := Config{Endpoint: value("STORAGE_ENDPOINT", "localhost:9000"), AccessKey: value("STORAGE_ACCESS_KEY", "vtpl_storage_admin"), SecretKey: value("STORAGE_SECRET_KEY", ""), Bucket: value("STORAGE_BUCKET", "vtpl-documents")}
	uri, err := url.Parse("http://" + c.Endpoint)
	if err != nil || uri.Hostname() == "" || uri.User != nil || uri.Path != "" || uri.RawQuery != "" || uri.Fragment != "" || strings.Contains(c.Endpoint, "://") {
		return c, invalid("STORAGE_ENDPOINT")
	}
	if uri.Port() != "" {
		port, err := strconv.Atoi(uri.Port())
		if err != nil || port < 1 || port > 65535 {
			return c, invalid("STORAGE_ENDPOINT")
		}
	}
	if c.AccessKey == "" {
		return c, invalid("STORAGE_ACCESS_KEY")
	}
	if c.SecretKey == "" {
		return c, invalid("STORAGE_SECRET_KEY")
	}
	if !bucketPattern.MatchString(c.Bucket) || strings.Contains(c.Bucket, "..") {
		return c, invalid("STORAGE_BUCKET")
	}
	switch strings.ToLower(value("STORAGE_SECURE", "false")) {
	case "true", "1", "yes", "on":
		c.Secure = true
	case "false", "0", "no", "off":
	default:
		return c, invalid("STORAGE_SECURE")
	}
	raw, present := lookup("VTPL_DOCUMENT_ENCRYPTION_KEY")
	if !present || raw == "" {
		secret := value("JWT_SECRET_KEY", "")
		if utf8.RuneCountInString(secret) < 32 {
			return c, invalid("JWT_SECRET_KEY")
		}
		raw = secret + ":vtpl_secure_doc_salt"
	}
	c.key = sha256.Sum256([]byte(raw))
	return c, nil
}

var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
