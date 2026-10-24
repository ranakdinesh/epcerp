package migrations

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSQLCConsumesTheSameOrderedMigrationHistory(t *testing.T) {
	root := filepath.Join("..", "..")
	config, err := os.ReadFile(filepath.Join(root, "sqlc.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			expected = append(expected, entry.Name())
		}
	}
	actual := []string{}
	for line := range strings.SplitSeq(string(config), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") || !strings.HasSuffix(line, ".up.sql") {
			continue
		}
		path := strings.TrimPrefix(line, "- ")
		name := filepath.Base(path)
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		embedded, err := Files.ReadFile(name)
		if err != nil || !bytes.Equal(body, embedded) {
			t.Fatalf("sqlc and migrator disagree on %s", name)
		}
		actual = append(actual, name)
	}
	if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
		t.Fatal("sqlc schema manifest must include every up migration in global version order")
	}
}

func TestHistoryRejectsDuplicateAssets(t *testing.T) {
	first := fstest.MapFS{"000001_one.up.sql": &fstest.MapFile{Data: []byte("SELECT 1;")}}
	second := fstest.MapFS{"000001_one.up.sql": &fstest.MapFile{Data: []byte("SELECT 2;")}}
	history := historyFS{sources: []fs.FS{first, second}}
	if _, err := history.ReadDir("."); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("duplicate provider must fail closed: %v", err)
	}
}

func TestCombinedHistoryBehavesAsAReadOnlyFilesystem(t *testing.T) {
	entries, err := Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	if err := fstest.TestFS(Files, names...); err != nil {
		t.Fatal(err)
	}
	if _, err := Files.ReadFile("../000001_auth.up.sql"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("history path traversal accepted")
	}
}
