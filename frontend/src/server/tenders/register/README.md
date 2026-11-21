> Historical migration notes. Current runtime and setup: [frontend README](../../../../README.md). The Go backend is no longer in the working tree.

# Tender register ownership

`EPCERP_TENDER_REGISTER_OWNER=nextjs` selects native named capture, register
list/detail/metadata edits, manager options/reassignment and legacy ownership
handover. Go omits these HTTP writers under the same flag. Its transaction-bound
Records/AssignmentActions contracts remain available to unmigrated consumers.

The code stays within the existing Tender module:

- `http.ts`: bounded authenticated request/response contract.
- `access.ts`: live permissions and reviewed manager designation policy.
- `validation.ts`: canonical IDs, text, versions and register query bounds.
- `service.ts`: transaction orchestration, operation replay and manager work.
- `repository.ts`: scoped metadata reads and compatible register projections.
- `sources.ts`: verified receipt consumption and retained named-source linkage.
- `ownership.ts`: legacy owner changes, with named records explicitly excluded.
- `events.ts`: append-only Tender events and business journal records.

Auth publishes credential-free assignment identities. Manager designation uses
**stable reviewed role IDs** in `EPCERP_TENDER_MANAGER_BUNDLE_IDS` (maximum 20),
active membership, account readiness and the complete existing manager permission
profile. Empty configuration produces no candidates; a superuser is not implicitly
designated. No roles, permissions or designation IDs are granted by this migration.

Capture, receipt consumption, source links, provision intent, assignment action,
Inbox notice/history, email intent and idempotency ledger commit in one transaction.
Metadata edits retain content revision; assignment changes cancel obsolete actions
and unsent email intent before creating the new addressed Inbox notice. Replay
rechecks live authority and visibility and returns the current projection.

There is **no schema change**; migration 99 remains required. Existing Go JSON
fingerprint casing/order/escaping is preserved. Password hashes and encryption
formats are unchanged. Existing participant grants confer metadata visibility only.
Legacy ownership uses its original optimistic version and append-only history.

## Explicit remaining boundaries

Project/estimate provisioning was already unimplemented: the Go capture code
created only `pending / project_provider_pending` operations. Native capture retains
that truthful state and null project/estimate IDs. The current capture contract has
no required project client, and there is no estimate provider. Completing that
workflow requires a project/estimate design; no placeholder entities are created.

Email jobs are durable intents, not delivery claims; this module makes no network
calls and does not send messages. Inbox delivery is immediate and transactional.

For compatibility, the older **named-record** capture path retains the existing
prepared document-reading rows and Go safety/reading workers. This does not change
the current draft cloud/manual TSC flow, which publishes verified originals without
OCR. Legacy original/result/history reads, safety verification, format conversion,
archive handling and retention workers retain their existing owners. This flag does
not migrate the whole Inbox, Projects or Tasks modules.

## Verification

`bash frontend/scripts/verify-tender-register.sh` requires an isolated
`epcerp_auth_qa_register_*` database and uses synthetic credentials/documents only.
It applies the owned migrations and builds a Go oracle for operation fingerprints,
manager options, foundation reads and Inbox notice consumption. It verifies atomic
rollback, concurrent replay, stale versions, designation revocation, receipt policy,
participant expiry, assignment cancellation and legacy isolation. It sends no email
or provider requests. Never point the suite at operational data.
