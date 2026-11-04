# Master Data

One module owns the shared directories used across ERP workflows: clients,
vendors/suppliers, items, manufacturers, categories, states/cities, client types and
industries. Each area has its own files inside this module; it is not a separate
application or Docker service. Auth remains a separate security boundary.

## Structure

```text
server/
  auth/access.ts                 published permission-checking contract
  db.ts                          connection pool and transactions
  errors.ts                      shared application failures
  master/
    http.ts                      adapter for migrated browser operations
    shared/
      references.ts              existing reference readers and shared locks
      text.ts                    compatible persisted name keys
      simple-fold.json           generated Go Unicode mapping
      validation.ts              shared IDs and validation failures
    clients/
      domain.ts                  records, tax/contact rules
      validation.ts              input and query validation
      service.ts                 use cases and permission enforcement; no SQL
      repository.ts              parameterized SQL and response mapping
```

Add the next area under `master`, rather than creating a new top-level module for
each small directory. Keep each area's validation, repository and permissions
distinct; do not combine all logic into one service file. Future mobile adapters
can call the same application services.

## Current ownership

Only **Clients** has migrated in this slice. States/cities, client types, industries,
items, vendors and other master areas remain Go-owned until their respective
migrations. Clients reads the existing reference tables within its transaction,
using the same advisory lock as Go reference maintenance. This prevents a native
save racing a reference deactivation or city/state reassignment.

Set `EPCERP_CLIENTS_OWNER=nextjs` on both API and frontend. Local Compose defaults
both to Next.js; absent the setting, existing deployments keep Go ownership.
The native adapter serves the existing browser routes. Go stops registering the
six migrated Clients HTTP operations. Its published client-directory reader stays
available to Projects and other unmigrated consumers. There is no fallback to Go
after a native operation fails. Explicit `EPCERP_CLIENTS_OWNER=go` on both services
is the rollback path; leave `EPCERP_AUTH_OWNER=nextjs` unchanged.

## Existing data and rules

No schema change or new migration system is introduced. Existing records, IDs,
client codes, actor/timestamp fields, import provenance and contact rows remain in
the same database. Updates retain client codes and never rewrite legacy columns.
Omitted/deactivated contacts are retired, not deleted; reactivating a client does
not reactivate those contacts. Existing permissions stay granular:
`settings.clients.read` and `settings.clients.write`.

The service resolves active client type, industry, state and city selections,
checks city/state membership, verifies PAN/GSTIN format/checksum/state consistency,
and validates contact ownership. Imported name keys use Go's Unicode simple fold,
including its treatment of `ß`, Greek sigma, dotted I and whitespace. Do not
replace this with JavaScript lowercase or a locale-dependent comparison. PostgreSQL
timestamp fractions are retained in responses, including microseconds.

This preserves the existing Clients audit fields and soft-retired contacts. The
old Clients directory has no immutable revision-history API; this migration does
not invent one or move purchasing's client-policy history into Master Data.

## Verification

Provide `EPCERP_DATABASE_URL` for a deliberately isolated QA database whose name
starts with `epcerp_auth_qa_` or `epcerp_master_qa_`, then run from the repo root:

```sh
bash frontend/scripts/verify-master.sh
```

The runner applies this checkout's migrations only to the QA database, compares
native validation with the actual Go parser, tests real SQL behavior and runs the
Auth regressions. Email is mocked. The test-only failure constraint used to verify
rollback is installed and removed only inside that guarded QA database.

Regenerate the checked-in Unicode mapping after a deliberate Go Unicode upgrade:

```sh
cd backend
go run ./tools/master-compatibility > ../frontend/src/server/master/shared/simple-fold.json
```

The browser mutation verifier `frontend/scripts/verify-master-clients.mjs` is
restricted to an isolated server at loopback port 59092. It tests create/edit,
lifecycle, reference selectors and visibility through Go's project directory.
Operational preservation and full-stack readiness checks are recorded separately
under `deployment/epcerp`.
