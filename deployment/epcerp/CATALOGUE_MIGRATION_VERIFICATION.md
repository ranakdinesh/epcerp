# Master Data / Catalogue: local verification, 4 November 2026

This records the catalogue slice. The subsequent vendor profile/lifecycle
activation is recorded in [vendor profile verification](VENDOR_PROFILES_MIGRATION_VERIFICATION.md).

## Change and scope

Items, categories/subcategories/base units, manufacturers and configuration
templates now execute natively in the existing Next.js Master Data module. Their
26 HTTP route patterns keep the existing browser contracts. No additional Docker
service, database, schema migration or ORM was added. Vendor/supplier document and
approval workflows remain Go-owned.

Each area separates pure domain rules, application services and repositories.
Shared catalogue adapters handle HTTP, reviewed parameterized SQL, validation and
compatible serialization. The native event writer appends the same business
events, with zero delivery targets, in the transaction containing the subject and
its audit history. See [the module guide](../../frontend/src/server/master/README.md).

`EPCERP_CATALOGUE_OWNER=nextjs` is set consistently on local API and frontend.
Go stops registering only the migrated catalogue HTTP operations. Its published
catalogue readers remain available to project materials and purchasing. There is
no native-failure fallback to a second writer. Existing deployments without this
flag keep Go ownership.

A routing boundary found during verification was corrected: client purchase
policies remain Go-owned despite their nested `/clients/{id}/purchase-rules` URLs.
Both a route-ownership regression and the real Go policy endpoint were checked.

## Source actually verified

Integration branch: `codex/development`, base checkpoint
`95ba388731bc4fedf0c3e733378be5c2addf7295`, plus this catalogue migration's local
working-tree changes. The checkpoint saves the preceding verified reference-data
migration. No changes were pushed or merged into `main`.

Remotes were refreshed before building. All remote tips were confirmed ancestors:
`main`/`chor/auth` at `7563358`, remote development at `f896cf1`, `tenderFMS` at
`1c7c4f6`, table evidence at `7acaa51`, and TSC guardrails at `1786850`.
The private manifest records full commit and running image IDs.

Verified application/test/deployment source SHA256:
`c89b5ea9c5fa7c71e68f38c77339d2e9c0070fbe8da1053e9808dc474fe1cffc`.
API, migrator, frontend and worker binaries were built from this integration
checkout. API/document workers used the verified cached Linux runtimes; the
reranker's source was unchanged. Native requests never invoke a Go subprocess.

## Feature-specific checks

- **63 server checks passed**, with no failures or skips, against
  `epcerp_auth_qa_catalogue_20261104`. This includes Auth, Clients and reference
  regressions. The complete runner is `bash frontend/scripts/verify-master.sh`
  with a guarded QA database URL; Auth requires the `epcerp_auth_qa_` prefix.
- **93 offline fixtures** compare native validation, cleaned values, hashes,
  imports, template previews and item derivation with the actual Go code. They
  cover Unicode models, NFKC/full folding, whitespace, generated prefixes,
  controlled options, null/type failures and legacy source fingerprints. The
  full-fold mapping is compared with Go's generated table.
- Real PostgreSQL checks cover permission boundaries, live grant revocation,
  category parent/unit inheritance, browse/detail/filter responses, optimistic
  versions, reasons, manufacturer eligibility and lifecycle, and immutable
  template revisions with retained field/option codes.
- Five concurrent identical item creates return the same issued ID, with one
  creation-history entry. Tests cover immutable item/snapshot/history evidence,
  sequential numbering, status guards, inactive references, reuse of legacy
  pre-normalization model candidates, and rejection after serial 999.
- Timestamp microseconds and arbitrary legacy JSON numbers retain their precision
  in responses and audit snapshots. Decimal/exponent JSON tokens are rejected for
  integer request fields instead of being silently coerced by JavaScript.
- Failure constraints in the QA database prove that event-journal failure rolls
  back item creation, manufacturer-history failure rolls back a profile, and
  publication-history failure rolls back a template revision. Constraints are
  removed in `finally` blocks and were never installed in the operational DB.
