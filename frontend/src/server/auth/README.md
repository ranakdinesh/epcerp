# Native authentication and authorization

This module owns credentials, sessions, account security, user administration,
role administration and the permission registry when `EPCERP_AUTH_OWNER=nextjs`.
The same ownership setting must be supplied to the Go API and Next.js. Local
Compose defaults both to Next.js; existing deployments remain on Go until their
configuration is explicitly changed. There is no fallback to Go after a native
authentication error.

## Boundaries

- `domain.ts`: identity, permissions and password policy.
- `security.ts`: bounded Argon2id/i verification and Go-compatible HS256 tokens.
- `repository.ts` and `../db.ts`: parameterized SQL and transaction boundaries.
- `sessions.ts`, `account.ts`, `administration.ts`: application services.
- `http.ts` and `validation.ts`: adapters for existing browser routes. Future mobile
  routes can call these services; no new mobile API is published in this change.
- `bootstrap.ts`: checks the applied migration version and reconciles the registry
  and recovery administrator without replacing existing credentials.
- `mail.ts`: encrypted durable credential-email queue, leased delivery and transport.

Go retains a database-backed identity reader and live authorization checks for
unmigrated business modules. Profile presentation/photos, organisation hierarchy,
team inventories and offboarding remain Go-owned business workflows. In Next.js
ownership mode Go does not register the migrated login/session/account/user/role
HTTP operations, run their bootstrap, or consume credential-email jobs. Its old
implementation remains available solely for an explicit rollback to `go` ownership.
Removing that rollback code is a later step after release acceptance.

## Data and compatibility

Use the existing PostgreSQL database and the existing ordered Go migrations.
There is no ORM schema sync, new migration history, reset, or schema change here.
The existing schema must be clean and at least migration 96. The local integration
checkout currently owns migration 98; use its migrator before starting services.

Existing Argon2id and Argon2i PHC hashes, user IDs, role grants, refresh families,
access-token hashes, signing keys, issuer/audience and credential versions are
preserved. Existing passwords and valid tracked sessions can cross the ownership
switch. Mandatory changes for existing temporary credentials still apply.

SQL audit triggers and business events stay transactional. Authorization is read
from live database roles, never trusted from a browser or embedded permission list.
Refresh replay commits family revocation before returning 401. Existing access
tokens are revoked on logout, session termination, password reset/change and
deactivation; reactivation does not restore old sessions.

Public login has a shared PostgreSQL-backed limit of 100 attempts/minute plus
account-level five-failure/15-minute lockout. Recovery/reset/verification each have
a shared limit of five requests/15 minutes across instances. These aggregate limits
do not trust caller-supplied IP headers. Revisit their scope if traffic increases.

## Configuration

Next.js needs server-only `EPCERP_DATABASE_URL`, the same `VTPL_JWT_SECRET_KEY`,
`VTPL_JWT_ISSUER` and `VTPL_JWT_AUDIENCE` as Go, and the existing
`EPCERP_IMAGE_ENCRYPTION_KEY` for queued mail. Existing optional session lifetime
settings keep their Go names and bounds. Never prefix these variables with
`NEXT_PUBLIC_` or place credentials in tracked files. Bootstrap credentials are
needed only when provisioning a new database; no existing password is overwritten.

Email needs the existing Brevo key, verified sender and public URL **and** explicit
`EPCERP_EMAIL_DELIVERY_ENABLED=true`. Local Compose defaults delivery to false.
User creation/reset fails atomically when delivery is unavailable. The private
AES-GCM queue uses the same key derivation and envelope as Go; welcome/reset jobs
survive the switch in either direction. Next's Node server consumes leased jobs
from `instrumentation.ts`; this requires a persistent Node deployment, as used by
the local Docker stack. A future serverless deployment needs a dedicated consumer.

Keep the same ownership setting on both services. Stop old auth writers before
switching, preserve all secrets/volumes, take a private verified backup, start the
matching services, then verify existing login, permissions and unmigrated workflows.
Rollback switches both services to `EPCERP_AUTH_OWNER=go` with the same schema and
keys. Never force a migration version or restore an older database merely to roll
back application ownership. Restoring a database backup would discard newer writes.

## Verification

`npm test`, `npx tsc --noEmit`, `npm run lint`, and a production build cover the
existing frontend. `npm run test:auth` runs native security tests; database and Go
interop tests explicitly skip unless their environment is supplied.

For the complete migration suite, create an empty isolated database whose name
starts with `epcerp_auth_qa_`, export its `EPCERP_DATABASE_URL`, then run:

```sh
bash frontend/scripts/verify-native-auth.sh
```

The runner applies this checkout's owned migrations only to that QA database,
builds a synthetic Go compatibility checker and runs the native tests. It mocks
every outbound email call. It does not read operational user passwords, reset
operational accounts or contact providers. It leaves synthetic test records in
the QA database. The Go checker proves both Go Argon2 variants work in Next.js and
Next.js tokens/password hashes work in Go.

Also run `go test ./internal/modules/auth/... ./internal/architecture/...` from
`backend`. Combined-stack/browser smoke verification is separate from these tests.
