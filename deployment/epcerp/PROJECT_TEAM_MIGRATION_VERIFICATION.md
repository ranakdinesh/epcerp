# Project Team and Responsibilities migration verification

Feature commit: `8af4276` on `codex/development`.
Final integration commit: `36ffe1daa9600fd0155e8996c8a787a13578b463` (API environment correction).
Final local verification: 2026-11-05T01:21:38.128337+00:00.
Combined source SHA-256: `4109e22cf135817738908506d5f31e47426bb7870da7c1173f47f433f58b5cf4`.
Schema **101, clean and unchanged**. No schema migration was added.

## Native scope and organization

Next.js owns the existing responsibilities workspace, assignment options, creation,
revision, revocation and profile history under `EPCERP_PROJECT_TEAM_OWNER=nextjs`.
These files stay inside `frontend/src/server/projects/team`, separated into HTTP,
validation, repository and service concerns; the module README describes boundaries.
Auth publishes live role directories and memberships without exposing credentials.
Native failures never fall back to Go. Browser writes require the same origin.

Team read requires existing global eligibility plus current PM/portfolio authority
or an effective scoped profile. Management requires team.write plus current PM,
portfolio.write or superuser authority. Profiles narrow current role permissions;
they cannot grant new global authority, team management, PM/Procurement ownership
or process bypass. Active assignment requires current account/role eligibility and
an active open project. Revocation remains possible after role/account eligibility
is lost or a project is held, closed or inactive.

Project locking and optimistic profile versions serialize revisions. User and role
identities are immutable. Each save atomically writes the profile, immutable snapshots
and correlated journal event; project versions and source workflow state are unchanged.
Original wording and historical permission codes remain intact.

Go omits these HTTP routes under the matching flag. Published Go access gates,
readable-project/owner directories and work inventory remain for unmigrated purchasing,
Finance and workflow modules, reading the same native profile records. No new service
was added; purchasing/Finance workflows remain outside this migration.

## Feature checks: synthetic data only

- **17 passed, zero skipped**, including the frozen build snapshot. Original Go HTTP
  handlers and the published transaction-bound access gate served as the oracle.
  Checks cover bidirectional assignment/revision/history payloads, role directories,
  current PM/portfolio/scoped visibility, role/membership/account/profile revocation,
  forbidden global grants, financial scope, immutable identities, duplicates, stale
  versions, concurrent edits, closed/inactive projects, failed-journal rollback,
  bounded input/directories/workspaces/history and manager handover.
- Real Docker HTTP login, workspace/options, creation/revision/revocation/history,
  stale-version, permission and origin rejection passed on isolated port 59096 with
  Go deliberately unreachable. Cloud/email disabled; synthetic database only.
- Frozen application source: **237 frontend tests passed**, TypeScript passed,
  ESLint zero errors/four existing warnings, optimized frontend build passed.
  Go Project Access, Projects, Purchasing, document packages and app checks passed.
- Final deployment checking caught a missing flag in the API's shared environment.
  Commit `36ffe1d` supplies it. The corrected integration snapshot was rebuilt and
  the protected cutover rerun. Application code was unchanged by this configuration
  correction; final smoke checks confirmed native ownership and retired Go routes.

## Combined source and local preview

All 7 fetched remote tips below were ancestors before testing.
API/migrator, frontend and workers were built from one frozen integration checkout.
The snapshot includes 47 concurrent Tender/upload/provider files,
including schema-101 definitions already used locally. Their exact hashes are in the
private source manifest. These edits were preserved and excluded from the Team
commits. Later changes outside the snapshot are not claimed as tested/deployed.
Dedicated `team-36ffe1d` tags avoid overwriting shared rolling images.

All **nine services are healthy**. Read-only combined smoke checks passed for native
Team/Projects session and origin guards, retired Go team/master routes, retained Go
purchasing routing, earlier native Inbox/Tasks/Tender/Master resources and private
renderer authentication. The matching migrator ran before startup; schema stayed
101 clean. All five application services have native Team ownership enabled.

## Preservation

Active work was checked before stopping writers. The private PostgreSQL backup passed
dump listing, full restore-stream reading and SHA-256 verification. Exact prior image
indexes were preserved after matching platform digests through unstarted containers.

All **192 existing tables** were fingerprinted before/after the final cutover.
Changed tables: auth.roles, auth.login_attempts. Changes were limited to the audited expected startup
superadmin role edit_version increment and any listed read-quota counters in the
private manifest. Existing business data, profiles/history, password hashes,
permissions and credentials were preserved. All **22 object-storage objects** and
**2 encrypted source files**, including ownership and permissions, matched.

Actual TSC provider keys, approvals, settings and budgets were preserved and compared
across all five application services. Model-call count was 2 during the protected
comparison and 2 after final smoke checks. These verification checks sent no
operational LLM commands or real messages. No volume removal, database reset,
migration-version forcing, merge to main, remote push or production deployment.
Users keep their existing passwords.

Private backup, source/runtime manifests and check logs: `/private/tmp/epcerp-team-before-20261105T012034Z`.
QA container and temporary QA credential files were removed after verification;
isolated QA database and dedicated images retained.

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
| `api` | `sha256:36aacea82d9746374c075502185b14634e8267df8f255b86f6f6f50145921939` | `epcerp-api:team-36ffe1d` |
| `storage` | `sha256:e750697a12d1f8c19628bf57c0bf30a29ddc0fc489b42de39074d83804430527` | `epcerp-storage:local` |
| `frontend` | `sha256:601158b29bdf309229b6d66c2129e9ad7d536c5f0cf959cf1fcd73eab71083b0` | `epcerp-frontend:team-36ffe1d` |
| `document-worker` | `sha256:57f6533a7f7103981ff97668469d68c8c2cf823e4c4b1b51c640c52aac67e2a5` | `epcerp-document-worker:team-36ffe1d` |
| `document-package-worker` | `sha256:57f6533a7f7103981ff97668469d68c8c2cf823e4c4b1b51c640c52aac67e2a5` | `epcerp-document-worker:team-36ffe1d` |
| `clamav` | `sha256:3bba6dd5ac6663d55b80f86a3126e146612d93b593cc2617188d13f637d91260` | `clamav/clamav:1.4@sha256:3bba6dd5ac6663d55b80f86a3126e146612d93b593cc2617188d13f637d91260` |
| `tsc-worker` | `sha256:36aacea82d9746374c075502185b14634e8267df8f255b86f6f6f50145921939` | `epcerp-api:team-36ffe1d` |
| `tsc-reranker` | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` | `epcerp-tsc-reranker:local` |
