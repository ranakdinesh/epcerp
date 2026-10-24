package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func load(values map[string]string) (Config, error) {
	return Load(func(key string) (string, bool) {
		if key == "EPCERP_DATABASE_URL" {
			if value, ok := values[key]; ok {
				return value, true
			}
			return "postgresql://localhost/epcerp", true
		}
		value, ok := values[key]
		return value, ok
	})
}
func TestDefaultsAndValidatedOverrides(t *testing.T) {
	s, err := load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Environment != "development" || s.LogLevel != "info" || s.DatabasePoolSize != 5 || s.DatabaseMaxOverflow != 5 || s.MaxRequestBodyBytes != 1048576 || s.RateLimitRequests != 120 || s.RateLimitWindow != time.Minute {
		t.Fatal("default drift")
	}
	if s.DatabaseURL() != "postgresql://localhost/epcerp" || len(s.AllowedHosts) != 3 || s.CORSOrigins[0] != "http://localhost:3000" {
		t.Fatal("compatibility drift")
	}
	s, err = load(map[string]string{"VTPL_ALLOWED_HOSTS": "api, *.example.com, ::1", "VTPL_CORS_ORIGINS": "https://example.com/", "VTPL_FORCE_HTTPS": "true", "VTPL_DATABASE_MAX_OVERFLOW": "0"})
	if err != nil || !s.ForceHTTPS || s.DatabaseMaxOverflow != 0 || s.CORSOrigins[0] != "https://example.com" {
		t.Fatalf("override rejected: %v", err)
	}
}
func TestInvalidConfigAreRejectedWithoutValues(t *testing.T) {
	for key, values := range map[string][]string{
		"ENVIRONMENT": {"unsafe"}, "LOG_LEVEL": {"fatal", ""}, "DATABASE_URL": {"", "postgresql://", "https://localhost/db", "postgresql://localhost:0/db"},
		"DATABASE_POOL_SIZE": {"0", "51", "invalid"}, "DATABASE_MAX_OVERFLOW": {"-1", "51"},
		"MAX_REQUEST_BODY_BYTES": {"1023", "52428801"}, "RATE_LIMIT_REQUESTS": {"0", "10001"}, "RATE_LIMIT_WINDOW_SECONDS": {"0", "3601"},
		"FORCE_HTTPS": {"unknown"}, "ALLOWED_HOSTS": {"", "*", "https://example.com", "example.com:8000", "bad*host", "*.127.0.0.1"},
		"CORS_ORIGINS": {"*", "https://name@example.com", "https://example.com/path", "https://example.com?secret=private", "https://example.com:0"},
	} {
		for _, value := range values {
			envKey := "VTPL_" + key
			if key == "DATABASE_URL" {
				envKey = "EPCERP_DATABASE_URL"
			}
			_, err := load(map[string]string{envKey: value})
			if err == nil || err.Error() != "invalid VTPL_"+key {
				t.Fatalf("%s: unexpected validation %v", key, err)
			}
		}
	}
	s, err := load(map[string]string{"EPCERP_DATABASE_URL": "postgresql://localhost/db?application_name=private-canary"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprintf("%v %#v %+v", s, s, s), "private-canary") {
		t.Fatal("settings formatting exposed sensitive values")
	}
}

func TestTenderManagerBundleConfigurationFailsClosed(t *testing.T) {
	id := "18e66d87-c4db-4771-bbdb-03ad3684e345"
	s, e := load(nil)
	if e != nil || len(s.TenderManagerBundleIDs) != 0 {
		t.Fatal("implicit manager designation", e)
	}
	s, e = load(map[string]string{"EPCERP_TENDER_MANAGER_BUNDLE_IDS": "  " + strings.ToUpper(id) + "  "})
	if e != nil || len(s.TenderManagerBundleIDs) != 1 || s.TenderManagerBundleIDs[0] != id {
		t.Fatal("stable identity config", e)
	}
	for _, value := range []string{"Tender Manager", id + "," + id, id + ",", strings.Repeat(id+",", 21)} {
		_, e = load(map[string]string{"EPCERP_TENDER_MANAGER_BUNDLE_IDS": value})
		if e == nil || e.Error() != "invalid EPCERP_TENDER_MANAGER_BUNDLE_IDS" {
			t.Fatal("unsafe designation config", e)
		}
	}
}