- Existing Go Master, Auth, architecture, project-items and purchasing regressions
  passed. **232 frontend tests passed**. TypeScript passed; lint had zero errors
  and four existing warnings. The final Linux frontend image built successfully.

The runner also checks that generated native catalogue SQL matches the existing
reviewed query source. Tests submit no real messages or provider jobs.

## Browser interaction and consumer proof

Environment: isolated Docker frontend at `http://localhost:59092`, Chrome through
Playwright, synthetic credentials, providers/email disabled; desktop 1440×1000
and mobile 390×844. The Browser plugin was unavailable. No operational records
were mutated by browser automation.

The verified interaction path was login → category/base-unit/subcategory creation
→ first configuration → revised configuration → manufacturer create/edit → item
issuance → manufacturer lifecycle → item retire/reactivate/history.

| Check | Result |
| --- | --- |
| Intended URL, title where supplied, visible application identity | Passed |
| Meaningful content and no framework error overlay | Passed |
| Create/edit/publication/lifecycle controls update persisted state | Passed |
| Unexpected browser/console errors | None |
| Screenshot inspection and document-level mobile overflow | Passed |
| Go purchasing selector reads the native item | Passed |
| Go selector excludes/reactivates the item with manufacturer lifecycle | Passed |
| Go saves a synthetic project material requirement using the native item | Passed |
| Go client purchase-policy route remains reachable through the frontend | Passed |

An unconfigured template intentionally returned 404 before its first publication;
the existing UI handled that pending state. The QA harness supplied the required
same-origin header for its protected project write. Temporary containers and
private QA environment files were removed after verification. Synthetic database
fixtures were retained for review.

Inspected screenshots:
`/private/tmp/epcerp-catalogue-browser/categories-desktop.png`,
`/private/tmp/epcerp-catalogue-browser/manufacturers-desktop.png`,
`/private/tmp/epcerp-catalogue-browser/item-history-desktop.png`,
`/private/tmp/epcerp-catalogue-browser/items-desktop.png`, and
`/private/tmp/epcerp-catalogue-browser/items-mobile.png`.

## Combined-stack activation and preservation

The local preview at `http://localhost:9092/dashboard?screen=items` now uses native
Auth, Clients, references and catalogue operations. **All nine services are
healthy.** Go business readiness returns 200. Retired Go Master endpoints return
404; Go's nested client purchase-policy endpoint remains registered/protected.
The local login page returns 200 and unauthenticated native catalogue browser
routes return 401. These smoke checks do not establish acceptance of every other
integrated feature.

Application writers were stopped before a private PostgreSQL archive was taken.
The archive passed table-of-contents inspection and complete SQL decoding. The
matching migrator ran before services restarted. **Migration 98 remained clean**;
no schema upgrade, reset or forced migration version occurred.

Before/after aggregate fingerprints matched for clients, contacts, all four
reference directories, catalogue tables, category/manufacturer/item histories,
import provenance, template fields/options/publications, business events,
projects, project material requirements/events and user password hashes. Existing
records, IDs, relationships, versions, issued SKUs and historical evidence remain
unchanged. Users do not need to change passwords.

Database and object-storage volumes, credentials, signing/encryption/storage keys
and existing provider approval controls were preserved. No real messages, paid
provider jobs or private-document uploads were submitted. Production was not
deployed. Authenticated operational acceptance with the user's current session
remains a user check; automated mutation tests used only synthetic QA accounts.

## Private backup and rollback

Verified backup and activation manifest:
`/private/tmp/epcerp-catalogue-before-20261104T092315Z`.
This contains private database/configuration material and must remain local.

Use the recorded prior image tags and set `EPCERP_CATALOGUE_OWNER=go` on API and
frontend for catalogue rollback. Keep Auth, Clients and references at `nextjs`.
The rollback does not require restoring an older database over newer writes.
Final source, remote-tip, image and preservation evidence is in the private
manifest. Main/release review and production deployment remain separate steps.
