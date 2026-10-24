package schema

import (
	"github.com/ranakdinesh/epcERP/sql/migrations"
	"strconv"
	"strings"
	"testing"
)

func TestLatestVersionMatchesEmbeddedOwnedHistory(t *testing.T) {
	files, e := migrations.Files.ReadDir(".")
	if e != nil {
		t.Fatal(e)
	}
	latest := 0
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") {
			continue
		}
		version, e := strconv.Atoi(strings.Split(file.Name(), "_")[0])
		if e != nil {
			t.Fatal(e)
		}
		if version > latest {
			latest = version
		}
	}
	if LatestVersion != latest {
		t.Fatalf("Update owned migration metadata: latest %d, declared %d", latest, LatestVersion)
	}
}
