# Complete local Go application retirement — 5 November 2026

The local preview at http://localhost:9092 runs Next.js and the private Node
document processor. The Go API, document worker, package worker and TSC worker
are stopped. The old local reranker is also stopped. Postgres, MinIO object
storage and ClamAV remain as infrastructure. MinIO's vendor implementation is
not application code being migrated.

Application commit: `72215c9788b40b79167c95bb4349ddff4e9cd939` on
`codex/development`. All fetched remote branch tips were verified ancestors.
The frozen combined build also includes the 62 recorded ongoing Tender/provider
files; their original edits remain separate in the development working tree.
The exact overlay hashes, images, remote commits and checks are recorded in
[the machine-readable verification](GO_RETIREMENT_LOCAL_VERIFICATION_20261105.json).
No merge into main, push or production deployment was performed.

## Runtime and structure

Native platform modules now handle Director governance, work calendar and project
schedule recalculation, general encrypted documents, departments/reporting/team,
profile contacts/photos and offboarding. They retain live permissions, version
fences, existing encryption keys/formats, immutable histories and database locks.
Offboarding still requires the actual manager and transfers through the owning
workspaces; it does not reset passwords or deactivate accounts implicitly.

Native Tender code now owns package verification, claim renewal, immutable
manifests, retention maintenance and historical reading/TSC access and exports.
The private Node processor runs malware scanning and bounded Python/Poppler/
LibreOffice/Tesseract helpers. It runs no local model and makes no provider calls.
Old local reading/model queue retries remain intentionally retired; cloud and
manual TSC preparation remain the supported flow.

The owned migration CLI is now Node (`node /app/tools/migrate.mjs up`), using the
101 byte-identical SQL history files under `frontend/migrations`; provenance is
recorded in `history.json`. Schema 101 was already present and remained unchanged.
The CLI refuses unowned, dirty, ambiguous, negative or newer schema histories.
There is no reset/force command and no schema synchronization at app startup.
Future application changes and migrations belong in the frontend tree. The Go
source remains available for historical comparison and controlled rollback.

Default Compose excludes the old services with `profiles: [legacy-go]`.
`EPCERP_GO_DISABLED=true` prevents unknown routes falling back to Go, and the
configured legacy URL points at unreachable localhost port 1. Both Docker images
were built from the same frozen combined checkout. The processor was built using
the existing verified offline toolchain cache; its Dockerfile also supplies the
Go-free toolchain build stage for fresh builds. No Go application executables are
present at the processor's application paths.

## Verification

Feature checks: 240 frontend tests, 20 platform/document/migration tests, 22 Auth
compatibility tests, 14 project tests, 42 purchasing tests and 16 budget tests.
TypeScript, scoped ESLint and the production Next.js build passed.

Combined-stack checks: 62 authenticated Docker HTTP checks passed with Go
unreachable and cloud/email disabled. These cover native platform and master
routes, project/purchasing workspaces, budget save/approval, Director designation,
format PDF preview, scanned upload through receipt/verification/publication,
original download and manual TSC preparation. Synthetic files only were used.
The archived Go implementation was used only as an oracle in separate feature
parity tests; it was not used by the Docker application.

Post-cutover checks: 17 read-only HTTP checks passed against the live local
preview while every legacy application service was stopped. A 120-second internal
test access session was immediately revoked. The old configured bootstrap
password differs from the user's current password and was correctly rejected;
no password was reset. Normal test security/session/quota writes are recorded in
the verification JSON. Existing password hashes were checked unchanged.

Before cutover all application writers were stopped and a private custom-format
backup was verified through pg_restore listing and full archive decoding, with a
SHA-256 digest. All 192 application tables were compared before/after startup.
Only the expected superadmin role edit_version increment occurred; all other role
fields, password hashes, permissions and business records were preserved. Both
filesystem originals and all 22 existing S3 objects were unchanged. Existing
provider approvals, keys, budgets and model settings were preserved. The existing
21 model-call records stayed unchanged; no provider calls or real messages were
made during verification.

The private backup, credentials, rollback configuration and frozen checkout stay
outside Git. The protected backup location is recorded locally in
`/tmp/epcerp-retire-backup-path`; do not publish its private contents. Existing
volumes and keys were preserved, and the old images remain available for rollback.
This local verification does not constitute production release approval.
