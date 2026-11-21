# Supplier documents migration verification

Verified locally on 2026-11-04. Supplier document handling runs in Next.js inside
Master Data, alongside the existing supplier profiles and catalogue. No new service
or database was introduced. Production was not deployed.

## Ownership and structure

`frontend/src/server/master/documents` contains the HTTP adapter, validation/public
DTO, application services, reviewed SQL repository, transactional approval hooks,
and file encryption/media/storage adapter. The Master README describes these
boundaries and repeatable QA setup.

`EPCERP_VENDOR_DOCUMENTS_OWNER=nextjs` is set on local API and frontend. Go no
longer registers document list, upload, legacy-import, preview, download or archive
HTTP operations. It retains published document/evidence readers and explicit rollback
code. Finance verification/decisions, reviewer assignment/options and manual
`documents/reconcile-approvals` remain Go-owned. Native errors never retry through Go.
All four independent profile/document ownership combinations are tested.

The browser proxy checks write authority before buffering upload bytes. Native
services resolve live grants again before writes. Document, approval and task rows,
immutable histories and business events share the database transaction. Replacements
preserve previous objects and retire matching review/correction tasks. Legacy import
retains source idempotency without inventing approval submissions.

## Tested integration source

Branch: `codex/development`. Base checkpoint: `ba49f15e1e1429cdd8e934ee04567a2ecb6dc2d4` plus the
working-tree migration. The preceding vendor-profile migration was saved as that
local checkpoint; nothing was merged into main or pushed.

Tested source SHA-256: `a9c96df5b62ac3c56117285f49af529f11d489cb869fe934cf9dc8a878e0f793`.
This fingerprints tracked/untracked Go, TypeScript, SQL, JSON, YAML, shell, MJS and
Python source under backend/frontend/deployment. Documentation is excluded.
Remote tips were fetched before verification/cutover and each was checked as an
ancestor of this integration checkout:

| Remote ref | Included commit |
| --- | --- |
| `origin` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/chor/auth` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/codex/development` | `f896cf1c5cc513c7721b4354a8441c102b1947c9` |
| `origin/codex/table-aware-tender-evidence` | `7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2` |
| `origin/codex/tsc-processing-guardrails` | `17868507ffaa76221dbb8574856a819149973cac` |
| `origin/main` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/tenderFMS` | `1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` |

API/migrator, frontend and Go workers were built from this checkout. The reranker
source was unchanged and its existing local image was retained without model downloads.

| Running service | Image ID |
| --- | --- |
| api | `sha256:5097f399a07299f252ae9e7fe3627dec38c010eb992f37318517d505db0e2ed1` |
| frontend | `sha256:8b5a221b400cad69f05f9e14cf12d7e26390b1ea624ab160f62803b49d4c5779` |
| document-worker | `sha256:9b503ec43108e0d2cce9a809687a2e65cb4426d918b2602348140036fae8e822` |
| document-package-worker | `sha256:9b503ec43108e0d2cce9a809687a2e65cb4426d918b2602348140036fae8e822` |
| tsc-worker | `sha256:5097f399a07299f252ae9e7fe3627dec38c010eb992f37318517d505db0e2ed1` |
| tsc-reranker | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` |

## Feature verification

- **91 Auth/Master server tests passed**, including the actual Go parser/oracles and
  real PostgreSQL/object-storage operations against `epcerp_auth_qa_documents_20261104`
  and dedicated `epcerp-documents-qa-20261104` bucket. Test files sharing the database
  run sequentially, preserving the profile-only isolation assertions.
- Go AES-GCM ciphertext opens in Node and Node ciphertext opens in Go. Hash mismatch,
  tampering and truncated ciphertext are rejected. Both inline legacy ciphertext and
  encrypted objects are exercised without exposing storage/cipher fields in responses.
- PDF/PNG/JPEG/GIF detection matches Go on synthetic supported files; corrupt,
  disguised, unsupported and oversized files fail. The existing PDF header/trailing-EOF
  acceptance policy is retained; this is not a new full PDF validator or malware scan.
- Live read/write/import grants, Finance alternative read, inactive/revoked users,
  archived creator/Finance access, cross-supplier reads, optimistic versions,
  concurrent singleton replacement, legacy idempotency, audit/approval rollback,
  automatic/pending approvals and review/correction task retirement passed.
- **232 frontend tests passed**. TypeScript and Docker production build passed.
  Lint has no errors and four existing warnings (login/purchase images and radio ARIA).
