# Tender FMS ownership and dependency map

Tender is already one business module. Its process composition boundary is
`module.go`; it does not require a separate API container or database connection.
PostgreSQL schema `tender` separates its persistence from Auth and other modules.

## Code ownership

| Location | Responsibility |
| --- | --- |
| `domain/` | Tender identity, source intake, named capture, assignment and access rules |
| `ports/` | Capability-specific persistence, identity, document and notice interfaces |
| `application/service.go` | Legacy source-intake and document use cases |
| `application/records.go` | Named identity, eligible Manager and metadata-participant foundation |
| `application/capture.go` | Receipt capture, request replay, metadata and Manager reassignment |
| `application/ownership.go` | Existing legacy owner handover |
| `adapters/inbound/` | HTTP translation, strict requests, safe response projections |
| `adapters/outbound/` | Tender repositories and bridges to published shared-service contracts |
| `adapters/outbound/internal/db/` | Generated sqlc code; never hand edit |
| `contracts/` | Published transaction-bound Records and AssignmentActions contracts |
| `sql/queries/` | Tender-owned, schema-qualified sqlc query sources |
| `sql/migrations/` | Owned immutable historical SQL and forward schema migration |

There is no duplicate Tender module in Projects or Purchase. These focused services
belong together under Tender; shared services below remain independently owned.
Do not merge other modules into Tender merely because its workflow calls them.

## Database ownership

Migration 90 moves these twelve existing tables from `public` using `SET SCHEMA`:

- `tender.tender_fms_intakes`
- `tender.tender_fms_events`
- `tender.tender_fms_owner_changes`
- `tender.tender_fms_manager_assignments`
- `tender.tender_fms_record_events`
- `tender.tender_fms_participant_grants`
- `tender.tender_provision_operations`
- `tender.tender_capture_operations`
- `tender.tender_source_links`
- `tender.tender_source_actions`
- `tender.tender_email_jobs`
- `tender.tender_email_attempts`

Keep table names and all existing IDs, records, actors, versions and constraints.
Do not recreate tables, copy records into new IDs or change the pool search_path.
Existing indexes, triggers and foreign keys follow the table objects. There is no
Tender-specific stored function to relocate; shared immutable-evidence functions
remain with their existing owners.

Auth's identity/permissions remain in `auth`. Projects, Purchase, document packages,
document readings, inbox and event-journal tables remain in their existing schemas.
Cross-schema foreign keys preserve references; a schema move is not a microservice
split or a database-enforced replacement for application architecture boundaries.

## Shared services and actual coupling

| Service | Actual Tender dependency | Ownership boundary |
| --- | --- | --- |
| Auth | Identity resolver, ready users, reviewed bundle eligibility, permissions | Published Auth contracts, live checks; no Tender writes to users/roles |
| Document reading | Validate/queue/read bounded encrypted original and extracted artifacts | `Documents`/capture bridges call document-reading contracts; shared worker retained |
| Document packages | Consume reviewed receipt and enumerate accepted members | Package-owned receipt and member contracts in the caller's shared transaction |
| Event delivery | Append restricted business events atomically | Shared journal via published writer |
| Communication inbox | Deliver addressed Manager notices and source reauthorization | Inbox owns notices; it reads Tender action evidence through `AssignmentActions` |
| Auth offboarding | Discover Tender responsibility and use source-owned handover | `WorkInventory` provided through the generic Auth work-source contract |
| Projects | Existing optional `project_id` FK on pending provisioning operation | Actual staged Project/estimate provisioning is still pending, not implemented by this schema move |
| Purchase | Planned quotation/BOM/PE workflow relationships | No direct Tender-to-Purchase repository imports or SQL calls exist in this increment |

`internal/app/app.go` binds these providers. Inbox consumes a Tender contract;
Tender consumes an Inbox contract. Their composition joins one shared transaction
and does not create a Go package import cycle or allow sibling-adapter access.

The mixed histories at platform versions 21, 86 and 88 remain platform-owned:
21 introduces shared document reading plus legacy Tender intake; 86 owns document
packages and its consumed-Tender FK; 88 evolves shared addressed-inbox rules.
Do not split or rewrite already applied migrations to make folder ownership cosmetic.
Fully Tender-owned history pairs 82, 85 and 87 are relocated unchanged, followed by
90. All providers feed one globally ordered migration registry and version table.

## Frontend and runtime

The existing `frontend/src/components/tenders/` workspace and authenticated
`frontend/src/app/api/tender-fms/[[...path]]/route.ts` proxy remain unchanged.
Routes, permissions, JSON shapes, source-module identifiers and stored provenance
retain their existing values. API, migrator and shared document-worker deployment
must agree on the latest schema version. No new long-running service is added.

Named receipt capture has source code and native tests, but its provisioning
projection explicitly remains pending when Project/estimate providers are absent.
Queued Tender email jobs are not proof of provider acceptance or real delivery.
This cleanup does not complete Tender FMS, register planned permissions, seed staff,
grant visibility, send notifications externally or implement the future scheduler.

## Verification and operator handoff

Run pinned sqlc vet/diff, embedded-history/AST checks and Linux Go checks. Native
PostgreSQL tests cover populated version-89-to-90 upgrade, all twelve tables and
their OIDs/content, external FKs, immutable history, Manager assignment constraints,
rollback/reapply, receipt capture/replay, inbox and offboarding. Never execute the
Windows architecture test executable on this workstation; use the isolated Linux
verification environment due to its reported antivirus detection.

Before upgrading an existing database, take and verify a private full backup,
stop source writers, fingerprint existing records/constraints, apply migration 90,
reconcile the results and start matching images. A bounded downgrade to 89 moves
the same tables back; `DROP SCHEMA tender` has no CASCADE and refuses unknown assets.
Do not edit applied SQL after deployment or grant broad schema access to work around
an application permission failure. Separate runtime roles need explicit schema USAGE
and their approved table privileges; ownership/migrator privileges stay separate.


## Unassigned document-reading drafts (migration 91)

`application/drafts.go` owns uploader-scoped intake independently of named capture.
`ports/drafts.go`, `domain/drafts.go` and the matching inbound/outbound adapters
provide create/list/search/read, retained batch attachment, original/result/history
access and authorised retry. Drafts do not create a Manager assignment, Project,
estimate, action, notification or email. Later handoff must use explicit contracts.

`document_drafts`, `draft_sources` and immutable `draft_events` belong to Tender.
Package retention, manifest and publication remain Document Packages-owned; reader
jobs/results/history remain Document Reading-owned. Root composition binds all
providers to one transaction for publication. The package worker dispatches accepted
draft batches automatically and continues other drafts when one owner is blocked.

Both runtime workers use infrastructure's shared PostgreSQL advisory slot and
fenced renewable claims. See deployment/epcerp/compose.yaml for the private scanner,
persistent signature/source volumes and bounded workers. Fifteen-minute attempts
may require splitting a very large scan. Processing completeness is not reviewed
business correctness; Office-part/low-OCR warnings require source review.
