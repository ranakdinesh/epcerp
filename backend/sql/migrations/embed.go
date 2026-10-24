// Package migrations owns epcERP's independently deployable PostgreSQL history.
package migrations

import (
	"embed"
	"io/fs"

	authmigrations "github.com/ranakdinesh/epcERP/internal/modules/auth/sql/migrations"
)

// Files is embedded in the migration command; no Python checkout or working-directory
// specific SQL path is required when deploying the binary.
//
//go:embed *.sql
var platformFiles embed.FS

// Files is one globally ordered history, including module-owned Auth migrations.
var Files = historyFS{sources: []fs.FS{platformFiles, authmigrations.Files}}
