# Projects migration verification

Feature commit: `dc8658e135263cf5b6e69b9afb0b8e5f89f44932` on `codex/development`.
Final local smoke verification: 2026-11-05T00:58:50.629189+00:00.
Combined source SHA-256: `a32a50f526287863eaf6f84c27d7d2d54f62a615e4fd1a015239500b0cb502d5`.
Schema **101, clean and unchanged**. No Projects schema migration was added.

## Native scope

Next.js now owns Project Master list/search/detail, creation, assignment options,
manager handover/history, active-draft correction/history, metadata options and
schedule previews under `EPCERP_PROJECTS_OWNER=nextjs`. Go removes those HTTP
routes under the same flag. Native failures never fall back to Go.

Creation atomically initializes the project, manager history, procurement workspace,
owner history and journal events. Existing permissions, scoped visibility, optimistic
versions, Go-compatible audit snapshots, inactive-client retention and working-calendar
calculations remain. No new lifecycle transition endpoint was introduced.

Code is in `frontend/src/server/projects`, separated into validation, schedule,
repository, access, service and HTTP files. Initial procurement registration uses
`frontend/src/server/purchasing/project-registration.ts`. The module README explains
its boundaries. Purchasing workflow, responsibilities/team, financial actions and
materials remain with their current Go modules. Published Go project readers and
calendar recalculation remain available for these consumers.

## Feature verification: synthetic data only

- **14 passed, zero skipped** on the working checkout and frozen integration
  snapshot. Original Go HTTP services/readers served as the oracle. Bidirectional
  creation/correction/history, rollback, permissions/revocation, literal searches,
  calendar preview/recalculation, stale versions and concurrent handovers passed.
- Docker HTTP login, create/list/read/edit, options, schedule preview, handover/history,
  stale-write and origin rejection passed with Go deliberately unreachable. Retained
  nested routes used the unavailable Go fallback. Cloud and email were disabled.
- Frozen snapshot: **237 frontend tests passed**, TypeScript passed, ESLint zero
  errors/four existing warnings, optimized Next.js Docker build passed. Affected Go
  Projects, calendar, purchasing, document package and app checks passed.

## Combined integration and runtime

All 7 fetched remote tips below were ancestors of the feature commit.
API/migrator, frontend and workers were built from one frozen integration checkout.
It includes 47 concurrent Tender/upload/provider files,
including schema-101 definitions already used locally. Exact hashes are in the
private source manifest. Those edits remain untouched and excluded from the Projects
commit. Later edits outside the snapshot are not claimed as tested or deployed.

Dedicated `projects-dc8658e` tags avoid overwriting shared rolling tags. All **nine
services are healthy**. Read-only combined smoke checks passed for native Projects
session/origin guards, retired Go master routes, retained Go responsibilities,
earlier native Inbox/Tasks/Tender/Master routing and private renderer authentication.
The matching migrator ran before startup; schema stayed 101 clean.

## Preservation and activity

Writers were stopped after active-work checks. A private PostgreSQL backup passed
dump listing, full restore-stream reading and SHA-256 verification. Exact running
rollback image indexes were preserved after matching platform digests through
unstarted probe containers.

All **192 existing tables** were fingerprinted before/after cutover. Only the
expected startup superadmin role edit_version increment changed; no request counters
changed. Business data, password hashes, permissions and credentials matched. All
**22 object-storage objects** and **2 encrypted source files**, including ownership and
permissions, matched. Actual runtime provider keys, settings and budgets were
retained across application services and checked. Users keep their passwords.

The model-call count remained 1 throughout the protected cutover comparison.
After the preview resumed, it increased to 2 and Tender processing was active.
That concurrent activity is separate from Projects verification: these checks used
synthetic QA data with cloud/email disabled and sent no operational LLM commands or
real messages. Existing provider approvals and budgets were left in place.

No volume removal, database reset, migration-version forcing, merge to main, remote
push or production deployment occurred. Private backup, manifests and check logs:
`/private/tmp/epcerp-projects-before-20261105T005736Z`.
QA container and temporary QA credential files were removed; isolated QA database
and dedicated images retained.

## Included remote tips

| Ref | Commit |
| --- | --- |
| `origin` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/chor/auth` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/codex/development` | `f896cf1c5cc513c7721b4354a8441c102b1947c9` |
| `origin/codex/table-aware-tender-evidence` | `7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2` |
| `origin/codex/tsc-processing-guardrails` | `17868507ffaa76221dbb8574856a819149973cac` |
| `origin/main` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/tenderFMS` | `1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` |

## Verified running images

| Service | Platform image ID | Image reference |
| --- | --- | --- |
| `db` | `sha256:b0f9560a2de083e2cc7382e75f808c7381a32852a7ec49117deedb300e552b24` | `postgres:17-alpine` |
| `api` | `sha256:0ab0d2015f17ecd45fe8e573874418766dbac331f3e69a59b41ce37737aaae00` | `epcerp-api:projects-dc8658e` |
| `storage` | `sha256:e750697a12d1f8c19628bf57c0bf30a29ddc0fc489b42de39074d83804430527` | `epcerp-storage:local` |
| `frontend` | `sha256:860605e2625e6cdb7742820201df0c2b0f8be4624445edfca35eda42a477774c` | `epcerp-frontend:projects-dc8658e` |
| `document-worker` | `sha256:be54c69686751f7b1585745bc479bc366a3418a5902e42f021b8dce5b5965ae2` | `epcerp-document-worker:projects-dc8658e` |
| `document-package-worker` | `sha256:be54c69686751f7b1585745bc479bc366a3418a5902e42f021b8dce5b5965ae2` | `epcerp-document-worker:projects-dc8658e` |
| `clamav` | `sha256:3bba6dd5ac6663d55b80f86a3126e146612d93b593cc2617188d13f637d91260` | `clamav/clamav:1.4@sha256:3bba6dd5ac6663d55b80f86a3126e146612d93b593cc2617188d13f637d91260` |
| `tsc-worker` | `sha256:0ab0d2015f17ecd45fe8e573874418766dbac331f3e69a59b41ce37737aaae00` | `epcerp-api:projects-dc8658e` |
| `tsc-reranker` | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` | `epcerp-tsc-reranker:local` |
