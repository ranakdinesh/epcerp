# Master Data

One module owns the shared directories used across ERP workflows: clients,
vendors/suppliers, items, manufacturers, categories, states/cities, client types and
industries. Each area has its own files inside this module; it is not a separate
application or Docker service. Auth remains a separate security boundary.

## Structure

```text
server/
  auth/access.ts                 published permission-checking contract
  db.ts                          connection pool and transactions
  errors.ts                      shared application failures
  events/journal.ts              transactional writer for Master business events
  master/
    http.ts                      adapter for migrated browser operations
    shared/
      references.ts              existing reference readers and shared locks
      text.ts                    compatible persisted name keys
      simple-fold.json           generated Go Unicode mapping
      validation.ts              shared IDs and validation failures
    clients/
      domain.ts                  records, tax/contact rules
      validation.ts              input and query validation
      service.ts                 use cases and permission enforcement; no SQL
      repository.ts              parameterized SQL and response mapping
    references/
      domain.ts                  kinds, permissions and compatible code generation
      validation.ts              strict input and query parsing
      service.ts                 directory lifecycle and state/city transactions
      repository.ts              whitelisted directory persistence
    catalogue/
      http.ts                    adapter for the four catalogue areas
      sql.ts                     generated reviewed SQL; no runtime Go dependency
      database.ts                typed query execution and precise timestamps
      validation.ts              catalogue input helpers
      serialization.ts           compatible hashes and JSON encoding
      json.ts                    lossless JSON and integer-token parsing
      full-fold.json             Go full folding for item model identities
    categories/                  category/subcategory/base-unit rules and history
    manufacturers/               eligibility, profile, lifecycle and imports
    templates/                   publication, controlled options and preview
    items/                       item identity, numbering, lifecycle and history
    vendors/                     supplier profiles, lifecycle and legacy import
      domain.ts                  profile validation and exact payment arithmetic
      service.ts                 permission checks and transactional use cases
      repository.ts              supplier rows, provenance, history and events
      http.ts                    ownership limited to profile HTTP routes
    documents/                   supplier documents within the same Master module
      domain.ts                  metadata, import rules and public response shape
      service.ts                 upload/read/archive use cases and transactions
      repository.ts              reviewed SQL, history and business events
      approval.ts                transactional upload/replacement review hooks
      files.ts                   compatible encryption, media inspection and S3
      http.ts                    bounded multipart/JSON and secure binary responses
```

Add the next area under `master`, rather than creating a new top-level module for
each small directory. Keep each area's validation, repository and permissions
distinct; do not combine all logic into one service file. Future mobile adapters
can call the same application services.

## Current ownership

**Clients, client types, industries, states, cities, Items, categories, base units,
manufacturers, configuration templates, vendor profiles/lifecycle and supplier
document handling** run natively in Next.js. Reviewer assignment, Finance decisions
and manual approval reconciliation remain Go-owned. Clients and
reference administration use the same transaction advisory
lock. This prevents a client save racing a reference deactivation or city/state
reassignment. Go consumers continue reading the same tables.

Set `EPCERP_CLIENTS_OWNER=nextjs` on both API and frontend. Local Compose defaults
both to Next.js; absent the setting, existing deployments keep Go ownership.
The native adapter serves the existing browser routes. Go stops registering the
six migrated Clients HTTP operations. Its published client-directory reader stays
available to Projects and other unmigrated consumers. There is no fallback to Go
after a native operation fails. Explicit `EPCERP_CLIENTS_OWNER=go` on both services
is the rollback path; leave `EPCERP_AUTH_OWNER=nextjs` unchanged.

`EPCERP_REFERENCES_OWNER=nextjs` independently migrates all four reference
directories. Set it consistently on API and frontend; local Compose defaults to
Next.js, while an absent setting elsewhere retains Go ownership. Go unregisters
only these HTTP operations and retains its published reference readers. Set
`EPCERP_REFERENCES_OWNER=go` on both services for reference rollback; keep Auth and
Clients ownership unchanged. Native errors never fall back to a second writer.

Reference writes require `settings.client_types.write`, `settings.industries.write`
or `settings.locations.write`; reads require their respective `.read` permission.
State deactivation retires active cities atomically. Reactivating a state does not
reactivate its cities. City creation/editing and activation require an active
parent state. Edits preserve existing IDs, creation metadata and inactive status.
Names and generated codes retain Go's whitespace, Unicode and duplicate semantics.
Reference JSON bodies are bounded to 4 KiB at both the browser proxy and native
adapter. Responses preserve the existing five-field contract.

