# Master Data / References: local verification, 4 November 2026

## Scope and organization

Client types, industries, states and cities now run in the existing Next.js
Master Data module, alongside Clients. Auth stays separate. No new application,
service, database or migration framework was added. Items, categories,
manufacturers, templates, vendors and their workflows remain Go-owned.

The new `frontend/src/server/master/references` area separates domain rules,
validation, application services and SQL repositories. The common Master HTTP
adapter serves the existing browser routes. Future API adapters can reuse these
services. See [the module guide](../../frontend/src/server/master/README.md).

`EPCERP_REFERENCES_OWNER=nextjs` is set on both local API and frontend, independently
of Auth and Clients ownership. Go unregisters the five reference HTTP route
patterns while retaining its published readers for unmigrated workflows. Native
operation failures do not fall back to Go. Existing PostgreSQL tables, IDs,
permissions, creation metadata, name keys and response contracts are retained.

State deactivation also retires active cities in one locked transaction. State
reactivation leaves cities inactive. City creation, editing and activation require
an active parent. The same advisory lock protects Clients reference selections.

## Source actually tested

Integration branch: `codex/development`, checkpoint
`744d47592326a6a162d098810cb83ac74bae1e87`, plus this reference migration's local
working-tree changes. The checkpoint contains the preceding verified Auth and
Clients migrations. These changes have not been pushed or merged into `main`.

Remotes were fetched before verification. Every remote branch tip was confirmed
to be an ancestor of the integration checkout: `main`/`chor/auth` at `7563358`,
remote development at `f896cf1`, `tenderFMS` at `1c7c4f6`, table evidence at
`7acaa51`, and TSC guardrails at `1786850`.

Final application/test/deployment source SHA256:
`97b5fcffd5be5da3e47c84aed209e77c400192b7ccbb32d7fd4104f061fafa88`.
Full remote commits and activated image IDs are retained in the private manifest.
API, migrator, frontend and worker binaries came from this integration checkout;
the cached reranker source was unchanged.

## Feature verification

- **47 server checks passed**, with no skips, using only
  `epcerp_auth_qa_master_20261104`. The run includes Auth and Clients regressions.
  Run `bash frontend/scripts/verify-master.sh` with a guarded QA database URL.
- All four reference kinds were compared against the actual Go parser and domain
  preparation: whitespace, Unicode casing/folding, generated code limits, null
  handling, canonical UUID forms, unknown fields and invalid input.
- Real PostgreSQL checks cover granular permissions, immediate role revocation,
  create/edit/list/filter, response fields, actors, preserved creation metadata and
  inactive status, duplicate name/code scopes, city reassignment, inactive parent
  rejection, state/city cascade and concurrent lifecycle operations.
- A deliberately failing QA constraint proves city retirement rolls back if the
  state update fails. That constraint was removed in a `finally` block and was
  never installed in the operational database.
- Go Master, Auth and architecture regressions passed. **232 frontend tests
  passed**. TypeScript passed; lint had zero errors and four existing warnings.
  The final Linux Docker frontend build passed.
- Chrome/Playwright browser checks passed on the final isolated Docker image at
  `http://localhost:59092`, using synthetic credentials and disabled providers.
  The Browser plugin was unavailable. All four screens exercised create/edit and
  lifecycle operations; the state cascade and absence of automatic city revival
  were confirmed. Page identity, meaningful content, absence of framework errors,
  browser console and document overflow checks passed at 1440×1000 and 390×844.
- The Clients browser regression also passed against the final image: reference
  selectors, create/edit/lifecycle and visibility through Go's project client
  directory. Temporary QA containers and private environment files were removed.

Browser evidence was inspected locally:
`/private/tmp/epcerp-references-browser/cities-desktop.png` and
`/private/tmp/epcerp-references-browser/cities-mobile.png`.
The initial QA run needed development HTTP enabled in its temporary environment;
its mobile identity assertion was adjusted for the existing VTPL CORE header.
Neither correction changed application behavior.

## Combined-stack activation and preservation

The preview at `http://localhost:9092` now uses native Auth, Clients and references.
All nine stack services are healthy. The Go business readiness endpoint returns
200; retired Go Clients/reference endpoints return 404. Browser-facing directory
routes return 401 without a session, and the local login page returns 200.
These smoke checks do not constitute acceptance of every integrated feature.

Application writers were stopped before taking a private PostgreSQL archive.
The backup passed both table-of-contents inspection and complete SQL decoding.
The matching migrator ran before services restarted. **Migration 98 remains clean**;
there was no new schema migration, database reset or forced migration version.

Sorted aggregate fingerprints before and after activation matched for existing
clients, contacts, client types, industries, states, cities and all user password
hashes. Users do not need to change passwords because of this migration. Database
and object-storage volumes, signing/encryption/storage keys and existing provider
approval settings were preserved. No real messages, paid provider jobs or private
document uploads were submitted.

Private verified backup and activation manifest:
`/private/tmp/epcerp-references-before-20261104T085056Z`.
This directory contains private database/configuration material and must stay local.

Reference rollback uses its recorded prior image tags and sets
`EPCERP_REFERENCES_OWNER=go` consistently on API and frontend. Keep
`EPCERP_AUTH_OWNER=nextjs` and `EPCERP_CLIENTS_OWNER=nextjs`. No database restore is
needed to switch ownership. Production was not deployed. Authenticated operational
acceptance with the user's current session remains a user check; automated
mutations were confined to the isolated QA database.
