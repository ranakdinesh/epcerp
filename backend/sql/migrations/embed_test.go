package migrations

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestEmbeddedHistoryHasCompleteOrderedPairs(t *testing.T) {
	entries, err := Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`^([0-9]{6})_(.+)\.(up|down)\.sql$`)
	type migration struct {
		name       string
		directions map[string]bool
	}
	versions := map[int]*migration{}
	latest := 0
	for _, entry := range entries {
		match := pattern.FindStringSubmatch(entry.Name())
		if entry.IsDir() || match == nil {
			t.Fatalf("unexpected embedded migration: %s", entry.Name())
		}
		version, err := strconv.Atoi(match[1])
		if err != nil || version < 1 {
			t.Fatalf("invalid migration version: %s", entry.Name())
		}
		if versions[version] == nil {
			versions[version] = &migration{name: match[2], directions: map[string]bool{}}
		}
		pair := versions[version]
		if pair.name != match[2] || pair.directions[match[3]] {
			t.Fatalf("duplicate version or mismatched migration pair: %s", entry.Name())
		}
		pair.directions[match[3]] = true
		body, err := Files.ReadFile(entry.Name())
		if err != nil || strings.TrimSpace(string(body)) == "" {
			t.Fatalf("missing migration contents: %s", entry.Name())
		}
		if version > latest {
			latest = version
		}
	}
	if latest == 0 {
		t.Fatal("no migrations embedded")
	}
	for version := 1; version <= latest; version++ {
		pair := versions[version]
		if pair == nil || !pair.directions["up"] || !pair.directions["down"] {
			t.Fatalf("migration %d must have both up and down files", version)
		}
	}
}
