# epcERP Docker deployment

Run `./setup.sh` from this directory for the local preview. It preserves existing
`.env` credentials and named volumes, builds native services, applies the owned
Node migration CLI, and starts the app at http://localhost:9092.

Current services: `frontend`, private `document-processor`, one-shot `schema`,
PostgreSQL `db`, MinIO `storage`, and `clamav`. The application has no Go services
or local-model/reranker service. The storage Dockerfile builds pinned upstream
MinIO vendor code; it contains no ERP business logic.

Use `sh ./compose.sh config --quiet` to validate configuration without printing
credentials. Use this wrapper for subsequent operations too, for example
`sh ./compose.sh up -d --no-build --no-deps --wait frontend`. It always loads
`.env`, then optional `.env.gemini` and `.env.providers`, so recreating services
retains the existing provider keys, approval and budgets. A bare Compose command
that omits those files can silently disable an already configured provider.
New cloud providers remain disabled until explicitly configured and approved.

Before any schema upgrade, stop application writers and take a verified private
database/object-storage backup. Build frontend, Node migrator and processor from
the same combined `codex/development` checkout. Never use volume deletion, schema
reset or migration force to switch branches. Local checks do not authorize a
production deployment, real messages or paid provider calls.

The deployed development server has its own HTTPS gateway and office-LAN frontend;
preserve that server's private configuration and routes when preparing releases.
Do not replace its Compose file with this local-preview configuration.

Historical migration and deployment evidence, including the verified 5 November
server cutover, is in [history](history/). Those records describe past releases,
not current setup instructions. The retired Go reference is available through
Git history rather than the active working tree.
