# Supplier Finance migration verification

Verified locally on 2026-11-04. Supplier Finance verification, reviewer assignment
and approval reconciliation now run inside the existing Next.js Master Data module.
No new service, database or schema migration was introduced. Production was not deployed.

## Code and ownership

`frontend/src/server/master/verification` separates domain rules/fingerprints,
application services, SQL/evidence/history and HTTP adapters. Document approval hooks
remain in `master/documents/approval.ts`. Auth publishes live authorization and the
limited assignment directory; credential fields stay inside Auth. The published
Inbox writer at `server/inbox/supplier-review.ts` writes addressed local assignment
notices, notice history and the corresponding business event in the caller transaction.

`EPCERP_VENDOR_VERIFICATION_OWNER=nextjs` is set on API/frontend locally. Go retires
verification GET/POST, reviewer assignment GET/PUT/options and reconciliation POST.
Native failures never retry through Go. All eight combinations of supplier profile,
document and Finance ownership are tested. Go's transaction-bound purchasing gate,
Inbox and My tasks routes/readers remain available to unmigrated consumers.

Finance read or supplier read grants status access; supplier-only reads hide review
details. Review requires Finance read/write. Assignment/options require Finance
read/assign; eligible reviewers require Finance view/read/write. Directory eligibility
and first-password readiness remain separate as in Go, while interactive operations
still require completed credentials and live grants. Reconciliation requires document
write/import and Finance auto-approve.

The same supplier lock serializes document changes, decisions, assignments and Go
purchasing checks. The fingerprint preserves Go's sorted struct JSON and SHA-256,
including supplier version/lifecycle/GST and each active document's ID/type/hash,
version and actual previewability. Approval records are projected independently, so
unrelated uploads retain valid decisions. Partial approvals are progress; required
and all active documents must be readable and accepted before an order is allowed.
Stale review versions/fingerprints are rejected. Review/assignment history, document
approvals, review/correction tasks, business events and addressed notices are atomic.

## Tested integration source

Branch: `codex/development`. Base checkpoint: `515370fbc2262eb3188b66e84a51a89b17f1a2aa` plus the
working-tree Finance migration. The preceding verified document migration was saved
as this local checkpoint. Nothing was merged into main or pushed.

Tested source SHA-256: `3de2f53f0c179d8a8fd00d0381c058d81e05aaffe089988683fb0eb7d37dfb21`.
The manifest fingerprints tracked/untracked Go, TS/TSX, SQL, JSON, YAML, shell, MJS
and Python under backend/frontend/deployment; documentation is excluded. Remote tips
were fetched before testing and cutover, and all intended tips are ancestors:

| Remote ref | Included commit |
| --- | --- |
| `origin` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/chor/auth` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/codex/development` | `f896cf1c5cc513c7721b4354a8441c102b1947c9` |
| `origin/codex/table-aware-tender-evidence` | `7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2` |
| `origin/codex/tsc-processing-guardrails` | `17868507ffaa76221dbb8574856a819149973cac` |
| `origin/main` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/tenderFMS` | `1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` |

API/migrator, frontend and Go workers were built from this checkout. Unchanged
reranker source used the retained local image without model downloads.

| Running service | Image ID |
| --- | --- |
| api | `sha256:537ca949b21f42658102cf003821096a6adfbb5ce1964fe07445e8298bf9a17e` |
| frontend | `sha256:b6a912dd39d0a071be536a0d5716ebc88240ed53fce500ab5ab9923b97fd6980` |
| document-worker | `sha256:d8014df1d449390bf5168d97876e1005b91597dffa529a20a22be7a177625ff0` |
| document-package-worker | `sha256:d8014df1d449390bf5168d97876e1005b91597dffa529a20a22be7a177625ff0` |
| tsc-worker | `sha256:537ca949b21f42658102cf003821096a6adfbb5ce1964fe07445e8298bf9a17e` |
| tsc-reranker | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` |

## Feature-specific verification

- **105 Auth/Master server tests passed**, including 43 Finance fingerprint/status/
  validation fixtures compared with actual Go rules, all eight supplier ownership
  combinations and real SQL/storage checks. The isolated database was
  `epcerp_auth_qa_finance_20261104`; document tests used the dedicated
  `epcerp-documents-qa-finance-20261104` bucket and synthetic keys.
- The actual Go purchase-order gate ran inside QA transactions after native writes.
  Fingerprint, state, quotation eligibility and purchase-order eligibility matched
  Next.js after partial/missing approvals, rejection/replacement, extra documents,
  suspension/reactivation and unreadable legacy evidence.
- Permission separation, pending first-password accounts, eligible reviewer options,
  immutable history, addressed notices/events, pending-task reassignment, revocation,
  correction tasks/resubmission, stale and concurrent reviews, unknown/oversized
  bodies and cross-supplier/archived document decisions were checked.
