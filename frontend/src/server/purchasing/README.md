> Historical migration notes. Current runtime and setup: [frontend README](../../../README.md). The Go backend is no longer in the working tree.

# Purchasing server module

This module owns the complete purchasing HTTP workflow in Next.js. The existing database, password hashes and encrypted object formats are retained. Set `EPCERP_PURCHASING_OWNER=nextjs` consistently on the frontend and every Go application writer. Native failures never fall back to Go. Go HTTP routes and upload guards are disabled by the same switch; transaction-bound readers needed by unmigrated consumers remain available.

## Layout

- `paths.ts`, `http.ts`, `http-input.ts`: route ownership, HTTP dispatch, bounded JSON/multipart input and private download responses.
- `workspace`, `bo`, `materials`, `indents`, `groups`, `indent-documents`, `holds`, `client-rules`: ownership, preparation and source publication.
- `quotations`, `approvals`, `awards`, `sourcing`, `loi-documents`: received evidence, exact prices, independent Accounts decisions, immutable award allocations and reviewed vendor LOIs.
- `technical`, `orders`, `acceptance`, `advances`: technical/client evidence, signed draft revisions, numbered PO release, vendor response and separate Finance approval/payment evidence.
- `inspections`, `inspection-reports`, `quality`, `client-clearance`, `dispatch`, `site-receipts`, `grns`, `engineer-confirmations`, `accounts-acceptance`: inspected, verified and received material through final Accounts handover.
- `progress`: owned completion facts, exact demand coverage, the original stage weights and calendar signals. Coordination alone cannot complete a specialist stage.
- `documents`: native PDF rendering and embedded licensed fonts. LOI wording and company letterhead stay with `loi-documents`.
- `core`: shared authorization, exact decimal/rational arithmetic, time, bounded evidence inspection, transactional audit and SQL plumbing. Business state transitions stay in their owning capability.

## Boundaries

`service.ts` is the server-only entry point. `policy.ts` owns transitions and validation; `context.ts` joins current published upstream facts; `repository.ts` writes owned tables and immutable history. Published source readers use their caller's database transaction. Catalogue eligibility comes from the master module's purchase readers, not duplicated approval rules.

Every write rechecks live authority and optimistic versions. Financial decisions retain their separate project scope and transactional Director notices. Source edits require explicit downstream revisions. Money and quantities use exact integer/rational calculations. File upload authorization precedes buffering; evidence remains encrypted and purpose/revision-bound.

No purchasing operation sends mail, bank instructions or cloud-model requests. Dispatch and payment actions record actual evidence supplied by an authorized user.

## Verification

Run `frontend/scripts/verify-purchasing.sh` only with an isolated database named `epcerp_auth_qa_purchase_*` and synthetic credentials. The runner applies owned Go migrations, builds the original Go compatibility oracle, and exercises native HTTP writes plus independent Go/native reads, source invalidation, immutable history, rollback, permission revocation, exact quantities/prices and encrypted-file interchange. Cloud and email delivery are disabled; S3 is an in-memory localhost fixture. Concurrent runs are rejected because the Director fixture is shared.

`npm test`, TypeScript, scoped ESLint and affected Go unit suites complement these feature checks. A local Docker preview must use the frozen combined `codex/development` checkout, a verified private backup, matching API/migrator/frontend/worker images, and preservation checks. Preview verification does not authorize a production deployment or merge into `main`.
