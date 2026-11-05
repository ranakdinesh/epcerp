> Historical migration notes. Current runtime and setup: [frontend README](../../../README.md). The Go backend is no longer in the working tree.

# Native personal Inbox

`EPCERP_INBOX_OWNER=nextjs` selects native list/detail and read/acknowledgement
receipts. Matching Go HTTP routes are absent under the same flag; an invalid Go
owner fails startup. Existing Go transaction-bound Finance/quotation delivery
contracts remain available to source modules that have not migrated. Native
supplier/Tender delivery writers continue to share their source transaction.

- `http.ts` and `validation.ts`: bounded routes, payloads and query contract.
- `service.ts`: live authorization, personal scope, optimistic receipts.
- `repository.ts`: exact timestamp mapping, recipient-scoped reads, history/journal.
- `types.ts`: compatible notice/history DTOs.
- `supplier-review.ts`, `tender-assignment.ts`: published transactional delivery.

All notice kinds are supported: Director, quotation, supplier review and Tender
manager. A superuser can read only notices addressed to that account. Read access
requires `communication.inbox.read`; receipts additionally require `.write`.
Auth's administration lock keeps role/status checks consistent with the transaction.
There is no password/hash, permission, database or object-storage migration.

Opening a notice never marks it read. Acknowledge sets read_at if absent and records
acknowledged_at. Existing receipt timestamps never change. Every actual change
advances the notice version and appends before/after history plus its correlated
business event atomically. The caller must send the current version: stale retries
return 409; repeating a completed action with the current version is a no-op.
History is limited to the latest 100 events and listings to 20/50/100 rows per page.
PostgreSQL microseconds are retained without JavaScript Date truncation.

A notice is retained correspondence, not authority over source work. Superseded
Tender assignments stay in the original recipient's Inbox and can be acknowledged,
as in Go. Receipts cannot change, approve, revive or complete the assignment.
Source modules independently validate current ownership/permissions when opened.
No email, external messaging or provider calls are made by this module.

The browser proxy rejects cross-origin receipt writes. Native service failures never
fall back to another writer. Runtime schema remains owned by the Go migrator.

`bash frontend/scripts/verify-inbox.sh` requires an isolated
`epcerp_auth_qa_inbox_*` database and synthetic credentials. The Go HTTP oracle uses
UTC, matching Docker, and checks full DTO/history parity in both directions.
Tests cover all notice kinds, read-only/revoked accounts, recipient isolation,
filters, microseconds, duplicate/stale/concurrent receipts, transactional rollback
and obsolete assignment correspondence. Operational data is never used as a fixture.
