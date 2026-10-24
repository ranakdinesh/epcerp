package infrastructure

import (
	"github.com/ranakdinesh/epcERP/internal/config"
	"testing"
	"time"
)

func TestPoolSettingsCompatibility(t *testing.T) {
	settings, err := config.Load(func(key string) (string, bool) {
		if key == "EPCERP_DATABASE_URL" {
			return "postgresql://localhost/test", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := poolConfig(settings)
	if err != nil || parsed.MinConns != 5 || parsed.MaxConns != 10 || parsed.MaxConnLifetime != 30*time.Minute || parsed.ConnConfig.ConnectTimeout != 5*time.Second {
		t.Fatal("pool configuration mismatch")
	}
}
