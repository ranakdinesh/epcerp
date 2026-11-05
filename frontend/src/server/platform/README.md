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

Schema history is now owned by `frontend/migrations`. The first 101 SQL files
are byte-identical copies of the original owned Go migration history; provenance
hashes are in `history.json`. Future migrations belong here. Run the explicit
Node CLI (`node tools/migrate.mjs up`) with the application writers stopped and a
verified private backup. It refuses unowned, dirty or newer schemas; it has no
force/reset operation and the application never performs DDL on startup.
