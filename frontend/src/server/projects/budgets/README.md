> Historical migration notes. Current runtime and setup: [frontend README](../../../../README.md). The Go backend is no longer in the working tree.

# Project budgets

This capability lives within the Projects server module. `http.ts` owns
`GET/POST /projects/:project/budget` when `EPCERP_PROJECT_BUDGETS_OWNER=nextjs`.
Set that flag on every application writer; Go then retires its budget HTTP routes.
The existing database and approval history remain in place; no schema migration is needed.

- `input.ts`, `validation.ts`, `money.ts`, `policy.ts`: bounded command translation,
  exact decimal lexemes and BigInt paise, hierarchy/allocation rules and item pricing.
- `context.ts`: live project authority, eligible owners and current BO item snapshots.
- `repository.ts`: owned budget/head/allocation writes and immutable history/events.
- `finance.ts`: scoped approval, Go-compatible source fingerprint and transactional
  Director inbox delivery. Approval uses saved data and cannot accept browser snapshots.
- `service.ts`: save, price_items, approve and revise inside the shared transaction.
- `approved.ts`: published effective approved baseline for purchasing. A draft revision
  retains the prior approved source until the replacement is approved.

Project-row locking serializes budget, BO-source and purchasing decisions. Auth role
changes are rechecked live. Approved records require a revision before editing;
heads and allocations are deactivated rather than deleted. Stored decimal zero and
struct-field ordering retain Go compatibility, including historical financial hashes.

Run `frontend/scripts/verify-project-budgets.sh` against a separate synthetic database
named `epcerp_auth_qa_purchase_*`. It disables cloud/email and compares native writes,
Go edits, reports/history, source fingerprints, revisions, exact item prices, current
financial scope and rollback when the Director cannot receive a notice. The full
purchasing suite checks consumers of the published approved baseline separately.
