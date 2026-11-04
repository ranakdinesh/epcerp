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
    references/
      domain.ts                  kinds, permissions and compatible code generation
      validation.ts              strict input and query parsing
      service.ts                 directory lifecycle and state/city transactions
      repository.ts              whitelisted directory persistence
```

Add the next area under `master`, rather than creating a new top-level module for
each small directory. Keep each area's validation, repository and permissions
distinct; do not combine all logic into one service file. Future mobile adapters
can call the same application services.

## Current ownership

**Clients, client types, industries, states and cities** run natively in Next.js.
Items, vendors and other master areas remain Go-owned until their respective
migrations. Clients and reference administration use the same transaction advisory
lock. This prevents a client save racing a reference deactivation or city/state
reassignment. Go consumers continue reading the same tables.

Set `EPCERP_CLIENTS_OWNER=nextjs` on both API and frontend. Local Compose defaults
both to Next.js; absent the setting, existing deployments keep Go ownership.
The native adapter serves the existing browser routes. Go stops registering the
six migrated Clients HTTP operations. Its published client-directory reader stays
available to Projects and other unmigrated consumers. There is no fallback to Go
after a native operation fails. Explicit `EPCERP_CLIENTS_OWNER=go` on both services
is the rollback path; leave `EPCERP_AUTH_OWNER=nextjs` unchanged.

`EPCERP_REFERENCES_OWNER=nextjs` independently migrates all four reference
directories. Set it consistently on API and frontend; local Compose defaults to
Next.js, while an absent setting elsewhere retains Go ownership. Go unregisters
only these HTTP operations and retains its published reference readers. Set
`EPCERP_REFERENCES_OWNER=go` on both services for reference rollback; keep Auth and
Clients ownership unchanged. Native errors never fall back to a second writer.

Reference writes require `settings.client_types.write`, `settings.industries.write`
or `settings.locations.write`; reads require their respective `.read` permission.
State deactivation retires active cities atomically. Reactivating a state does not
reactivate its cities. City creation/editing and activation require an active
parent state. Edits preserve existing IDs, creation metadata and inactive status.
Names and generated codes retain Go's whitespace, Unicode and duplicate semantics.
Reference JSON bodies are bounded to 4 KiB at both the browser proxy and native
adapter. Responses preserve the existing five-field contract.

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
Reference tests also compare all four kinds against Go, exercise live permissions,
parent filters, duplicate scopes, preserved metadata, cascade rollback, inactive
parent guards and concurrent state/city lifecycle operations.

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
