# epcERP Go backend

Independent project for the owner's future use, developed in parallel with the
client's Python VTPL Core. Each project has its own database, configuration,
credentials and releases. Python keeps SQLAlchemy/Alembic; epcERP uses Go-owned
SQL migrations and sqlc/pgx. The VTPL CORE frontend name and required API shapes
are retained. No shared database, token/session exchange or replacement cutover.

Module: `github.com/ranakdinesh/epcERP`. Go **1.26.2**, chi/v5, sqlc **v1.30.0**,
pgx/v5, golang-migrate **v4.19.1**, zerolog. Real hexagonal module boundaries are
checked with Go AST tests; modules expose published contracts rather than internals.

## Database and startup

Create a new, empty PostgreSQL database for epcERP with its own credentials.
Provide `EPCERP_DATABASE_URL` through your shell or secret store; no credentials
belong in tracked source, logs or command examples. Only native `postgres://` or
`postgresql://` URLs are accepted. `VTPL_DATABASE_URL` and Python driver schemes
are not supported. The migration and API commands use the independent URL.

For IDE development on this workstation, `backend/.env.local` contains ignored
loopback overrides and loads the private epcERP Docker secrets. PostgreSQL and MinIO
remain in Docker; the API and frontend run natively. Start only those infrastructure
dependencies, apply migrations, and run the API with:

```sh
./deployment/epcerp/ide-infrastructure.sh start
cd backend
make migrate-local-up
make run-local
```

Run `make migrate-local-version` to inspect the native database migration version.
When started from `backend`, plain `go run ./cmd/api` and
`go run ./cmd/migrate up` also load the ignored `.env.local`. Existing process
variables remain authoritative, so Docker and production continue using their
injected settings.

```sh
cd backend
make migrate-up
make migrate-version
make build
go run ./cmd/api
```

`cmd/migrate` embeds one ordered history from `backend/sql/migrations/` and Auth's
`internal/modules/auth/sql/migrations/`. sqlc reads the same history through the
ordered manifest in `sqlc.yaml`; Auth queries live in `auth/sql/queries/` and address
the PostgreSQL `auth` schema. The deployed binary needs no external SQL directory.
See `../docs/GO_AUTH_SCHEMA_ISOLATION.md`. The backend is one Go
module with no sibling SQL-module dependency. Migrations are explicit operator
commands. Connection setup verifies connectivity with a bounded ping; it never
creates tables or checks migration versions. Apply migrations before starting the
API or worker; Compose enforces this order through its migration service.
The migrator also rejects unrelated populated databases. `epcerp_schema_identity`
identifies the Go-owned schema; `schema_migrations` tracks its version/dirty state.

The first six migrations cover Auth, reference administration, Clients, private
document storage, catalogue eligibility definitions and Manufacturers. Migration 7
adds the immutable business-event journal and leased delivery outbox. Migration 8 adds draft Projects and immutable manager handovers.
Migration 9 adds personal profiles and account security audit revisions. Migration 10
renames the profile table to `user_profile` without changing stored data; current
head is 65, including encrypted profile image references. Business
records start empty. Auth reconciles its own Go permission registry and recovery
administrator on every API startup. This is independent provisioning,
not a migration of the client database or all future ERP capabilities.

Bounded rollback requires an explicit positive count, no larger than the clean
applied version:

```sh
make migrate-down STEPS=1
```

Down reverses the selected migration (migration 10 restores the former table name); use it only for an intended rollback.
There is no default down-all, force or drop command. A failed migration remains
visible as dirty and needs operator investigation. Add new numbered up/down files
for subsequent schema changes; update `schema.LatestVersion` and regenerate sqlc.
Do not edit an applied migration in a deployed environment.

