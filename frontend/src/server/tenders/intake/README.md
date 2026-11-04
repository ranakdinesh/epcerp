# Native Tender intake

`EPCERP_TENDER_INTAKE_OWNER=nextjs` owns draft create/list/detail, batch retention,
receipt declaration/upload/seal/original download, verified draft publication,
and TSC format administration with Word/PDF exports. This owner requires
`EPCERP_TSC_OWNER=nextjs`; invalid mixed ownership fails closed.

All services reuse the existing owned PostgreSQL tables. There is no schema
upgrade in this batch, no ORM synchronization and no password/key replacement.
The Go API no longer registers the replaced draft/receipt/format endpoints under
this owner. Named capture, manager assignment, ownership and project provisioning
remain in Go for the next migration batch. Historical Go TSC exports and old
intake original/result/history reads remain available. New legacy OCR uploads
and retries are retired under the native intake owner.

- `access.ts`: current session/account/permission checks and shared actor quotas.
- `drafts.ts`: owner-scoped drafts, idempotent retention, historical artifact access.
- `receipts.ts`: declaration validation, Go-compatible operation fingerprints,
  version/lease fences, atomic events and stream claim/finalize transactions.
- `objects.ts`: private immutable UUID objects using the existing `EPCPKG01`
  framing, one-MiB AES-256-GCM chunks, final size/hash and disk-capacity checks.
- `publisher.ts`: publish verified, owned, eligible draft sources atomically.
  It does not create local OCR/readings or call an LLM. It resumes from SQL after
  restart and defers sources when the owner's current permissions are insufficient.
- `http.ts`: thin HTTP boundary with bounded JSON and existing response shapes.
- `../formats/`: format validation, immutable audit events, legacy question
  identity rules and frozen document exports.

Authorization is checked before reading request bytes, before claims and again
before upload finalization/returning originals. Object I/O is outside SQL
transactions. An upload failure aborts only that claim; orphaned objects and
retained source cleanup remain owned by the existing storage maintenance worker.
Read access never grants another owner's draft or receipt, including to superadmins.
Verified original members remain immutable. Source state and current verification
claim must match; unverified uploads cannot become TSC evidence.

Safety scanning, bounded archive extraction and Office normalization remain in
the isolated document-package worker. The Go draft publisher is disabled for this
owner. Existing encrypted originals, historical reading results and jobs are kept.
The historical text endpoint presents retained segments rather than recreating
retired local retrieval; original bytes and result JSON remain authoritative.

## PDF rendering and deployment

The Word template package is synchronized from the existing reviewed Go asset with
`node frontend/scripts/sync-tender-format-template.mjs`; `--check` verifies its hash
and parts. A format's saved questions/revision are frozen before document rendering.
The native format writer preserves unchanged legacy extraction IDs; changed meanings
receive new UUIDs and lose their legacy row mapping.

`EPCERP_TENDER_FORMAT_RENDER_URL` defaults to `http://document-worker:8082`.
`backend/tools/tender-format-preview/worker.mjs` is a private JavaScript host for
LibreOffice and the still-unmigrated Go reader child. It adds no Docker service.
The renderer receives only application-generated Word bytes with a short-lived,
context-specific HMAC using the existing document key. It runs one conversion at
a time, caps request/output sizes, uses private temporary directories, scrubs child
environment variables and terminates the conversion process group on timeout/exit.
The application also takes the same PostgreSQL CPU-slot lock used by Go workers.
The PDF is rendered from the identical Word bytes offered for download.

The renderer must remain on the private worker network; do not publish port 8082.
The frontend joins that network and mounts the existing private document-package
volume at `EPCERP_DOCUMENT_PACKAGE_ROOT`. It runs as UID/GID 10001, matching the
existing API/worker volume owner; encrypted source files keep their permissions
and format. File scanning/purging and other reading modules remain isolated.

## Checks

`bash frontend/scripts/verify-tender-intake.sh` requires a disposable database whose
name starts `epcerp_auth_qa_intake_`. It uses synthetic keys, builds a Go compatibility
oracle, checks the reviewed template package and runs the native integration tests.
Do not run a background QA worker against that unit-test database. Browser verification
uses a separate database and source volume, with cloud approval disabled.