`EPCERP_CATALOGUE_OWNER=nextjs` transfers Items and its three supporting areas
together. Local Compose sets it consistently on API and frontend. An absent flag
outside Compose keeps Go ownership; `go` on both services is the rollback path.
Go unregisters only the migrated HTTP routes and retains catalogue readers used
by project materials and purchasing. Client purchase policies remain Go-owned even
though their URLs are nested below `/clients`; native route ownership explicitly
excludes them. No native operation falls back to Go after a failure.

Each catalogue area has `domain.ts`, `service.ts` and `repository.ts`. Domain rules
are pure; services resolve permissions and transactions; repositories own SQL and
audit/event persistence. No new Docker service was introduced. The native event
writer appends the same zero-delivery-target business events inside the subject's
transaction, preserving the Go journal contract.

Catalogue permissions retain their existing `settings.categories`,
`settings.manufacturers`, `settings.configuration_templates` and `settings.items`
read/write grants, with separate import grants. Corrections require versions and
reasons. Template publication appends a revision and retains old field/option
codes. Issued item identity, snapshots, SKU numbers and audit records stay
immutable. SKU issuance remains 001–999 per category code, with shared advisory
and row locks preventing duplicate identities and stale updates.

Item identity uses Go-compatible NFKC/full-fold model normalization and the
existing sorted ASCII JSON hash policy. Legacy model candidates are reused without
rewriting evidence. Template preview has its own existing identity policy; do not
combine those hashes. Legacy JSON numbers and timestamp microseconds are preserved
through reads and audit snapshots. Native catalogue code requires Node 22+.

`EPCERP_VENDOR_PROFILES_OWNER=nextjs` transfers only supplier list, create, update
and legacy-import operations. Set it consistently on API and frontend; local
Compose defaults to Next.js, while an absent flag elsewhere keeps Go ownership.
Set this flag to `go` on both applications for profile rollback without changing
the other module owners. Document ownership is controlled separately below.
`/suppliers/{id}/verification`, including reviewer assignments, continues through
Go. Go's supplier directory remains available to those consumers.

Profile reads preserve the existing alternative grant: either
`settings.suppliers.read` or `accounts.vendor_verification.read`. Writes require
`settings.suppliers.write`; adoption requires `settings.suppliers.import`.
Updates retain code, ID, creation fields and import provenance, require the
current version, and append immutable supplier history plus the business event
inside the same transaction. Lifecycle changes use the existing profile PATCH.
Document rows and approval state are not rewritten by profile updates.

Qualification remains separate from Finance verification. Go computes its current
evidence fingerprint from the same supplier version, lifecycle and GST fields.
Stale Finance commands remain rejected after native profile changes. Payment terms
use exact decimal arithmetic, including the existing inclusive 0.01 total tolerance,
zero-total allowance and integer credit-day rule. Profile bodies remain bounded
to 128 KiB; source identity/revision/timestamps stay hidden from public responses.

## Supplier documents

`EPCERP_VENDOR_DOCUMENTS_OWNER=nextjs` on both applications transfers list, upload,
legacy import, preview/download and archive HTTP operations. Local Compose defaults
to Next.js; absent the flag elsewhere, Go remains the owner. Explicit `go` on both
applications is the rollback switch. Reconciliation at
`/suppliers/{id}/documents/reconcile-approvals` stays in Go, alongside Finance
verification and reviewer assignment. Published Go evidence readers remain available
for Finance and purchasing. Native failures never retry through a Go writer.

Reads require `settings.supplier_documents.read` or the existing Finance read grant;
upload/archive requires document write; legacy import requires document import.
Auth's published live-capability contract resolves these permissions without exposing
credentials. Archive/download access follows the existing creator/import/Finance
rules. Supplier row locks serialize replacements, archives and Go Finance decisions.
Document history, approval history, review/correction tasks and business events are
written in the same database transaction. Go Finance rejection creates a correction
task that a native replacement completes as `resubmitted`.

Use the existing server-only `EPCERP_SUPPLIER_DOCUMENT_KEY` and `VTPL_STORAGE_*`
settings unchanged. Existing inline ciphertext and stored bucket/key references are
read without relocation or re-encryption. New immutable objects contain compatible
AES-256-GCM nonce/ciphertext/tag bytes, with a verified plaintext SHA-256. No object
is deleted during replacement/archive or after an uncertain commit; failed database
writes can leave encrypted orphan objects for the existing offline storage operator,
matching Go's policy. Metadata never exposes ciphertext or storage references.

