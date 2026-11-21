# Native Inbox migration verification

Feature commit: `d7bb2d562bbc5d24752f835179fa615aa9ddb74e` on `codex/development`.
Committed feature source fingerprint: `c0ea07fea2a03c3e04a40974f26289532254027003482291abd21d485b3744da`.
Combined-stack observation: 2026-11-04T16:10:03.496532+00:00.

## Scope

Next.js owns personal Inbox list/detail, mark-read and acknowledgement endpoints
under `EPCERP_INBOX_OWNER=nextjs`. Go omits those HTTP routes and fails startup for
an invalid owner. Browser receipt writes require the same origin. Native failures
never fall back to Go. No new service or Inbox-owned database migration is required.

All four notice kinds are supported: Director decisions, quotations, supplier
reviews and Tender assignments. Notices remain scoped to their original recipient,
even for a superuser. Permissions and account state are checked live; write receipts
require both Inbox read and write permissions.

Receipt changes preserve microsecond precision, advance the optimistic version and
append immutable before/after history plus the correlated business event in one
transaction. Opening a notice does not mark it read. Repeating a completed action
with the current version is a no-op; a stale version returns 409. Acknowledgement
sets read time if needed and never rewrites existing receipt times.

Superseded assignment correspondence remains acknowledgeable by its original
recipient, matching Go. A receipt cannot revive/approve/complete the source work.
Source modules continue to enforce their own current-assignment permissions.

## Feature checks — isolated data

- **13 passed, zero skipped** using `frontend/scripts/verify-inbox.sh` and the clean
  committed development checkout on isolated schema 99. Full Go HTTP/Next.js DTO
  and history parity, all notice kinds, recipient isolation, live revocation,
  filters, timestamp precision, no-op/stale/concurrent updates, rollback on history
  failure, and obsolete assignment correspondence were checked. The Go oracle
  uses UTC, matching Docker.
- Go receipt writes are read back by Next.js and native receipt writes by Go.
  The history and business journal retain matching source correlation/scope.
- Clean-checkout frontend tests: **233 passed**. TypeScript passed. ESLint: zero
  errors, four existing warnings. Optimized Docker build passed.
- Go Inbox, Tender FMS, app and schema checks passed. New tests verify Go Inbox
  routes are absent under native ownership and invalid owners fail closed.
- Dedicated schema-99 image: actual HTTP login, listing/detail, read/acknowledge,
  no-op/stale handling and cross-origin rejection passed with Go unreachable.
  Synthetic credentials/documents only; cloud and email disabled.

## Combined local runtime — schema 100

Concurrent Tender document work advanced the local schema to **100, clean**, while
Inbox verification was in progress. The Inbox deployment preflight detected this
and aborted **before stopping writers, backing up, migrating or replacing services**.
No schema downgrade, version forcing or volume reset was attempted.

That concurrent combined-stack deployment already included native Inbox ownership.
Its **exact running frontend image** was subsequently started on an isolated QA
copy, verified by matching Docker image IDs, and tested through HTTP against QA
schema 100. Listing/detail, read/acknowledge, replay/no-op, stale version and origin
checks passed with Go deliberately unreachable and cloud/email disabled. This is
distinct from claiming that every concurrent Tender change was feature-tested here.

All **nine** live services were healthy. Read-only smoke checks confirmed native
Inbox ownership, Go Inbox route retirement, session/origin boundaries, prior native
Tender/Master ownership and private renderer authentication. Live schema was 100
clean. Verification made no operational Inbox mutations, provider calls or messages.

Inbox introduced no password, permission, credential or encryption-key changes and
required no user password reset. Test writes were confined to isolated QA databases.
The concurrent Tender deployment's schema/data verification belongs to that task;
this report does not claim a separate full-table backup audit for its changes.
Uncommitted Tender/upload work was left untouched and was not included in the Inbox
feature commit. No merge to main, remote push or production deployment was performed.

Evidence and logs: `/private/tmp/epcerp-inbox-verification-20261104T161002Z`.
QA containers and temporary credential files are removed after verification;
isolated synthetic QA databases and dedicated test images are retained.

## Remaining Go boundaries

Transaction-bound Finance/quotation delivery writers remain for unmigrated source
modules. Existing native supplier/Tender delivery writers continue unchanged.
Migrating Inbox receipts does not migrate Personal Tasks, Finance approvals,
Projects or their business decisions. No external email sender was added.

## Included fetched remote tips

Every fetched tip was confirmed an ancestor of the feature commit before verification.

| Ref | Commit |
| --- | --- |
| `origin` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/chor/auth` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/codex/development` | `f896cf1c5cc513c7721b4354a8441c102b1947c9` |
| `origin/codex/table-aware-tender-evidence` | `7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2` |
| `origin/codex/tsc-processing-guardrails` | `17868507ffaa76221dbb8574856a819149973cac` |
| `origin/main` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/tenderFMS` | `1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` |

## Observed running images

These are the actual combined-stack runtime IDs, including concurrent Tender work;
they are not presented as images built solely from the Inbox feature commit.

| Service | Image ID |
| --- | --- |
| `db` | `sha256:b0f9560a2de083e2cc7382e75f808c7381a32852a7ec49117deedb300e552b24` |
| `api` | `sha256:9037c2a8c33c61261a6dd7ce3df2b59d1df3fd365457644a84a46151af53d236` |
| `storage` | `sha256:e750697a12d1f8c19628bf57c0bf30a29ddc0fc489b42de39074d83804430527` |
| `frontend` | `sha256:0f0de76f7d3484647aea790731f60e1277bcb0d8137f49a77f75e5aff00a281b` |
| `document-worker` | `sha256:78c51c9b479fad32a237b6ae635b17f4f4fb0108b736a9cc016733193b38cee3` |
| `document-package-worker` | `sha256:78c51c9b479fad32a237b6ae635b17f4f4fb0108b736a9cc016733193b38cee3` |
| `clamav` | `sha256:3bba6dd5ac6663d55b80f86a3126e146612d93b593cc2617188d13f637d91260` |
| `tsc-worker` | `sha256:9037c2a8c33c61261a6dd7ce3df2b59d1df3fd365457644a84a46151af53d236` |
| `tsc-reranker` | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` |
