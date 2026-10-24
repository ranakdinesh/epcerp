# Independent local epcERP Docker stack

The object-storage image uses `storage.Dockerfile` in this directory. Its Compose
build context is `.`; the Go stack no longer needs `deployment/docker/`.

API/migrator and worker build contexts are `backend/`, which includes the embedded
`sql/migrations/` package within the backend's single Go module. The builder follows
the existing backend `go.mod` (Go 1.26.3). SQL remains embedded in the migration
binary; do not mount a separate migrations directory into running containers.

Open http://localhost:9092. This is a development preview with a fresh, persistent
PostgreSQL 17 database, Go 1.26.2 API/migrator and the independent VTPL CORE frontend.
It uses Compose project epcerp-local, its own network and postgres_data volume.
Frontend port 9092, PostgreSQL port 5439 and object-storage API port 9002 are
published on host loopback. The API has no host port. Connect to Go PostgreSQL at
127.0.0.1:5439, database/user `epcerp`, using `EPCERP_DB_PASSWORD` from the private
`.env` file.

From the repository root, with Docker and OpenSSL available:

    ./deployment/epcerp/setup.sh
    ./deployment/epcerp/credentials.sh

Setup creates random local database/JWT/recovery-admin/object-storage credentials once, in the
ignored deployment/epcerp/.env file with mode 0600, then builds and starts the stack.
Never commit or share that file. Credentials are preserved during subsequent builds.
The credentials helper prints only the recovery login when you run it locally.
Username is superadmin. Business records start empty; no Python data is copied.
Use the directory screens to create active references/clients, then Projects to
register drafts or manage users/manager permissions. Manager handover needs
projects.master.read plus projects.manager_assignments.write. Assigned candidates
must have live project read/write permissions. Activation/operational workflows and
real notifications remain unfinished. Private document storage uses this stack's
independent MinIO service and storage_data volume; its API is exposed only on host
loopback port 9002 for native backend development.

All implemented API modules load by default; endpoint permissions still govern access.
EPCERP_DATABASE_URL points only to this stack's database. The migration service runs
the embedded Go migration history before the API starts; API startup never creates
tables. The frontend uses only http://api:8001/api/v1, independent epcERP cookies and
the explicit development HTTP cookie setting. Runtime images run as non-root users.
The explicit EPCERP_HTTP_ADDRESS=0.0.0.0:8001 listener is private to the Docker
network; the normal host API default remains 127.0.0.1:8001.

Operational commands from this directory:

    docker compose --env-file .env -f compose.yaml ps
    docker compose --env-file .env -f compose.yaml logs --tail=100 api frontend
    docker compose --env-file .env -f compose.yaml stop
    docker compose --env-file .env -f compose.yaml start

## Native IDE development

Run PostgreSQL and MinIO in Docker while starting the backend and frontend from the
IDE. From the repository root:

    ./deployment/epcerp/ide-infrastructure.sh start

This stops the Docker API, frontend and document worker to prevent duplicate app
processes, then starts and waits for only `db` and `storage`. It preserves both
volumes. Use working directory `backend` for `go run ./cmd/api/`, and working
directory `frontend` for `npm run dev`. The backend connects to PostgreSQL at
127.0.0.1:5439 and MinIO at 127.0.0.1:9002; the frontend connects to the native API
at 127.0.0.1:8001.

Rebuild/start with setup.sh after source changes. A normal stop/down preserves the
database volume. Do not remove volumes or regenerate credentials when retaining
data; reset is a separate destructive action. Local HTTP preview is not a production
TLS deployment. Frontend dependency audit findings inherited from the cloned
lockfile require separate dependency remediation before production acceptance.

## Me and workspace preferences

Use the circular avatar in the app bar to open Me, change your password or sign out.
Me edits display name, profile photo and email; changing email/password requires
current-password verification. Upload PNG/JPEG up to 512KiB and 2048×2048 pixels;
photos become square 256×256 JPEGs. Remove photo restores initials. The sun/moon
button changes theme. Desktop navigation collapses to icons; mobile opens a drawer.
Theme and sidebar preference are stored locally, without authentication tokens.
Password changes update sign-in credentials and revoke refresh sessions. Seed/test
credential files retain initial values; changing a password through Me does not
rewrite those files. Test credentials were restored after this slice's verification.

Opt-in browser regression runners live in frontend/scripts:
verify-personal-workspace.cjs (profile/name/photo restored) and
verify-personal-account.cjs (Director email/password changed and restored).
Run only against this local test preview: they deliberately exercise account writes
and sign test sessions out. They require Playwright/Chromium; set
EPCERP_PLAYWRIGHT_MODULE to an available Playwright package directory if it is not
installed locally, and EPCERP_TEST_CREDENTIALS_FILE to an alternate private test
credential file if needed. Screenshots are written to the OS temporary directory.
These runners are separate from the default helper/unit tests and never print
credentials. Go database integration tests use an isolated temporary database.

Personal profile ownership is `auth/user_profile`, stored in `user_profile`.
Migration 10 reversibly renames the former table and preserves existing profiles.
Frontend/API use `/api/auth/user_profile/...` and `/api/v1/auth/user_profile/...`.
The desktop collapse control is attached to the sidebar edge; mobile keeps the
header menu button. Rebuild with setup.sh to apply the migration and matching routes.

## Retired one-time imports

The old `data/` snapshots, reconciliation manifests and five `import-*.cjs`
operator scripts are no longer part of the active deployment. They are not used
by startup or normal application requests. On 23 October 2026 they were archived
under `deployment/backups/retired-imports-20261023-214020.zip`, with all 19 files
verified against their original SHA-256 hashes before removing the active copies.
The adjacent `.sha256` file checks the archive itself. Keep this private backup
for source-to-destination mappings and historical reconciliation; it is not a
PostgreSQL or object-storage backup and must not be used as automatic seeding.

