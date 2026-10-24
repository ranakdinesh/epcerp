# Database SQL

`migrations/` coordinates the Go application's single ordered PostgreSQL history.
Auth-owned SQL lives at `internal/modules/auth/sql/migrations/`; Tender-owned SQL
lives at `internal/modules/tender_fms/sql/migrations/`. Other versions remain here.
All three providers feed the same golang-migrate runner and version table.
Each version has an `.up.sql` and `.down.sql` file.
Keep applied versions and their contents intact; add a new version for changes.
Small rename/constraint migrations are required history, not duplicate schema.

The migrations directory is a standard-library-only Go package within the backend's
single module. It embeds its SQL in the migration binary. There is no separate
migrations `go.mod`, local replacement or external SQL dependency. The compiled
migration command needs no SQL directory mounted at runtime.

From `backend/`, use the existing `make migrate-up`, `make migrate-version` and
explicit bounded rollback commands. Do not apply migrations to a different
project's database. Auth sqlc queries are at `internal/modules/auth/sql/queries/`;
Tender queries are at `internal/modules/tender_fms/sql/queries/`;
other queries remain module-owned in their existing directories. `sqlc.yaml`
contains one ordered up-migration manifest, shared by all generation blocks through
a YAML anchor. Add new versions to that manifest; a test rejects omissions, wrong
ordering or a mismatch between compiler SQL and the embedded migration bytes.

Migration 89 moves 19 Auth tables and three Auth functions from public to auth
without recreating records. Prior migration contents remain unchanged, including
the historical platform ownership marker in migration 1. The marker and global
migration version table remain in public. Never use search_path to hide missing
Auth qualification in runtime queries. See `docs/GO_AUTH_SCHEMA_ISOLATION.md` at
the repository root for upgrade, privileges, rollback and verification details.

Migration 90 relocates twelve existing Tender tables to schema `tender`. Fully
Tender-owned histories 82, 85 and 87 move to the module unchanged; mixed shared
histories 21, 86 and 88 stay here. See `internal/modules/tender_fms/README.md` from
the backend root for the ownership map and shared-service boundary.

Docker backend/worker builds use `backend/` as their build context, including this
SQL package. No operational database changes are performed by moving these files.

Verification: from `backend/`, `go test ./sql/migrations` validates the embedded
ordered up/down pairs. `make check` checks sqlc and Go code; `make verify`
includes fresh/rollback/reapply lifecycle coverage on harness-owned PostgreSQL.
