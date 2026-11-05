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

Local Compose uses `EPCERP_AUTH_OWNER=nextjs` on both application services. Existing
passwords and the PostgreSQL schema are preserved. See
[`frontend/src/server/auth/README.md`](frontend/src/server/auth/README.md) for the
migration boundary, compatibility, verification and explicit Go ownership rollback.
Clients uses `EPCERP_CLIENTS_OWNER=nextjs` on both services. See the
[Master Data guide](frontend/src/server/master/README.md) for its shared structure
and the areas that remain Go-owned.

For local native backend development, `backend/.env.local` selects the host
PostgreSQL database at `127.0.0.1:5432/vtplerp`. Keep the password private.
The Docker stack supplies its own database URL; loopback inside a container
refers to that container, not the host.
