# epcERP

Next.js application and Node services for EPC operations.

- `frontend/src/server/`: native application modules, authentication, authorization and background orchestration.
- `frontend/migrations/`: owned PostgreSQL migration history; explicit Node migration CLI at `frontend/tools/migrate.mjs`.
- `frontend/tools/document-processor/`: private offline document validation, conversion and extraction helpers.
- `deployment/epcerp/`: local Docker stack and operator tooling.
- `backend/`: archived Go implementation and compatibility test oracles; Go application services are disabled by default under the `legacy-go` Compose profile.

The complete local application now runs without the Go API or Go workers. See
[the runtime migration record](deployment/epcerp/GO_RETIREMENT_VERIFICATION.md)
and [native platform structure](frontend/src/server/platform/README.md).

See each project's README for setup and verification. Local `.env` files,
credentials, database snapshots, build outputs and dependencies stay outside Git.

Default Compose selects native ownership for all application modules and disables
Go fallback. Existing passwords, encryption keys and the PostgreSQL database are
preserved. Earlier module migration guides remain as historical references; the
runtime migration record above describes the current deployment.

Native services use `EPCERP_DATABASE_URL` for the owned epcERP database. The local
Docker database is exposed at `127.0.0.1:5439`; use the configured private
credentials and preserve its volumes. Never point the migration CLI at another
application's database. Loopback inside a container refers to that container.