Use normal Go application workflows for ongoing changes. Any future import is a
separately reviewed operation. The private `.env.test-users` file is retained for
the opt-in browser regression scripts, and `roles/` retains reviewed role definitions.
Preserve the database, object-storage volumes and all document/image encryption
keys independently; imported records no longer depend on these archived JSON files.

### Tender FMS document intake and reusable reader

After migration/deployment, Tender FMS is served at `/dashboard?screen=tender-fms` with authenticated, authorized access. Go migration 21 owns intake, queued readings, encrypted immutable originals/results and local evidence history. The dedicated `document-worker` uses only its internal database network, no internet/provider API. Its process receives no database credentials/encryption key, and applies memory/time/output limits. Local native PDF/XLSX/DOCX readers, English/Hindi OCR and headless-only LibreOffice support PDF, XLSX/XLS and DOCX/DOC. One source upload is at most 32 MiB; text/structured results are at most 8 MiB. Low OCR confidence requires comparison with the original. Legacy XLS conversion may recalculate formula values and warns explicitly.

`setup.sh` creates `EPCERP_DOCUMENT_READING_KEY` only if missing. Preserve this private key alongside database backups: original and reading ciphertext require it across rebuilds. Do not rotate it by editing the environment. Worker processes one document at a time; queued work persists in the database, interrupted claims recover after the 20-minute lease, and failed work has at most three attempts. Stop/restart the worker through Compose; never edit lease/status/results directly in an operational database.

Run `GOCACHE=/private/tmp/epcerp-go-cache ./backend/scripts/verify-document-reader.sh` after building the worker image to test the actual production reader binary, OCR and legacy converters offline in a disposable container. Fixtures contain synthetic test text and do not use any operational database. The normal Go check/verify gates cover hexagonal boundaries, owner/portfolio authorization, encryption, state/version rules, deduplication, lease fencing and transactional audit rollback. Tender-specific business-field/BOQ extraction is the next increment. Other-process integration, retention/capacity policy, advanced scan/table accuracy and richer progress remain future work.

## Purchase FMS project dashboard (migration 22)

`/dashboard?screen=purchase-fms` is the normal purchasing landing page. It lists
current purchaser assignments and PM oversight, or the explicit permissioned
portfolio. Selecting a project opens its scoped purchase menu. Available owner
assignment/history is independent of project-manager assignment. Purchasing view
and read are required; Auth reconciles assignment/portfolio permissions on startup.
The module is always registered in this isolated stack.

Rebuild the API, document-worker and frontend together, run the migrate service,
then start the matching images. Migration 22 adds purchasing workspace ownership
and immutable history; it does not import operational Python purchase records.
Source-derived BO/indent/sourcing/PO/inspection/delivery execution and progress are
subsequent migrations. Current empty execution displays “No purchase records”.
Reference steps are Planned. See `docs/GO_PURCHASE_FMS_DASHBOARD_SLICE.md`.

## Expanded Project Master capture (migration 23)

Projects → Create draft project now captures business/site/location, scope,
operations user and assigned PM, exact project value/currency, LOI/PO references,
basis/milestone/planned dates, duration/buffers and remarks. Saved values appear in
View → Project details. Location dropdowns need the existing location-reference
read permission. Project creation still requires live master read/write and
portfolio write; no new role-name bypass is introduced.

Deploy API/worker/frontend images together with migration 23. Existing minimal
drafts are preserved without dummy values; populated capture cannot be silently
rolled back. Working-calendar/holiday migration is pending, so net running days
are null rather than an inaccurate total. Source fields use exact decimal/date-only
string contracts. See `docs/GO_PROJECT_CAPTURE_ALIGNMENT.md`.

Client import removal (11 October 2026): rebuild API/worker/frontend and run the
migrate service before restart. Migration 59 supplies empty defaults for retired
client source columns so form creation works while historical provenance is kept.
The client/project legacy import scripts and routes are retired.

### Auth-owned profile images — 11 October 2026

Migration 60 adds image references without rewriting existing database photos.
`setup.sh` generates `EPCERP_IMAGE_ENCRYPTION_KEY` once in ignored `.env`; keep this
32-byte hex secret with backups and preserve it across rebuilds. New profile uploads
are processed by shared helpers and encrypted before private MinIO storage. Auth
owns metadata/audit transactions and authenticated reads; browser URLs remain
unchanged. Replaced/removed objects remain encrypted pending a separate retention
cleanup policy. See ../../docs/GO_AUTH_PROFILE_IMAGES.md.

### Offline transfers only — 18 October 2026

Supplier and manufacturer import tools no longer connect to a source application,
container, interpreter or database. They are optional explicit operator tools;
ordinary epcERP startup, build and workflows do not run them. Set
`EPCERP_SUPPLIER_IMPORT_FILE` or `EPCERP_MANUFACTURER_IMPORT_FILE` to a privately held,
reviewed JSON snapshot and optionally `EPCERP_FRONTEND_URL` (default
`http://localhost:3001`). Existing operator credentials and import permissions remain
required. Manufacturer input is the reviewed array of master rows; supplier input is
an object containing `suppliers` and `documents`, with each document's reviewed bytes
in base64 `content`. Retain source IDs/timestamps for mapping and idempotency. Scripts
verify copied values, stable source hash, repeat imports and downloaded file hashes
before writing reconciliation manifests. A source snapshot containing document bytes
is sensitive: keep it outside Git and use the existing private snapshot handling.
Historical source names in older snapshot files/manifests describe provenance only;
they are not live connections. No import or new approval was run during this repair.