- Go Master, Auth, architecture, project-material and Purchasing checks passed.
  The Go route test confirms native ownership retires document routes while keeping
  Finance reconciliation registered.

## Browser verification

Browser plugin not available; the frontend-testing-debugging skill's regular
Playwright fallback used installed Chrome. The isolated Docker browser target was
`http://localhost:59092`, with a QA Go API on port 58001, synthetic credentials/keys,
provider/email calls disabled and external browser requests blocked.

Flow: sign in → suppliers → document editor → upload PAN/cheque/GST → preview and
download → Go Finance reject → native replacement → archive/history.

| Check | Result |
| --- | --- |
| Intended URL, meaningful screen, visible application identity and title where supplied | Passed |
| Uploads accepted through native Next.js and readable through Go evidence/storage | Passed |
| Go legacy object upload readable through Next.js | Passed |
| Real one-page PDF rendered and downloaded byte-for-byte | Passed |
| Sandbox CSP, private/no-store, nosniff and attachment headers survive proxy | Passed |
| Go rejection creates a correction task; native replacement completes it as resubmitted | Passed, including SQL assertion |
| Old evidence retained, new evidence accepted, stale Finance fingerprint rejected | Passed |
| Archive removes active eligibility while preserving history | Passed |
| Desktop/mobile evidence, no document-level horizontal overflow or framework overlay | Passed |
| Unexpected browser/runtime errors in final run | None |

Screenshots were inspected at `/private/tmp/epcerp-documents-browser/`:
`documents-desktop.png`, `preview-desktop.png`, `history-desktop.png`, and
`suppliers-mobile.png`. Early browser runs exposed deliberately corrupted integrity
fixtures from the server tests; those fixtures are now restored after the negative
assertion, and the complete final browser run passed without filtering console errors.

Temporary QA containers and private QA environment files were removed. Synthetic
QA database/bucket fixtures remain available for review; operational mutations were
not used for browser acceptance.

## Combined-stack cutover and preservation

Local preview: `http://localhost:9092/dashboard?screen=suppliers`.
All nine services are healthy. Go readiness returns 200; all six migrated Go document
operations return 404. Finance verification, assignment and reconciliation remain
registered and protected. Login returns 200; unauthenticated browser document/profile
requests return 401. All six migrated owner flags are `nextjs` on API and frontend.

Application writers stopped before the private backup. PostgreSQL archive passed
both table-of-contents inspection and complete SQL decoding. The matching migrator
ran before services restarted. Schema **98 remains clean**: no schema change, reset
or forced migration version was introduced.

Fingerprints matched across **35 preservation groups**, including all supplier
profile/document/approval/task/assignment/verification tables, catalogue/directories,
business events, project materials and user password hashes. The existing operational
bucket's **22 object keys/sizes/ETags/timestamps** also matched before/after.
Database and object-storage volumes, credentials, signing/encryption keys and provider
approval controls were preserved. No password change is required. Existing files were
not relocated or re-encrypted. No real messages, paid provider jobs or private-document
cloud uploads were submitted. Combined-stack smoke checks do not establish full
acceptance of every unrelated feature; the user's existing operational session remains
available for their own acceptance checks.

## Limits and release review

Object storage and PostgreSQL are separate systems. As in Go, a rolled-back database
write can leave an encrypted orphan object for offline reconciliation; existing objects
are never deleted on rollback or uncertain commit. Storage connection, request and
idle socket timeouts are enforced.

The production dependency audit still flags the unchanged `next@16.3.4` (critical)
and `source-map-js@1.2.1` (high). These pre-existing findings require separate release
review before production promotion. The added S3 SDK has no findings in the final
production audit; Sharp was explicitly pinned to the patched 0.35.5 rather than the
previous transitive 0.35.4. The platform-complete lockfile passed Linux `npm ci`.

## Private backup and rollback

Verified final backup and manifest: `/private/tmp/epcerp-documents-before-20261104T103312Z`. These contain private configuration
and database material and must remain local. The archive/config files are private;
object-storage volumes were preserved and their metadata snapshot was verified.

Rollback images are tagged `before-vendor-documents-20261104`, with exact service
mapping in the backup's `rollback.compose.json`. Restore those images and set
`EPCERP_VENDOR_DOCUMENTS_OWNER=go` on API/frontend; keep Auth, Clients, references,
catalogue and vendor profiles at `nextjs`. Preserve the current database, volumes,
credentials and keys. A database restore is unnecessary. Main/release review and
production deployment remain separate steps.
