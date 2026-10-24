package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLocalEnvironmentCombinesSecretsAndOverrides(t *testing.T) {
	directory := t.TempDir()
	secrets := filepath.Join(directory, "secrets.env")
	local := filepath.Join(directory, ".env.local")
	if err := os.WriteFile(secrets, []byte("PRIVATE_VALUE=secret\nEXISTING_VALUE=shared\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("EPCERP_SECRETS_FILE=secrets.env\nEXPANDED_VALUE=$PRIVATE_VALUE\nEXISTING_VALUE=local\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	values := map[string]string{"EXISTING_VALUE": "process"}
	lookup := func(key string) (string, bool) {
		value, exists := values[key]
		return value, exists
	}
	set := func(key, value string) error {
		values[key] = value
		return nil
	}
	if err := loadLocalEnvironment(local, lookup, set); err != nil {
		t.Fatal(err)
	}
	if values["PRIVATE_VALUE"] != "secret" || values["EXPANDED_VALUE"] != "secret" {
		t.Fatal("secret reference was not expanded")
	}
	if values["EXISTING_VALUE"] != "process" {
		t.Fatal("local file replaced an existing process variable")
	}
}

func TestLoadLocalEnvironmentAllowsMissingFile(t *testing.T) {
	if err := loadLocalEnvironment(filepath.Join(t.TempDir(), "missing"), func(string) (string, bool) { return "", false }, func(string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
}
