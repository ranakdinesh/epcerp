# epcERP

Independent Go backend and Next.js frontend for EPC operations.

- `backend/`: Go API, workers, owned PostgreSQL migrations and native tests.
- `frontend/`: Next.js application connected to the Go API.
- `backend/sql/migrations/`: ordered Go-owned PostgreSQL migrations embedded in the migrator.
- `deployment/epcerp/`: Go Docker stack and operator tooling.

See each project's README for setup and verification. Local `.env` files,
credentials, database snapshots, build outputs and dependencies stay outside Git.

For local native backend development, `backend/.env.local` selects the host
PostgreSQL database at `127.0.0.1:5432/vtplerp`. Keep the password private.
The Docker stack supplies its own database URL; loopback inside a container
refers to that container, not the host.
