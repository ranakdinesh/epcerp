# Native platform services

All application request handling and queue orchestration runs in Next.js/Node.
`backend-request.ts` dispatches platform and business routes in process. Unowned
routes always return 404 and cannot make a network fallback.

- `organisation.ts`: departments, reporting lines and team directory.
- `inventory.ts`: live cross-module responsibilities used by offboarding.
- `offboarding.ts`: manager-approved plans, live transfer/version checks and history.
- `profile.ts`: private contact edits and encrypted profile images.
- `governance.ts`: Director designation and immutable history.
- `calendar.ts`: holidays and transactional project schedule recalculation.
- `storage.ts`: general encrypted documents using the existing key derivation.
- `processor.ts`: authenticated private offline document processor client.
- `health.ts`: liveness and owned schema readiness.

Authentication, authorization, passwords, sessions, permissions, database tables,
encryption formats and existing evidence remain in their current modules/tables.
The platform services use live Auth capabilities and existing transaction locks.
Offboarding requires transfers in the owning workspaces and does not silently
transfer responsibilities or deactivate an account.

`../tenders/intake/verification.ts` owns receipt leases, scanning/verification,
member encryption and immutable manifests. `maintenance.ts` owns the existing
retention policy with fenced purge leases. The private processor in
`frontend/tools/document-processor` invokes Python, ClamAV, Poppler, Tesseract and
LibreOffice, with no Go executables or LLM calls. It has no public service port.
Historical tender readings and TSC snapshots remain readable/exportable in
`../tenders/intake/historical.ts` and `../tenders/tsc/legacy.ts`; local model/OCR
queue retries remain retired in favour of cloud/manual preparation.

The current schema and its seed data are in
[`000119_baseline.up.sql`](../../../migrations/000119_baseline.up.sql).
The migrator installs this baseline on an empty database. An EPCERP database
already at version 119 keeps its current schema and data. Add subsequent
changes as sequential migrations starting at 120 and record their hashes in
`history.json`.

Run `node tools/migrate.mjs up` with application writers stopped and a verified
private backup. The CLI refuses unowned, dirty, newer, and pre-baseline schemas.
Application startup performs no DDL.
