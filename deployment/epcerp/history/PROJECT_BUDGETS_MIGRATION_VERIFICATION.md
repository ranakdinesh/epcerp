# Project budgets: native Next.js migration

Verified locally on 5 November 2026. Application commit:
`afa7f4cf8900a535eb897bccda83782898c5675b` on the combined `codex/development`
branch. The preview runs at `http://localhost:9092`. No production deployment,
branch push or merge into `main` was performed.

## Result

Next.js owns `GET/POST /projects/:project/budget` through
`EPCERP_PROJECT_BUDGETS_OWNER=nextjs`, applied consistently to all five application
writers. The same flag retires Go budget HTTP routes. Internal Go readers/inventory
needed by unmigrated consumers remain available. Native failures do not retry Go.

The capability lives under `frontend/src/server/projects/budgets`; its README
explains validation, policy, repository, scoped authority, financial approval and
the published approved-budget reader. Existing UI and database contracts are retained.
It supports cost heads/subheads, per-item BO rates, exact money and rounding, reports,
immutable history, approval snapshots and revisions. Preparing a revision retains
the previous approved baseline for purchasing until a replacement is approved.

Financial approval rechecks live permission and project scope; project management
alone does not grant approval. Saved evidence produces the same fingerprint as Go.
The decision, Director notice, budget and audit event commit together or roll back.
Heads/allocations are deactivated instead of deleted; historical identities and source
snapshots are retained. No purchasing or budget operation calls a cloud provider.

## Feature verification

- 16 budget tests passed, with no skips: native/Go read-write interoperability,
  reports/history, scope/revocation, Director-notice rollback, exact maximum HTTP
  decimal input, hierarchy limits, unit pricing/clearing, revision baselines and
  replacement approval fingerprints including historical item evidence.
- All 42 purchasing consumer tests passed, with no skips, through final Accounts
  acceptance and HTTP/task/progress/PDF parity.
- All 240 frontend regression tests passed. The budget origin guard also has
  a targeted proxy test using the real origin helper.
- TypeScript, scoped ESLint and affected Go budget/Finance suites, including the
  fail-closed ownership guard, passed.
- The frozen production Next.js build passed. Eleven Docker HTTP checks passed
  against a separate synthetic database with Go unreachable and cloud/email disabled.

The isolated host databases are named `epcerp_auth_qa_purchase_*`; no feature test
used operational records. The synthetic Docker checks used the existing private
purchasing QA database. Temporary QA containers and credential files were removed.

## Combined local deployment

All seven fetched remote refs were ancestors before verification. The frozen
integration checkout contains a recorded 62-file concurrent Tender/provider overlay;
later working-tree changes are outside this tested snapshot. The original working
tree was preserved. Exact refs, source hashes, image identities and checks appear in
`PROJECT_BUDGETS_LOCAL_VERIFICATION_20261105.json`.

The frontend was compiled from that checkout and packaged with Linux dependencies
from its exact lockfile; host native packages were excluded. API/migrator and every
worker were built from the same checkout. Docker Desktop/database recovery delayed
packaging; volumes were preserved and no reset or forced migration version was used.

Application writers were stopped before the private backup. Both archive listing
and full restore-stream parsing verified the database dump, which was SHA-256 hashed.
The matching migrator completed with schema 101 clean and unchanged. The prior image
payloads and complete runtime controls remain available privately for service rollback.

The preservation audit compared 192 tables: business data, passwords and permissions
were unchanged, apart from the expected superadmin startup edit-version increment.
Both original document files and all 22 stored objects matched their prior snapshots.
Encryption keys and existing provider approvals/settings were retained. Model-call
count remained 21 throughout cutover; these checks made no provider calls.

All nine services are healthy. Combined checks confirmed native budget ownership on
all five application writers, retired Go budget GET/POST routes, session/origin guards,
existing migrated modules and private renderer protection. This local verification
is separate from release approval or a reviewed merge to `main`.
