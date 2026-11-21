# Master Data / Vendor profiles: local verification, 4 November 2026

## Scope and behavior

Supplier list, create, profile/lifecycle update and legacy import now execute in
Next.js under the existing Master Data module. Each area separates domain rules,
application services, repositories and the HTTP adapter. No additional application,
Docker service, database or schema migration was introduced.

`EPCERP_VENDOR_PROFILES_OWNER=nextjs` is set on both local API and frontend. Go
unregisters just the four profile route patterns. Nested document, reviewer and
Finance verification endpoints remain Go-owned, with the published supplier
directory reading the same rows. Native failures never retry through Go. See
[the module guide](../../frontend/src/server/master/README.md).

Existing IDs, immutable codes, creation metadata, source provenance, permissions,
versions and history are preserved. Updates require the current version and a
reason. The supplier change snapshot and zero-target business event commit in the
same transaction as the profile. Legacy adoption retains Go's idempotence/conflict
rules and source-time precision. Payment terms retain Go's exact decimal arithmetic.

Qualification is independent of document approval. Go continues to compute its
evidence fingerprint and readiness rules from the current supplier version,
activity, qualification and GST information. Native profile changes invalidate
stale Finance commands; qualification alone cannot bypass required evidence.

## Source actually tested

Integration branch: `codex/development`, base checkpoint
`1ef042f5e44e7820a9c0e284c43f0303dde55ecc`, plus this vendor-profile migration's
local working-tree changes. That checkpoint saves the preceding verified catalogue
migration. Nothing was pushed or merged into `main`.

Remotes were refreshed and every branch tip confirmed as an ancestor before
building: `main`/`chor/auth` at `7563358`, remote development at `f896cf1`,
`tenderFMS` at `1c7c4f6`, table evidence at `7acaa51`, and TSC guardrails at
`1786850`. The private activation manifest records full commits and image IDs.

Verified application/test/deployment source SHA256:
`7d00fa15f3fafd13edd4ea1e71fa3e2fd536a7590c50f9a3b2c485d8eb49d541`.
API, migrator, frontend and workers were built from this integration checkout.
API/document workers used the existing verified Linux runtimes; reranker source
was unchanged. The native repositories use reviewed SQL, with a synchronization
check; requests do not invoke Go subprocesses.

## Profile-specific verification

- **75 server checks passed**, with no failures or skips, against the isolated
  `epcerp_auth_qa_vendors_20261104` database. This includes the existing Auth,
  Clients, reference and catalogue regressions. Run
  `bash frontend/scripts/verify-master.sh` with a guarded QA database URL.
- **106 supplier fixtures** compare parsing, normalization, legacy import and data
  fingerprints against actual Go code. These include Unicode character/byte
  limits, unknown fields, nulls, qualification, reviewed catalogue-link constraints,
  payment types, exponent limits, exact tolerance boundaries and invalid timestamps.
- Real SQL checks cover supplier/Finance-only read grants, denial of unauthorized
  writes/imports, immediate revocation, create/edit/lifecycle, immutable code,
  optimistic versions, simultaneous edits, creation metadata and import provenance.
- Failure constraints prove that supplier-history failure rolls back both creates
  and updates, and event-journal failure rolls back profile/history creation. They
  were installed/removed only inside the guarded QA database. Existing immutable
  history triggers reject attempts to rewrite supplier changes.
- Profile tests also check that document and verification evidence tables remain
  unchanged. Qualification/onboarding flags do not create a Finance verification.
  Streamed profile payloads above 128 KiB fail before parsing.
- Go Master, Auth, architecture, project-items and purchasing regressions passed.
  **232 frontend tests passed**. TypeScript passed; lint had zero errors and the
  same four existing warnings. The final Linux frontend image built successfully.

## Browser and Go-consumer checks

Environment: isolated Docker frontend at `http://localhost:59092`, Chrome through
Playwright, desktop 1440×1000 and mobile 390×844. The Browser plugin was unavailable.
Synthetic credentials/data were used; providers and email were disabled.

The tested flow was login → supplier creation with payment/contact/indent terms
→ Go document step → profile edit → stale Finance rejection → qualification
approval → deactivate/reactivate → suspend/restore qualification → detail view.

| Check | Result |
| --- | --- |
| Intended URL, title where supplied and visible application identity | Passed |
| Meaningful content and no framework error overlay | Passed |
| Native create/edit/lifecycle controls persist the expected state | Passed |
| Immutable ID/code/creation metadata and increasing versions | Passed |
| Go document list, reviewer assignment and options read the native supplier | Passed |
| Native edit changes Go's fingerprint; old review command returns 409 | Passed |
| Approval/activation does not bypass missing PAN, cheque or GST evidence | Passed |
| Inactivity/suspension prevents quotation eligibility in Go | Passed |
| Unexpected browser/console errors | None |
| Inspected screenshots and document-level mobile overflow | Passed |

Screenshots were inspected locally at
`/private/tmp/epcerp-vendor-profiles-browser/suppliers-desktop.png`,
`/private/tmp/epcerp-vendor-profiles-browser/supplier-details-desktop.png`,
`/private/tmp/epcerp-vendor-profiles-browser/go-documents-desktop.png`, and
`/private/tmp/epcerp-vendor-profiles-browser/suppliers-mobile.png`.

The temporary QA containers and private environment files were removed. Synthetic
database fixtures were retained for review. Full document upload/download and
approval migration remains a subsequent slice; this browser run exercised their
read/authority boundary rather than submitting an approval or uploading a file.

## Combined-stack activation and preservation

The preview at `http://localhost:9092/dashboard?screen=suppliers` now uses native
vendor profiles, Auth, Clients, references and catalogue operations. **All nine
services are healthy.** Go readiness returns 200; retired profile/Master endpoints
return 404. Go document, verification, assignment and reviewer-option routes remain
registered and protected. The login page returns 200 and unauthenticated profile
and nested browser routes return 401.

Application writers were stopped before a private PostgreSQL archive was taken.
The backup passed table-of-contents inspection and complete SQL decoding. The
matching migrator ran before startup. **Migration 98 remains clean**; there was no
schema upgrade, database reset or forced migration version.

Before/after fingerprints matched across **35 preservation groups**: supplier rows,
all 11 supplier profile/document/approval/task/assignment/verification tables,
catalogue and directory records, business events, project materials and user
password hashes. Database/object-storage volumes, credentials, signing/encryption
keys and existing provider controls were preserved. No password change is required.

No real messages, paid provider jobs or private-document uploads were submitted.
Production was not deployed. These combined-stack smoke checks do not establish
acceptance of every other feature. Authenticated operational acceptance with the
user's current session remains a user check; automated mutations used only QA data.

## Private backup and rollback

Verified backup and manifest:
`/private/tmp/epcerp-vendor-profiles-before-20261104T100227Z`.
This contains private database/configuration material and must stay local.

Use the recorded prior images and set `EPCERP_VENDOR_PROFILES_OWNER=go` on API and
frontend for profile rollback. Keep Auth, Clients, references and catalogue at
`nextjs`. Document and Finance ownership stays Go throughout; restoring an older
database over newer writes is unnecessary. Main/release review and production
deployment remain separate steps.