sqlc reads the same migration history, ignores down files and generates only used
models. Each module still owns its queries and private generated pgx package.
There are no separate compiler schema copies, SQLAlchemy models or Alembic steps.
See the official [sqlc migration documentation](https://docs.sqlc.dev/en/latest/howto/ddl.html)
and [golang-migrate](https://github.com/golang-migrate/migrate).

## Implemented capabilities

- Event delivery foundation: immutable event metadata/attempts, transactional
  publication, leased claims, bounded retry/crash recovery; Manufacturer writes are
  the first publisher. No notification consumer or project timeline is exposed yet.

- Projects: draft registration, scoped list/detail, eligible assignment options,
  versioned manager reassignment and immutable handover history. One manager can
  manage several projects. Writes publish restricted project events atomically.
  Requires authenticated, authorized access. See
  `../docs/GO_PROJECT_IDENTITY_OWNERSHIP_SLICE.md` for permissions and Python-rule mapping.
  Activation, process owners, tasks and notification recipients remain unfinished.
- Health: public `/api/v1/health/live` and `/api/v1/health/ready`.
- Auth: sessions, users, roles, live permissions, refresh rotation/replay handling,
  activity and recovery safeguards.
- Clients: six directory methods, tax/reference checks, contacts/history and lifecycle.
- Reference administration: client types, industries, states and cities, including
  active-parent rules and state/city cascade.
- Manufacturers: list/create, versioned profile/lifecycle PATCH and immutable history.
- Private storage: encrypted upload, metadata/list/download and soft deactivation.
  Requires configured private S3/MinIO credentials.
- Master: existing catalogue categories/subcategories/units, configuration templates
  and item administration, references, clients, vendors and manufacturers are grouped
  under `internal/modules/master`. The module owns shared domain, ports, application,
  contracts and adapters, with an entity service/repository per concern. HTTP handlers
  live in adapters/httpx, with request/response exchange types and conversions in
  httpx/dtos; persistence lives in adapters/repository.
  repository/store.go initializes the single generated SQLC query object for a supplied
  connection or transaction. All Master queries compile into repository/sqlc;
  repository/dtos owns SQL row/value and stored JSON snapshot conversions.
  `cmd/api/master.go` constructs Master services and repositories;
  `master/adapters/httpx/routes.go` receives those services and binds routes once,
  without database or repository dependencies. The unused historical material-catalogue reader is
  removed; migration history is preserved. See `../docs/GO_MASTER_MODULE_REFACTOR.md`.

All implemented modules are registered at startup without module-enable flags. The API currently listens on `127.0.0.1:8001`.
Retained `VTPL_` keys configure platform/security, Auth and storage in this project's
own environment. They do not cause it to load Python configuration. Auth
requires `VTPL_JWT_SECRET_KEY`, `VTPL_BOOTSTRAP_SUPERADMIN_EMAIL` and
`VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD`. JWT issuer/audience default to `epcERP` /
`epcERP-api`; sessions belong to this project only. The frontend must use its own
backend URL and server cookie context for each deployment.

## Go-native contracts

Names use Go Unicode simple-fold equivalence; whitespace uses Go's Unicode rules.
`ß` and `ss` are distinct, while uppercase/lowercase equivalents compare equally.
The large generated Python Unicode maps and generators are removed. Codes use
native uppercase with their existing ASCII validation/derivation rules. Normalize
future imported data using the Go rules and reconcile any collisions.

JSON booleans must be booleans. Manufacturers versions must be positive signed
32-bit JSON integers; strings, booleans and decimal values are rejected. Timestamps
use UTC RFC3339Nano. Existing frontend field names/routes and safe validation/error
shapes are retained where useful; Python's exact coercion/rendering is not required.
Standard PHC Argon2 password hashing, JWT and AES-GCM remain sound independent Go
implementations. Identifier/actor/provenance, exact JSON numbers, authorization,
locks, versions and atomic audit rules remain enforced. Zerolog omits secrets,
personal data and full inputs; write-success events follow commit.

## Verification

```sh
make generate
make check build
make verify
```

Generation/checks use Go and sqlc only. `make verify` additionally needs PostgreSQL
CLI tools (`initdb`, `pg_ctl`, `createdb`) and free loopback port 8001. It accepts
no external test DSN: Go tests provision and remove their own temporary clusters,
apply Go migrations and run the actual Go API. No Python interpreter, virtualenv,
Alembic or client database is used. Restricted environments may need populated
`GOMODCACHE` / `GOCACHE` and permission for loopback processes.

Checks cover architecture, formatting, vet, race tests, registry integrity,
sqlc vet/diff and both binary builds. Owned database tests cover fresh migration,
repeat up, rollback/reapply, ownership/stale-version rejection, Auth/session/live
permissions, Clients/contact history, references/cascades, Manufacturers versions,
immutable audit/late-commit rollback/concurrency and structured-log privacy.
Storage tests use real Go-migrated PostgreSQL and AES-GCM with an object-port fixture:
persistence, corruption and PUT/audit rollback are covered, but this suite does not
claim S3/MinIO network-adapter integration. Historical Python parity harnesses are
removed; their earlier results are historical evidence, not current release gates.

Explicit data export/import with validation, provenance, identifier mapping and
reconciliation is a separate unfinished increment. No operational records have
been imported. Independent deployment and target PostgreSQL 17 acceptance remain
pending. Current delivery contract: [GO_INDEPENDENT_FOUNDATION_SLICE.md](../docs/GO_INDEPENDENT_FOUNDATION_SLICE.md).


## Proposed project operating model

Before the next business migration, review [GO_PROJECT_OPERATING_MODEL.md](../docs/GO_PROJECT_OPERATING_MODEL.md).
It proposes business-module grouping around global Masters/Catalogue, Clients,
Vendors, Tenders, Projects, Purchasing and Finance, with focused capability services
and project reporting for doers, resolvers, blockers and recovery actions. Award/
readiness and escalation decisions remain open. The Master grouping was approved and implemented on 11 October 2026; see
`GO_MASTER_MODULE_REFACTOR.md`. Other grouping proposals require explicit review.
Use the current baseline/backlog for delivered project/tender/purchasing workflows.
Draft project identity and manager handover are implemented separately.

## Independent frontend and governance roadmap

The Go frontend is ../frontend (VTPL CORE branding, local port 3001), with its
own sessions and server-only EPCERP_API_URL. Implemented Auth/Clients/references and draft Projects/manager ownership
are published there. Process/team execution, Finance and communication consumers
follow ../docs/GO_COMMUNICATION_GOVERNANCE_SPINE.md and remain unfinished.

## Event delivery foundation

See ../docs/GO_EVENT_DELIVERY_FOUNDATION_SLICE.md for verified contracts, integration
and operator instructions. Upgrade the independent database explicitly to head 10
before starting the rebuilt API. Publication uses the source transaction; queue
consumers run after commit and must deduplicate their effects. No empty worker or
fake notification delivery is started by API boot. Draft Projects ownership and assignment history are implemented. Cross-module
project history/inbox
and Director/Finance enforcement are the next unfinished capabilities.

## Local isolated Docker preview

The running local stack is available at http://localhost:9092. Setup and operator
instructions: [deployment/epcerp/README.md](../deployment/epcerp/README.md).
It owns a fresh PostgreSQL 17 database/volume, migration service, Go API and Go
frontend. Only loopback port 9092 is published. API listener can be explicitly set
using EPCERP_HTTP_ADDRESS; the default host listener remains 127.0.0.1:8001.
Credentials are generated into the ignored private deployment .env; run the local
credentials helper to retrieve the recovery administrator login.


## Personal account

Auth structure (11 October 2026): sessions, user administration, roles, account
security and bootstrap reconciliation have separate named services and narrow
repository ports. HTTP route registration receives services only; request parsing
and response conversion live in adapters/httpx/dtos. Persistence and SQL value
mapping live in adapters/repository and repository/dtos; store.go alone creates
module-owned repository/sqlc query objects. Auth's Argon2/JWT/configuration adapters
remain Auth-owned under adapters/security. cmd/api/auth.go constructs the services
and publishes one identity resolver to consumers. Domain errors contain semantic
kinds, with status codes mapped at HTTP boundaries. See
../docs/GO_AUTH_MODULE_REFACTOR.md for the structure and retained contracts.

Profile belongs to Auth's domain, ports, application and adapters; ProfileService
uses the shared repository Store, transaction manager and generated SQLC package.
Its table remains `user_profile`; API/proxy URLs stay `/api/v1/auth/user_profile/...`
and `/api/auth/user_profile/...`. Active users have intrinsic account.profile.read/write
and account.security.write permissions; writes always target their own account.
Display name, photo upload/removal and verified account security edits retain their
existing contracts. Password changes revoke refresh sessions.

`helpers/images.go` validates PNG/JPEG, bounds decoding, crops and resizes to a
256×256 JPEG (512KiB upload, maximum 2048×2048 input). Shared
`infrastructure/image_storage.go` encrypts immutable objects before MinIO storage
and verifies/decrypts reads. `object_storage.go` provides the common bounded MinIO
client, also used by document storage. API responses expose neither keys nor hashes.
Set `EPCERP_IMAGE_ENCRYPTION_KEY` to a persistent secret 32-byte hex key. Local
setup generates it once in ignored .env; preserve it when deploying or rotating
other credentials. Migration 60 adds object references/digests. New photos live in
MinIO; existing database photos remain readable. Removing/replacing a photo clears
its active reference; old encrypted objects remain for a future retention/cleanup
use case. Commit errors never trigger unsafe deletion of potentially referenced objects.
See ../docs/GO_AUTH_PROFILE_IMAGES.md for ownership, migration and verification.

## Client entry

Clients are entered and edited through normal forms. The legacy client import
service, route, permission declaration, source mapping and pending-fields response
are removed. The dependent project legacy import path is also removed. Future Excel
import must be implemented in a separately scoped package. Existing applied schema
history and stored provenance are preserved; active queries do not consume it.
Migration 59 supplies empty defaults for retired required source columns so normal
client creation needs no import metadata.

Client contact coverage (migration 11): contact_type accepts EIC, HOD,
SITE_CONTACT or an empty unclassified value. Multiple contacts per type are
supported; department is optional. Missing contacts/types do not block creation;
responses expose missing_contact_types based on active contacts for follow-up.
Entered-contact, statutory and directory validation remain enforced. Upgrade
explicitly with the Go migrate command before starting the updated API. See
../docs/GO_CLIENT_CONTACT_TYPES.md for the contract and acceptance checks.

Vendors/Suppliers read/import slice (migration 12): provide EPCERP_SUPPLIER_DOCUMENT_KEY as a private 32-byte hex secret.
Suppliers owns master/provenance; Supplier Documents owns AES-256-GCM encrypted
PostgreSQL evidence, SHA256 integrity and source provenance, using the supplier
published directory in shared transactions. Source import retries are idempotent;
central restricted business events are atomic with writes. Only read/import APIs
are implemented. Normal edits/uploads/Finance verification remain separate work.
See ../docs/GO_SUPPLIER_MASTER_IMPORT.md for endpoints, permissions and acceptance.


Project working calendar (migration 24): `work_calendar` owns dated holidays and publishes the reader used by Projects. Authenticated GET/POST/PATCH `/api/v1/work-calendar/holidays` requires settings.holidays read/write, JSON writes up to 16 KiB, a reason, active flag and expected version (0 for create). Existing HTTP rate/payload controls apply. POST `/api/v1/projects/schedule-preview` is read-only but requires project creation authority, strict JSON up to 64 KiB. Creation recalculates all derived values server-side. Holidays and affected saved project calculations/history/events share one transaction. Go holidays start empty; no Python database is shared. See ../docs/GO_PROJECT_WORKING_DAYS_SLICE.md.


Shared pure text functions live in `helpers/normalization.go`. Clients, references
and manufacturers use the same `helpers.Fold`; email lowercasing uses `helpers.Lower`.
Capability-specific whitespace and name policies stay with their existing callers,
so persisted comparison keys keep their current behavior. Architecture checks guard
helpers from depending on module code or runtime infrastructure.

Shared document encryption and bounded PDF/raster inspection live in
`internal/infrastructure/document_cipher.go` and `document_media.go`. Callers supply
their size policy; Master retains its domain errors.
Master authentication uses `internal/infrastructure/auth.go` and the pure
`internal/identity.Principal` value; permissions remain enforced by application services.

## Purchase FMS structure

Purchase workflows live in one `internal/modules/purchase_fms` unit. It owns BO,
holds, indents/groups/documents, client rules, quotations/approval workflow,
packages, sourcing, technical/evidence, orders and acceptance/inspection/delivery
through engineer confirmation, including progress reporting. Named workflow services
and repositories share domain/ports/contracts, HTTP routes/DTOs and one generated
SQLC package. Only repository/store.go initializes queries; shared transaction
binding preserves local/central audit atomicity. Finance decisions and project
records remain connected through published ports. Existing URLs, permission codes,
stored snapshots and history labels are preserved. See
../docs/GO_PURCHASE_FMS_CONSOLIDATION.md for scope and verification.

## Shared email and account onboarding

`internal/infrastructure/email.go` supplies the reusable Brevo transport and
professional escaped HTML/plain-text rendering. Modules inject a port and call
`Send(ctx, recipientEmail, subject, message)`; Auth owns welcome/reset content and
an encrypted credential email outbox. Migration 79 adds temporary password expiry,
mandatory first-login change, immediate credential-version session revocation,
immutable reasoned history and leased delivery retries. Existing users are unchanged.

Private configuration: `EPCERP_BREVO_API_KEY`, `EPCERP_EMAIL_SENDER_EMAIL`,
`EPCERP_EMAIL_SENDER_NAME` (old recovery sender aliases are accepted), plus
`EPCERP_PUBLIC_URL`, default `https://erp.vishaltechnopower.in`. The sender must be
verified in Brevo. `EPCERP_RECOVERY_URL` is superseded by the common public URL.
Configure key and sender together and restart the API. Unconfigured email leaves
read/edit access available but creation/reset fails atomically with 503.

New user creation accepts username/email/role_names, generates a temporary password
server-side and queues a welcome email. The Users key action requires a reset reason.
Temporary passwords expire in 24 hours and must be changed before workspace access.
No temporary password is returned to the administrator. The first login route is
`/change-password`; public recovery uses `/forgot-password` and `/reset-password`.
See `../docs/GO_ACCOUNT_ONBOARDING_EMAIL.md` for API changes, delivery semantics,
configuration and verification. Brevo 403/sender setup and real delivery remain pending.