- Injected approval-history and inbox failures rolled back the review/assignment,
  task changes, histories and journals together. Repeated decisions retain task
  idempotency. Reconciliation preserves existing decisions, retires older singleton
  files, leaves unreadable files unapproved and remains repeatable.
- **232 frontend tests passed.** TypeScript and Linux Docker production build passed.
  Lint has no errors and four existing warnings (login/purchase images and radio ARIA).
- Go Master, Auth, architecture, Purchasing and Communication Inbox checks passed.
  Route tests cover independent Finance retirement and retained document ownership.

Canonical regression command is `bash frontend/scripts/verify-master.sh`, with a
private environment pointing to a guarded `epcerp_auth_qa_` database and dedicated
local QA storage bucket. It never targets the operational database. See the Master
README for environment and ownership details.

## Browser verification

Browser plugin not available; the frontend-testing-debugging skill's Playwright
fallback used installed Chrome against QA frontend port 59092 and Go API port 58001.
The QA API deliberately retained Go Finance handlers as the compatibility reference;
the tested frontend used native ownership. Separate synthetic admin, uploader and
reviewer accounts were used, with external browser requests blocked and provider/
email delivery disabled.

Flow: admin assigns reviewer → uploader submits pending PAN/cheque/GST → reviewer
approves/rejects → uploader replaces rejected cheque → reviewer approves replacement
→ operator reconciles existing records.

| Check | Result |
| --- | --- |
| Intended suppliers URL, meaningful app identity/content, no error overlay | Passed |
| Native assignment visible through Go Inbox and pending tasks through Go My tasks | Passed |
| Separate uploader/reviewer permissions and native approve/reject controls | Passed |
| Rejection creates correction; replacement completes it as resubmitted | Passed, including SQL assertion |
| Stale Finance command rejected after replacement | Passed |
| All documents accepted enables purchasing; Go status equals native status exactly | Passed |
| Reconciliation preserves existing decisions | Passed |
| Real PDF preview renders for Finance reviewer | Passed |
| Desktop/mobile screenshots and no document-level horizontal overflow | Passed |
| Unexpected browser/console errors in final complete run | None |

An early QA run found the synthetic roles lacked the existing Inbox/My tasks read
grants. Those fixture roles were corrected; application authorization was retained.
The complete final run passed without filtering errors.

Screenshots inspected in `/private/tmp/epcerp-finance-browser/`:
`assignment-desktop.png`, `pending-desktop.png`, `rejected-desktop.png`,
`approved-desktop.png`, `preview-desktop.png` and `approved-mobile.png`.
Temporary QA containers/private environment files were removed afterward; synthetic
QA database/bucket fixtures remain available for review.

## Combined-stack activation and preservation

Local preview: `http://localhost:9092/dashboard?screen=suppliers`.
All nine services are healthy and all seven native owner flags match on API/frontend.
The six retired Go Finance operations return 404. Go readiness returns 200; Go Inbox
and My tasks remain registered and protected. Login returns 200; unauthenticated
browser supplier/verification/assignment routes return 401.

Writers stopped before a private PostgreSQL archive and configuration backup. The
archive passed table-of-contents inspection and complete SQL decoding. The matching
migrator ran before startup. Schema **98 remains clean**, with no reset or forced
migration version.

Before/after fingerprints matched across **37 preservation groups**: existing supplier
profiles/documents/approvals/tasks/assignments/verification and histories, Inbox and
notice history, catalogue/directories, business events, projects/materials and password
hashes. The operational bucket's **22 object keys/sizes/ETags/timestamps** also matched.
Database/storage volumes, credentials, signing/encryption keys and existing provider
controls were preserved. No password change is required. No existing files were
relocated, re-encrypted or deleted. Automated mutations used QA data only.

No real messages, paid provider jobs or private-document cloud uploads were submitted.
Combined-stack smoke checks do not establish acceptance of every unrelated feature;
the user's existing operational session remains available for their own acceptance.

## Limits and rollback

The migration preserves Go's existing PDF header/trailing-EOF policy and image
inspection limits; it does not add a new full PDF validator or malware scanner.
The preceding document migration's encrypted-orphan policy after uncertain commits
is unchanged. Existing Next.js/source-map-js dependency audit findings recorded in
the document verification report remain separate release-review work before production.

Verified private backup/manifest: `/private/tmp/epcerp-finance-before-20261104T105002Z`. Keep this private material local.
Rollback image mapping is in `rollback.compose.json`; retained tags end in
`before-vendor-finance-20261104`. Restore those images and set only
`EPCERP_VENDOR_VERIFICATION_OWNER=go` on both API and frontend, keeping the other six
owners at `nextjs`. Preserve the current database, volumes, credentials and keys;
a database restore is unnecessary. Main/release review and production deployment
remain separate steps.
