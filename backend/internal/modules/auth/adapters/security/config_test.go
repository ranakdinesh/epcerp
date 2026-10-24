package security

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestAuthConfigurationValidationAndRedaction(t *testing.T) {
	values := map[string]string{"VTPL_JWT_SECRET_KEY": strings.Repeat("x", 32), "VTPL_BOOTSTRAP_SUPERADMIN_EMAIL": "fixture@example.invalid", "VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD": "fixture-password-canary"}
	lookup := func(key string) (string, bool) { value, ok := values[key]; return value, ok }
	config, err := LoadAuthConfig(lookup)
	if err != nil || config.Policy.SessionLifetime != 12*time.Hour || config.Tokens.AccessLifetime != 15*time.Minute || config.Tokens.Issuer != "epcERP" || config.Tokens.Audience != "epcERP-api" {
		t.Fatal("default configuration drift")
	}
	if strings.Contains(fmt.Sprintf("%v %+v %#v", config, config, config), "fixture-password-canary") {
		t.Fatal("configuration exposed secrets")
	}
	for key, value := range map[string]string{"JWT_SECRET_KEY": "short", "JWT_ISSUER": "", "JWT_AUDIENCE": "", "SESSION_HOURS": "13", "ACCESS_TOKEN_MINUTES": "61", "USER_PRESENCE_TIMEOUT_SECONDS": "29", "BOOTSTRAP_SUPERADMIN_USERNAME": "x", "BOOTSTRAP_SUPERADMIN_EMAIL": "", "BOOTSTRAP_SUPERADMIN_PASSWORD": "short"} {
		full := "VTPL_" + key
		old, exists := values[full]
		values[full] = value
		_, err := LoadAuthConfig(lookup)
		if err == nil || err.Error() != "invalid "+full {
			t.Fatalf("unexpected validation for %s", key)
		}
		if exists {
			values[full] = old
		} else {
			delete(values, full)
		}
	}
}
