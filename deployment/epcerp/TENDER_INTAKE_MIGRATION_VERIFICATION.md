# Native Tender intake and format migration — local verification

Application commit `088421610effff6d2546be711580316ad25460bb` on `codex/development`.
Verified 2026-11-04. Source fingerprint:
`93c5a26f319b4bb5c40d5a152ee8bd31cd189173c04bcc4e5b4ebcbc98f48f6e`.

## Delivered scope

Next.js owns draft creation/list/detail, owner-scoped batch attachment, receipt
creation/upload/sealing/original downloads, verified source publication, and TSC
format/question administration with Word downloads and PDF previews. These sit in
the existing Tender module, alongside the cloud/manual TSC flow.

Go no longer registers the replaced draft/receipt/format endpoints under the native
intake owner. Old local OCR upload/retry commands are retired and their legacy UI is
read-only. Existing original documents, result JSON, histories and TSC exports remain
retained. Native publication no longer creates unused local OCR/readings records.

The existing isolated Go safety worker still scans files, validates archives and
normalizes Office files. Its Go draft publisher is disabled under native ownership.
A private JavaScript/LibreOffice renderer runs in the existing document-worker
container, adding no service. It receives signed application-generated Word bytes,
shares the existing CPU-slot lock, caps resources, and cleans temporary processes/files.
Word/PDF previews use the same Word bytes and preserve the reviewed template package.
The frontend uses the existing private source volume as UID/GID 10001.

**Remaining Go application work:** named tender capture/register, manager assignment
and ownership, project provisioning and its notifications; then the other previously
identified application modules. Safety/parsing workers and the owned migrator are
also still Go components. This report does not claim the whole backend is migrated.

See [module structure and deployment](../../frontend/src/server/tenders/intake/README.md).

## Isolated feature checks

- 13 native intake/format checks passed: permissions, cross-owner denial even for
  superadmins, idempotent creates/retention, raw upload claims/replay/version fences,
  checksums, Go-compatible fingerprints/framing in both directions, tamper rejection,
  verification gating, atomic publication without OCR jobs, format identity rules,
  optimistic versions, duplicate rollback, audit events and frozen Word question order.
- Existing regressions: **105 Auth/Master**, **16 cloud/manual TSC**, and **232 frontend**
  tests passed. Auth/Master used a separate QA bucket; TSC providers were injected fixtures.
- Focused Go Tender FMS, package/document workers, application and owned-schema tests
  passed, including retired legacy write registration. TypeScript passed. Full lint
  has no errors and four existing warnings unrelated to this batch.
- Matching API/migrator, workers and production Next.js images built from this checkout.
- Browser plugin unavailable: regular headless Chrome/Playwright used isolated port
  59092, a separate database and a separate upload volume. Browser flow passed:
  copy/save a 70-question format → actual offline PDF preview and Word download →
  create draft → encrypted PDF upload → actual ClamAV/document verification → native
  publication → byte-identical original download through both native and Go handlers →
  manual TSC correction → desktop/mobile checks. No browser runtime errors/overflow.
- Database verification for that browser draft: **0 local reading jobs**, **1 publication
  event**, **0 model calls**. The browser-rendered preview and mobile editor were inspected.
- Chromium returned an empty DevTools response body for the inline PDF; verification
  downloaded the same endpoint through Playwright's authenticated request context and
  checked its PDF bytes, while the actual UI rendered the first page of the five-page document successfully.

Cloud approval was disabled for the intake browser fixture. No live tender quality
benchmark, paid provider call, real message or private-document cloud upload occurred.
The requested live tender trial remains deferred.

Reproduce native checks with `bash frontend/scripts/verify-tender-intake.sh` against
an isolated `epcerp_auth_qa_intake_*` database. It refuses the operational database,
checks template synchronization and builds its synthetic Go compatibility oracle.

## Combined local Docker smoke and preservation

[Tender FMS preview](http://localhost:9092/dashboard?screen=tender-fms).
All nine services healthy. Schema **99 clean and unchanged**; this batch needs no
schema migration. Native intake and TSC owner flags match across services. Smoke
checks verified session/same-origin guards, retired Go routes, retained business
routes and rejection of unsigned requests by the private renderer.

Application writers were stopped for a private verified backup and comparison.
The backup was listed/read completely with pg_restore and its SHA-256 recorded.
**190 existing tables, 22 S3 objects, 2 encrypted source objects and their file
permissions were checked.** Business data, password hashes, role permissions,
credentials, encryption keys and provider controls are unchanged. The existing Auth
bootstrap incremented the superadmin role edit_version once; all other role fields
and existing data matched. No database reset, migration-version forcing or volume
removal occurred. Operational model-call count stayed zero during verification.

Private backup and detailed checks: `/private/tmp/epcerp-intake-before-20261104T144148Z`.
Synthetic browser artifacts: `/private/tmp/epcerp-intake-browser-proof/`.
QA containers and temporary credential files are removed after verification; isolated
synthetic database/storage evidence is retained. No merge into main, push or production
deployment is part of this local cutover.

## Included remote tips

Every fetched tip below was confirmed an ancestor of the application commit before
the combined-stack test.

| Ref | Commit |
| --- | --- |
| `origin` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/chor/auth` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/codex/development` | `f896cf1c5cc513c7721b4354a8441c102b1947c9` |
| `origin/codex/table-aware-tender-evidence` | `7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2` |
| `origin/codex/tsc-processing-guardrails` | `17868507ffaa76221dbb8574856a819149973cac` |
| `origin/main` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/tenderFMS` | `1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` |

## Running images

| Service | Image digest |
| --- | --- |
| `frontend` | `sha256:85fd5bed9ce96dd5feaa612429bf9c1643497e2570daad5fdad2d4fb9e42e2ff` |
| `api` | `sha256:59dad1304b480004ca0f31f3966114027a16ff84b9ff9c883ddfb3dd7043cb8b` |
| `document-worker` | `sha256:f7c0c920e698c1b112d64e92085aef0d92adbda0fb59f971d3a8e168673ddf95` |
| `document-package-worker` | `sha256:f7c0c920e698c1b112d64e92085aef0d92adbda0fb59f971d3a8e168673ddf95` |
| `tsc-worker` | `sha256:59dad1304b480004ca0f31f3966114027a16ff84b9ff9c883ddfb3dd7043cb8b` |
| `tsc-reranker` | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` |
