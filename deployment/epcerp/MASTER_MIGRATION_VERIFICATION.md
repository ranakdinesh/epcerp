# Master Data / Clients: local verification, 4 November 2026

## Structure and scope

Clients is now an area inside the shared Next.js `server/master` module. States,
cities, client types, industries, vendors, items and other directories will share
that module as they migrate. They remain Go-owned in this slice. Auth remains a
separate security module. No additional Docker service was introduced.

See [the Master Data guide](../../frontend/src/server/master/README.md) for the
files, rules and reproducible verification commands. Clients application services
contain no SQL; persistence, input validation and domain rules have separate files.
The shared database utility now uses a module-neutral application error type,
and Auth publishes a permission-checking contract that hides credential fields.

`EPCERP_CLIENTS_OWNER=nextjs` is set on both local applications. Go no longer
registers the six migrated Clients HTTP operations. Its client-directory reader
continues to serve Projects and other unmigrated modules. Reference maintenance
continues in Go; native Clients uses its existing tables and shared mutation lock.

## Tested source

Integration branch: `codex/development`, base
`7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2`, plus the Auth and Clients working-tree
changes. These changes are local and have not been pushed or merged into `main`.

Remote tips were fetched and verified as ancestors before local activation:
`main`/`chor/auth` at `7563358`, `tenderFMS` at `1c7c4f6`, remote development at
`f896cf1`, TSC guardrails at `1786850`, table evidence at `7acaa51`. Full commit IDs
and image IDs are recorded in the private activation manifest.

Final application/test source SHA256:
`8ae706d58e72ff0f55eab37bcece89c8394713a8b02b82685e4ad2312edaccaa`.

## Clients-specific verification

- **38 server tests passed**, with no skips in the configured run: Clients and
  validation checks plus the existing native Auth regressions. Tests used only the
  separate `epcerp_auth_qa_master_20261104` database.
- Thirty request fixtures were compared with the actual Go parser, including tax
  rules, website handling, contact types/phones, Unicode name keys, email casing
  and tax-identifier casing. The generated Unicode map matches Go's case mappings;
  JavaScript expansions cannot silently widen accepted tax identifiers.
- Real PostgreSQL tests cover permissions, live role revocation, reference
  activity, city/state and GSTIN/state consistency, contact ownership, retirement,
  lifecycle, duplicate names, pagination, response fields, actor metadata, legacy
  provenance, timestamp microseconds and atomic rollback after a contact failure.
  The failure-injection constraint was installed/removed only in the guarded QA DB.
- Go Master Data, Auth and architecture regression tests passed. The existing
  **232 frontend tests passed**. TypeScript passed; lint had zero errors and the
  same four existing warnings. The final Linux frontend image built successfully.
- Browser verification passed against the final isolated Docker image: synthetic
  sign-in, list/create/edit/deactivate/reactivate, Go reference selectors, immediate
  visibility through Go's project client selector, desktop/mobile screenshots,
  no document-level horizontal overflow and no browser console errors. Synthetic
  mutations were confined to the QA database. Screenshots were inspected locally.

These checks establish Clients compatibility; they do not establish complete
acceptance of every other integrated feature or real provider behavior.

## Local activation and preservation

The local preview at `http://localhost:9092/dashboard?screen=clients` uses native
Clients and native Auth. All stack services are healthy. API/migrator and workers
were compiled from the integration checkout and assembled into the existing
verified local runtimes; the cached search runtime source was unchanged. No model
download, real email, paid provider job or private-document upload was submitted.

Writers were stopped for the initial activation backup. The matching migrator ran
before services restarted. **Migration 98 remained clean**, with no new schema
change, reset or version forcing. Sorted aggregate fingerprints confirmed every
existing client, contact and user password hash was unchanged across activation.
Original IDs, codes, timestamps, provenance and contact history remain in place.
Signing/encryption/storage keys and provider-approval settings were preserved.

The final frontend compatibility patch was backed up and verified separately.
Current private backup: `/private/tmp/epcerp-master-final-20261104T075304Z`.
Initial complete-stack backup: `/private/tmp/epcerp-master-before-20261104T074526Z`.
Both archives passed table-of-contents inspection and complete SQL-stream decoding.
The directories contain private database/configuration material and must stay local.

Clients rollback sets `EPCERP_CLIENTS_OWNER=go` on both applications, using the
recorded rollback images. Keep `EPCERP_AUTH_OWNER=nextjs`; rolling back Clients does
not require reverting Auth or restoring an older database over newer writes.

Production has not been deployed. Operational authenticated acceptance remains a
user check with their current credential; automated browser mutations used only
synthetic QA accounts.
