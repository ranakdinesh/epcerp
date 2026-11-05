# Entire purchasing workflow migration

User-confirmed scope: the entire purchasing workflow, not just workspace ownership.
Integration branch: `codex/development`. Existing database, credentials, encrypted
files, history, permissions, explicit provider approvals and budgets must be preserved.
No production deployment, real messages or paid provider calls are authorized here.

Implementation order (all phases are required before claiming full completion):

- [ ] Shared transaction/access/decimal contracts, workspace ownership, dashboard
      and progress over all purchasing stages.
- [ ] BO preparation, register completion/reopen, requirement publication and PDF;
      formal indents, grouping, vendor documents, invitations/notifications and holds.
- [ ] Quotations, encrypted attachments, tax/exact totals, comparisons/shortlists,
      budget preview, Accounts approval queue/review, task/inbox correspondence.
- [ ] Vendor awards, allocations, sourcing/LOI documents/review, technical approvals
      and evidence uploads, client-stamped purchase orders and release/document paths.
- [ ] Accounts acceptance, advances/disbursement, inspections/reports, quality and
      client clearance, dispatch, site receipt, GRN, engineer confirmation and linked
      final Accounts evidence/progress.
- [ ] Every registered purchasing route and dependent purchasing queue/document
      path accounted for; Go HTTP ownership retired only after native parity checks.
- [ ] Matching combined Docker API/migrator/frontend/workers built from one frozen
      integration checkout; owned migrations applied; private verified backup and
      preservation audit; isolated end-to-end tests and combined local smoke checks.

New implementations stay inside `frontend/src/server/purchasing`, organized by
business responsibility. Existing Go is the compatibility oracle and remains the
active runtime until the corresponding native path is verified and explicitly owned.
No generic permissive state machine or proxy facade qualifies as a migration. Source
snapshots, version checks, financial separation, immutable audit evidence, storage
signatures/encryption and invalidation of stale upstream evidence must remain.
Concurrent Tender/provider work is outside this feature's commits and must survive
combined local deployment. Record exact tested commits, overlay hashes, image IDs,
schema version and phase-specific versus combined verification separately.

## Current implementation checkpoint

Native services exist for shared purchasing scope/ownership, exact decimal/rational
arithmetic, BO/material publication, formal indent transitions and source evidence,
item/vendor grouping, holds, vendor indent documents/terms/dispatch records and
quotation revisions/comparisons/encrypted evidence, Accounts shortlists/drafts/budget previews/decisions/queues and atomic inbox correspondence, and the shared twenty-stage package catalogue/assignments/blocker coordination. They are **not enabled in live
routing yet**. PDF and complete downstream approvals/execution/progress remain required.

Twenty synthetic compatibility checks currently pass using the original complete
Go purchasing composition as the oracle. They verify bidirectional data and history,
atomic publication/rollback, BO reopen/version handling, formal release limits,
issued grouping/invitations, holds, frozen vendor terms/idempotent generation,
print/dispatch notices, exact quotation calculations and encrypted-file interoperability. Financial scope is also compared with the original Go access gate, including explicit profile requirements and revocation for the PM. New checks cover native package creation/assignment/task-only views/captured resolver/Go resolution, frozen shortlist history, approved budget binding, Finance-only budget visibility, missing-Director rollback, and direct Go/native decision fingerprint equivalence.
This is a checkpoint within the requested full migration, not a claim of completion
or a Docker deployment. The existing local preview remains on its prior verified build.
