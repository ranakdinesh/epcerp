# Tender register migration verification

Application source committed unchanged as `b92448a75819b53b3a5b151e5235a47230a4f748` on `codex/development`.
Verification completed 2026-11-04T15:30:15.665728+00:00. Schema **99, clean**, unchanged.
Image builds began from the integration working tree before the application commit;
that exact implementation was then committed and used for the HTTP/cutover checks.
Fingerprint of the committed application tree: `8b161880a8d7a857b871518205ebbbc55bb64b35f2db2d2afb7964a6129d67e8`.

Other upload-related changes appeared in the shared checkout after the image build.
They were left untouched, excluded from the committed-source fingerprint, and are
not covered by this migration verification.

## Native scope

Next.js owns named Tender capture, register listing/detail, metadata editing,
manager choices/reassignment and legacy owner handover under
`EPCERP_TENDER_REGISTER_OWNER=nextjs`. Matching Go HTTP writers are unregistered;
Go transaction-bound foundation/action readers remain for unmigrated consumers.
Code lives in `frontend/src/server/tenders/register`, with published Auth directory
and Inbox notice contracts. No new service or migration was introduced.

Capture atomically consumes the verified receipt and records source links, manager
history, business events, provisioning intent, addressed Inbox delivery, email intent
and the operation ledger. Metadata/assignment versions and content revision retain
their separate meanings. Reassignment cancels obsolete work/unsent mail while keeping
history. Existing participant grants remain metadata-only. Empty manager designation
configuration fails closed, including for superusers; no permissions or roles were
silently granted.

## Feature verification — isolated synthetic data

- `frontend/scripts/verify-tender-register.sh`: **13 passed, zero skipped** on
  `epcerp_auth_qa_register_20261104`, migration 99. Covers candidate policy,
  designation revocation, concurrent capture/replay, Go fingerprints, live scope,
  receipt ownership/draft binding/expiry/version/consumption, complete rollback on
  Inbox failure, participant expiry/revocation, optimistic edits, reassignment
  cancellation and legacy isolation.
- Go oracle accepts native records and assignment notices, and replays notice delivery
  without duplicate history. Existing immutable operation fingerprints match Go.
- Built Docker frontend on isolated port 59092, **Go backend deliberately unreachable**:
  HTTP login, manager choices, capture, replay/Location headers, metadata edit,
  reassignment/Inbox and named/legacy cross-origin write guards passed. Cloud and
  email were disabled. Sources were explicitly synthetic verified fixtures; this
  check does not claim to retest the independent scanner pipeline.
- Existing frontend tests: **232 passed**. TypeScript passed. ESLint: zero errors,
  four existing warnings. Optimized Docker/Next.js build passed.
- Affected Go tests passed: Tender FMS, communication Inbox, app and schema packages.

## Combined local stack smoke and preservation

All nine services healthy on the combined checkout. Local login/dashboard render,
unauthenticated native API boundaries, origin guards, private renderer authentication
and retired Go register/intake/format/TSC writers passed. The main preview was checked
read-only; synthetic mutations were confined to the QA database. Paid provider calls
and messages during verification: **zero**.

Writers were stopped before the private backup. PostgreSQL dump listing and full
restore-stream reads passed, and its SHA-256 was recorded. All 190
tables were compared: 188 table fingerprints matched exactly. The existing startup
bootstrap incremented the superadmin role edit_version once. One pre-existing Tender
draft-read quota row advanced its request window and count; no rows were added or
removed and no blocked-until/security fields changed. The strict initial comparison
halted on that counter; a targeted comparison against the verified dump explained it
before preservation was concluded. The legitimate counter update was retained.
Password hashes, role contents,
credentials and encryption keys were preserved. All 22 S3 objects
and 2 encrypted source files, including file
ownership/permissions, matched. No volumes were reset/deleted and no migration
version was forced. Schema remained 99 clean; model-call count remained zero.

Private backup and detailed manifest: `/private/tmp/epcerp-register-before-20261104T152249Z`.
Unique pre-register rollback tags are recorded privately in
`/tmp/epcerp-register-rollback-images.json`.
No merge to main, remote push or production deployment was performed.

## Remaining boundaries

**Project/estimate creation was already missing in Go.** The capture form has no
required project client and no estimate provisioner exists. Its operation therefore
remains honestly `pending / project_provider_pending` with null linked IDs. No fake
project or estimate is created. Completing that workflow is separate implementation
work; it is not described as successfully migrated project creation.

Email rows remain queued delivery intents; this migration does not send email.
The old named-record prepared source-reading contract remains compatible with Go
workers; draft cloud/manual TSC continues to publish verified originals without OCR.
Safety scanning, archive/Office conversion, retention workers, legacy document/result/
history reads, Inbox UI/acknowledgements, Projects and Tasks retain their prior owners.

## Included fetched remote tips

Every fetched tip was an ancestor before feature tests and the local cutover.

| Ref | Commit |
| --- | --- |
| `origin` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/chor/auth` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/codex/development` | `f896cf1c5cc513c7721b4354a8441c102b1947c9` |
| `origin/codex/table-aware-tender-evidence` | `7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2` |
| `origin/codex/tsc-processing-guardrails` | `17868507ffaa76221dbb8574856a819149973cac` |
| `origin/main` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/tenderFMS` | `1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` |

## Running image digests

| Service | Image digest |
| --- | --- |
| `frontend` | `sha256:c124c4a5ffe35ca1304a09b79db880f206c9665d588d3c5ef617b26fa0be7b4c` |
| `api` | `sha256:e0d85508d8b5f757acd238bedddc143da0a4864a2079a1bf078f42da71750fbe` |
| `document-worker` | `sha256:d3aeba84a7b573ae3dbecd3a53bdc87d65cb79f638be0adf5cc20dcb6e5e1eea` |
| `document-package-worker` | `sha256:d3aeba84a7b573ae3dbecd3a53bdc87d65cb79f638be0adf5cc20dcb6e5e1eea` |
| `tsc-worker` | `sha256:e0d85508d8b5f757acd238bedddc143da0a4864a2079a1bf078f42da71750fbe` |
| `tsc-reranker` | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` |
