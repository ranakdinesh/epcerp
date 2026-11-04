# epcERP

Independent Go business backend and Next.js application for EPC operations.

- `backend/`: Go API, workers, owned PostgreSQL migrations and native tests.
- `frontend/`: Next.js application with native authentication/authorization,
  Clients within the shared Master Data module, and connections to remaining Go APIs.
- `backend/sql/migrations/`: ordered Go-owned PostgreSQL migrations embedded in the migrator.
- `deployment/epcerp/`: Go Docker stack and operator tooling.

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
