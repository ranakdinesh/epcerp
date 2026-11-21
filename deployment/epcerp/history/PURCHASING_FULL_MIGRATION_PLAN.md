# Entire purchasing workflow migration

User-confirmed scope: the entire purchasing workflow, not just workspace ownership.
Integration branch: `codex/development`. Existing database, credentials, encrypted
files, history, permissions, explicit provider approvals and budgets must be preserved.
No production deployment, real messages or paid provider calls are authorized here.

Implementation order (all phases are required before claiming full completion):

- [x] Shared transaction/access/decimal contracts, workspace ownership, dashboard
      and progress over all purchasing stages.
- [x] BO preparation, register completion/reopen, requirement publication and PDF;
      formal indents, grouping, vendor documents, invitations/notifications and holds.
- [x] Quotations, encrypted attachments, tax/exact totals, comparisons/shortlists,
      budget preview, Accounts approval queue/review, task/inbox correspondence.
- [x] Vendor awards, allocations, sourcing/LOI documents/review, technical approvals
      and evidence uploads, client-stamped purchase orders and release/document paths.
- [x] Accounts acceptance, advances/disbursement, inspections/reports, quality and
      client clearance, dispatch, site receipt, GRN, engineer confirmation and linked
      final Accounts evidence/progress.
- [x] Every registered purchasing route and dependent purchasing queue/document
      path accounted for; Go HTTP ownership retired only after native parity checks.
- [x] Matching combined Docker API/migrator/frontend/workers built from one frozen
      integration checkout; owned migrations applied; private verified backup and
      preservation audit; isolated end-to-end tests and combined local smoke checks.

New implementations stay inside `frontend/src/server/purchasing`, organized by
business responsibility. Existing Go is the compatibility oracle and remains the
active runtime until the corresponding native path is verified and explicitly owned.
No generic permissive state machine or proxy facade qualifies as a migration. Source
snapshots, version checks, financial separation, immutable audit evidence, storage
signatures/encryption and invalidation of stale upstream evidence must remain.
Concurrent Tender/provider work is outside this feature's commits and must survive
combined local deployment. Record exact tested commits, overlay hashes, image IDs,
schema version and phase-specific versus combined verification separately.

## Implementation and verification

Native Next.js services now cover the complete workflow: ownership/dashboard,
BO and standalone material publication, formal indents, grouping, vendor documents,
client make rules and holds; quotation evidence/comparisons/shortlists, budget
preview and independent Accounts decisions; approved-group award allocations,
reviewed encrypted vendor LOIs, technical evidence and client approvals; signed
PO revision/release, vendor acknowledgement and independently reviewed advances;
inspection/witness points, IR/IRN, verified quality certificates, MDCC, dispatch,
site receipt, GRN/warranty, engineer confirmation and final Accounts handover.

`frontend/src/server/purchasing/README.md` documents the capability boundaries.
`paths.ts` and `http.ts` own the complete API surface. The single
`EPCERP_PURCHASING_OWNER=nextjs` switch retires Go purchasing, project-material and
final Accounts HTTP routes and upload guards; compatibility readers required by
unmigrated internal consumers remain. Native failures never retry through Go.

Synthetic feature checks run mutations through the native HTTP adapter and compare
saved records, immutable history, source snapshots and action projections with the
original Go composition. They cover exact quantities and prices, stale upstream
revisions, scoped authority/revocation, separate Finance decisions, Director-notice
rollback, encrypted-object interchange, LOI hashing/versioned reviews, multipart
input, PDF exports, stage weights, complete BO coverage and workspace summaries.
The private QA runner disables cloud/email delivery and uses a localhost-only
in-memory object fixture. Go ownership retirement has separate unit verification.

TypeScript, scoped ESLint, the frontend regression suite and a production Next.js
build are also checked. Native BO/comparison/Finance/LOI PDFs have been rendered and
visually inspected. No schema migration was introduced by purchasing.

The final local cutover record will identify the frozen combined checkout, all
included remote tips, concurrent Tender overlay hashes, matching image identities,
verified backup, schema version and preservation checks. Local preview verification
is separate from a reviewed merge to `main` or production deployment.

## Local cutover verified — 5 November 2026

Native purchasing is active at `http://localhost:9092` from integration snapshot
`ff7d0fa465151bd618619c05c08924be3177989c`. All seven refreshed remote refs were
ancestors before testing. The frozen checkout includes a recorded 57-file concurrent
Tender/provider overlay; later working-tree changes are outside this tested snapshot.
See `PURCHASING_LOCAL_VERIFICATION_20261105.json` for commits, refs, overlay hashes,
image identities, schema, preservation audit and separate feature/combined checks.

Feature verification passed: 42 isolated compatibility tests with no skips, 31 Docker
HTTP checks with Go unreachable and cloud/email disabled, 238 frontend regressions,
TypeScript, scoped ESLint, affected Go suites/ownership tests, native PDF visual review
and the frozen production build. A separate standalone HTTP run also passed 31 checks.
Docker Desktop recovery interrupted one earlier rerun; that interrupted run is not
counted as a pass. The final compatibility run used a fresh host PostgreSQL database,
while the packaged Docker HTTP checks used the isolated Docker QA database.

The private backup was validated through both archive listing and full restore-stream
parsing and hashed before the matching migrator ran. Schema 101 remained clean and
unchanged. After startup, 192 table snapshots matched except the expected superadmin
role startup edit-version increment; role contents, password hashes and business data
were preserved. Both stored originals and all 22 S3 objects matched byte snapshots.
Existing encryption keys and provider approvals/controls were retained. Model-call
count remained 21 throughout cutover; this migration made no provider calls.
All nine operational services passed health checks; native session/origin guards,
retired Go purchasing HTTP routes and private renderer protection passed combined
smoke checks. Existing Go internal readers remain for unmigrated consumers.

Docker Desktop recovered without resetting volumes. The frozen Next build was
packaged with Linux dependencies from its exact lockfile after the Docker webpack
build stalled; host native modules were excluded. API/migrator/workers share matching
linux/amd64 payloads. The private backup and prior image tags remain available for
rollback. No production deployment, reviewed main merge or branch push occurred.
