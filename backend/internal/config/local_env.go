package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const localEnvironmentFile = ".env.local"

type environmentEntry struct {
	value  string
	expand bool
}

// LoadLocalEnvironment loads ignored workstation configuration when present.
// Existing process variables always win, so container and production settings
// remain authoritative.
func LoadLocalEnvironment() error {
	return loadLocalEnvironment(localEnvironmentFile, os.LookupEnv, os.Setenv)
}

func loadLocalEnvironment(path string, lookup func(string) (string, bool), set func(string, string) error) error {
	local, err := readEnvironmentFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	merged := make(map[string]environmentEntry)
	if reference, ok := local["EPCERP_SECRETS_FILE"]; ok && strings.TrimSpace(reference.value) != "" {
		secretPath := expandEnvironment(reference.value, lookup)
		if !filepath.IsAbs(secretPath) {
			secretPath = filepath.Join(filepath.Dir(path), secretPath)
		}
		shared, readErr := readEnvironmentFile(filepath.Clean(secretPath))
		if readErr != nil {
			return fmt.Errorf("load local secrets file: %w", readErr)
		}
		for key, entry := range shared {
			merged[key] = entry
		}
	}
	for key, entry := range local {
		if key != "EPCERP_SECRETS_FILE" {
			merged[key] = entry
		}
	}

	resolved := make(map[string]string, len(merged))
	resolving := make(map[string]bool, len(merged))
	var resolve func(string) (string, error)
	resolve = func(key string) (string, error) {
		if value, exists := lookup(key); exists {
			return value, nil
		}
		if value, exists := resolved[key]; exists {
			return value, nil
		}
		entry, exists := merged[key]
		if !exists {
			return "", nil
		}
		if resolving[key] {
			return "", fmt.Errorf("cyclic local environment reference for %s", key)
		}
		resolving[key] = true
		value := entry.value
		var expansionErr error
		if entry.expand {
			value = os.Expand(value, func(reference string) string {
				if expansionErr != nil {
					return ""
				}
				var resolvedValue string
				resolvedValue, expansionErr = resolve(reference)
				return resolvedValue
			})
		}
		delete(resolving, key)
		if expansionErr != nil {
			return "", expansionErr
		}
		resolved[key] = value
		return value, nil
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, exists := lookup(key); exists {
			continue
		}
		value, resolveErr := resolve(key)
		if resolveErr != nil {
			return resolveErr
		}
		if err = set(key, value); err != nil {
			return fmt.Errorf("set local environment key %s: %w", key, err)
		}
	}
	return nil
}

func readEnvironmentFile(path string) (map[string]environmentEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := make(map[string]environmentEntry)
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !found || !validEnvironmentKey(key) {
			return nil, fmt.Errorf("invalid local environment entry at line %d", lineNumber)
		}
		expand := true
		if len(value) >= 2 && value[0] == value[len(value)-1] && (value[0] == '\'' || value[0] == '"') {
			expand = value[0] != '\''
			value = value[1 : len(value)-1]
		}
		entries[key] = environmentEntry{value: value, expand: expand}
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("read local environment: %w", err)
	}
	return entries, nil
}

func validEnvironmentKey(key string) bool {
	if key == "" || !(key[0] == '_' || key[0] >= 'A' && key[0] <= 'Z' || key[0] >= 'a' && key[0] <= 'z') {
		return false
	}
	for index := 1; index < len(key); index++ {
		character := key[index]
		if !(character == '_' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}

func expandEnvironment(value string, lookup func(string) (string, bool)) string {
	return os.Expand(value, func(key string) string {
		resolved, _ := lookup(key)
		return resolved
	})
}