Multipart bodies are bounded before parsing to 15 MiB plus 64 KiB overhead; individual
files remain limited to 15 MiB. JSON import/archive is bounded to 512 KiB. The proxy
checks upload authority before buffering, then native services recheck live authority
before writes. Actual media bytes determine the preview MIME type. PNG/JPEG/GIF
are fully decoded within the existing dimension/pixel limits; PDF acceptance retains
Go's header/trailing-EOF policy. This is not a new full PDF validator or malware scan.
Previews retain sandbox CSP, same-origin framing and nosniff through the browser proxy;
downloads remain authenticated attachments. Storage requests and idle sockets have
bounded timeouts.

Document tests additionally require `VTPL_STORAGE_ENDPOINT`, local storage credentials,
`VTPL_STORAGE_SECURE=false` and a dedicated bucket beginning `epcerp-documents-qa-`.
The runner supplies only synthetic signing/encryption keys and serializes test files
that share the guarded QA database. It verifies Go/Node encryption in both directions,
supported media, live grants, object and inline reads, stale/concurrent writes, imports,
atomic history/approval failures and task completion. Production deployment and private
cloud uploads remain separate from this local workflow.

## Existing data and rules

No schema change or new migration system is introduced. Existing records, IDs,
client codes, actor/timestamp fields, import provenance and contact rows remain in
the same database. Updates retain client codes and never rewrite legacy columns.
Omitted/deactivated contacts are retired, not deleted; reactivating a client does
not reactivate those contacts. Existing permissions stay granular:
`settings.clients.read` and `settings.clients.write`.

The service resolves active client type, industry, state and city selections,
checks city/state membership, verifies PAN/GSTIN format/checksum/state consistency,
and validates contact ownership. Imported name keys use Go's Unicode simple fold,
including its treatment of `ß`, Greek sigma, dotted I and whitespace. Do not
replace this with JavaScript lowercase or a locale-dependent comparison. PostgreSQL
timestamp fractions are retained in responses, including microseconds.

This preserves the existing Clients audit fields and soft-retired contacts. The
old Clients directory has no immutable revision-history API; this migration does
not invent one or move purchasing's client-policy history into Master Data.

## Verification

Provide `EPCERP_DATABASE_URL` for a deliberately isolated QA database whose name
starts with `epcerp_auth_qa_`, then run from the repo root. The combined runner
includes Auth's stricter database-name safety guard:

```sh
bash frontend/scripts/verify-master.sh
```

The runner applies this checkout's migrations only to the QA database, compares
native validation with the actual Go parser, tests real SQL behavior and runs the
Auth regressions. Email is mocked. The test-only failure constraint used to verify
rollback is installed and removed only inside that guarded QA database.
Reference tests also compare all four kinds against Go, exercise live permissions,
parent filters, duplicate scopes, preserved metadata, cascade rollback, inactive
parent guards and concurrent state/city lifecycle operations.
Catalogue checks compare native rules/hashes with the actual Go code, including
imports, Unicode models and template previews. Real SQL checks cover optimistic
versions, atomic audit failures, revision retention, concurrent deduplication,
immutable evidence, legacy candidate reuse, JSON/timestamp precision and SKU
exhaustion. Failure constraints are confined to the guarded QA database.
Supplier checks compare 106 validation/import/payment fixtures with Go, exercise
the alternative Finance read permission, concurrent versions, source provenance,
immutable history and rollback after audit/journal failure. Profile writes are
checked separately from the Go document, assignment and Finance browser boundary.

The native query registry uses the existing reviewed catalogue SQL during the
gradual migration. Regenerate it after deliberately changing those queries:

```sh
node frontend/scripts/sync-master-catalogue-sql.mjs
node frontend/scripts/sync-master-catalogue-sql.mjs --check
```

The combined runner checks that the registry is synchronized. Go remains a
development parity oracle and the existing schema migrator; it is not invoked by
native Next.js requests. Regenerate full model folding after a Go Unicode upgrade:

```sh
cd backend
go run ./tools/catalogue-compatibility fold > ../frontend/src/server/master/catalogue/full-fold.json
```

Regenerate the checked-in Unicode mapping after a deliberate Go Unicode upgrade:

```sh
cd backend
go run ./tools/master-compatibility > ../frontend/src/server/master/shared/simple-fold.json
```

The browser mutation verifier `frontend/scripts/verify-master-clients.mjs` is
restricted to an isolated server at loopback port 59092. It tests create/edit,
lifecycle, reference selectors and visibility through Go's project directory.
Operational preservation and full-stack readiness checks are recorded separately
under `deployment/epcerp`.
