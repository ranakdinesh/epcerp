# Personal Tasks migration verification

Feature commit: `f1ece4a4d9183ee950240840faf6d6aacf595032` on `codex/development`.
Final local checks: 2026-11-04T16:34:40.455854+00:00.
Combined source fingerprint: `22b290bb5b07bedb7966bea25f4e1ef5386d44d17c0451bac337f81f8938400d`.
Schema **100, clean and unchanged**. No Personal Tasks migration was added.

## Native scope

Next.js owns `GET /my-tasks` under `EPCERP_PERSONAL_TASKS_OWNER=nextjs`. Go omits
that route under the same flag and rejects invalid owner configuration. Native
errors do not fall back to Go. No additional service or generic task CRUD was added.

The existing Go dashboard is read-only. Task actions, source approvals, completion
and history remain in their owning purchasing/supplier modules. The migration
preserves the page contract: preparation rows with bounded pagination/filtering,
quotation work, and supplier document review/correction correspondence.

Live task view/read grants are required. Even superusers receive only their own
current assignments. Preparation execute/resolve permissions and current quotation
project scope are checked separately; task dashboard access grants no new source
permissions. Deadline calculations use India dates and the role-specific deadline.
The existing literal-search semantics, ordering and separate section totals remain.

Native published readers reuse catalogue decoding/eligibility and check current
client instructions (including alternatives), active holds, project status, indent
withdrawal and requirement revisions. Assigned historical work remains visible with
availability/blocker reasons. Supplier and quotation task grants are rechecked live,
including project responsibility profile membership. No documents or prices are
returned beyond the existing metadata contract.

## Feature verification — synthetic data only

- **11 passed, zero skipped**, both in the working checkout and the frozen combined
  build snapshot. Full JSON payloads were compared with an oracle using Go's original
  transaction-bound purchasing, Auth and supplier readers on isolated schema 100.
- Cases cover recipient scope, superuser isolation, doer/resolver selection,
  bounded pagination, literal wildcard and Unicode search, India midnight,
  project/client/item/unit/manufacturer inactivity, held projects, effective client
  restrictions, alternative manufacturers, active holds, withdrawn indents and
  revised requirements.
- Quotation cases cover current purchase ownership, explicit project responsibility
  grants/revocation, PM access and completed work. Supplier cases cover correction
  permission grants/revocation, filename fallback and retained completion evidence.
  Disabled roles/accounts are denied immediately. Dashboard reads leave package
  versions/steps unchanged; the module exposes no task mutation endpoint.
- **237 frontend tests passed** on the frozen snapshot. TypeScript passed. ESLint:
  zero errors, four existing warnings. Optimized Next.js/Docker build passed.
- Affected Go Personal Tasks, Purchasing, Master and app tests passed, including
  route retirement and invalid-owner tests. Combined document package/reading
  checks also passed for the worker code included in this build.
- Built frontend on isolated port 59094: real HTTP login, task listing, role/state
  and literal-search queries, actor-selector rejection, bad page-size rejection
  and GET-only boundary passed with Go deliberately unreachable. Provider calls
  and email were disabled. No paid cloud calls or real messages were sent.

## Combined checkout and local runtime

All fetched remote feature tips below were ancestors of the feature commit.
API/migrator, frontend and both document workers were built from one frozen
`codex/development` checkout. That snapshot includes 35
concurrent Tender/upload files, including the schema-100 definitions already needed
by the local preview. Their exact paths/hashes are recorded in the private source
manifest. The original concurrent edits were left untouched and were not included
in the Personal Tasks feature commit. Later edits outside that snapshot are not
claimed as tested or deployed here. Shared rolling image tags were not overwritten;
this deployment uses dedicated `tasks-f1ece4a` tags.

All **nine services are healthy**. Read-only combined smoke checks passed for
native task ownership, retired Go task routes, authenticated/origin boundaries,
prior native Inbox/Tender/Master routing and the private renderer's authentication.
The matching migrator was run before starting services; schema remained 100 clean.

## Preservation

Active provider/upload work was checked before stopping writers. The existing
PostgreSQL database was backed up privately; dump listing, full restore-stream read
and SHA-256 verification passed. Exact running rollback image indexes were preserved
under unique tags after checking platform digests through unstarted probe containers.

All **191 existing tables** were fingerprinted before and after.
Business data, task/source history, password hashes, permissions and keys were
preserved. Only the existing startup bootstrap's expected superadmin role edit_version
increment and an audited Tender read-quota counter changed. Counter changes were
limited to its request count; no Auth security fields or counter identities changed.
All **22 object-storage objects** and **2
encrypted source files**, including file ownership/permissions, matched.
Existing explicit provider settings and budgets were compared and retained.
The existing model-call count (1) did not increase during
these checks. No schema downgrade/reset, migration-version forcing or volume removal
occurred. Users do not need to change their passwords.

Private verified backup, detailed source/runtime manifest, before/after evidence
and check logs: `/private/tmp/epcerp-tasks-before-20261104T163239Z`.
QA containers and temporary credential files are removed after verification;
isolated QA databases and dedicated images are retained. No merge to main, remote
push or production deployment is part of this change.

## Remaining Go boundaries

Purchasing/supplier task actions and their published writer contracts remain with
their source modules. Projects, purchasing workflow editors and financial decisions
are not migrated by this dashboard change. Native task readers make no provider,
email or external messaging calls.

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
| `api` | `sha256:b256cddc386abcd00eb8a33c5e82901e5a19e76f94447a62a3d2f62bd086f96d` | `epcerp-api:tasks-f1ece4a` |
| `storage` | `sha256:e750697a12d1f8c19628bf57c0bf30a29ddc0fc489b42de39074d83804430527` | `epcerp-storage:local` |
| `frontend` | `sha256:e6db7d310e1b621ea5eae59a70919b874caf7aa8eb8d2755942ef920ed65c861` | `epcerp-frontend:tasks-f1ece4a` |
| `document-worker` | `sha256:26dbc2e9fa311b005dd63ab9622534552ef73243ada6e56d57d016c96e44fd8f` | `epcerp-document-worker:tasks-f1ece4a` |
| `document-package-worker` | `sha256:26dbc2e9fa311b005dd63ab9622534552ef73243ada6e56d57d016c96e44fd8f` | `epcerp-document-worker:tasks-f1ece4a` |
| `clamav` | `sha256:3bba6dd5ac6663d55b80f86a3126e146612d93b593cc2617188d13f637d91260` | `clamav/clamav:1.4@sha256:3bba6dd5ac6663d55b80f86a3126e146612d93b593cc2617188d13f637d91260` |
| `tsc-worker` | `sha256:b256cddc386abcd00eb8a33c5e82901e5a19e76f94447a62a3d2f62bd086f96d` | `epcerp-api:tasks-f1ece4a` |
| `tsc-reranker` | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` | `epcerp-tsc-reranker:local` |
