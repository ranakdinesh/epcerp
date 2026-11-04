# Next.js Auth migration: local verification, 4 November 2026

## Source and ownership

Tested on `codex/development` at integration head
`7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2`, with the Auth working-tree changes in
this checkout. These Auth changes have not been committed, pushed or merged into
`main`. All fetched remote tips were verified as ancestors before stack testing:

| Remote branch | Included commit |
| --- | --- |
| main / chor/auth | `756335819939beb72596e1200a7c9c510660f265` |
| tenderFMS | `1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` |
| codex/development | `f896cf1c5cc513c7721b4354a8441c102b1947c9` |
| codex/tsc-processing-guardrails | `17868507ffaa76221dbb8574856a819149973cac` |
| codex/table-aware-tender-evidence | `7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2` |

The local API and frontend both run `EPCERP_AUTH_OWNER=nextjs`. Next.js owns login,
refresh/logout, session administration, recovery, account security, users, roles
and permission administration. Go's migrated Auth endpoints return 404 in this
mode. Go retains its live identity/permission reader and business workflows,
including employee hierarchy, offboarding and profile/photos. The old Go Auth
implementation remains available for explicit rollback until release acceptance.

Application/test source digest recorded at activation:
`b3f66d088276a600b07999359dde5ed14b6edf1137588aee3cc938269dcd25b7`.
The private activation manifest records the source digest and image IDs.

## Auth-specific verification

- **22 native security/database tests passed**, no skips in the configured run.
  PostgreSQL tests used only `epcerp_auth_qa_20261104`, separate from the ERP database.
- Go-generated Argon2id and Argon2i hashes verify in Node; Node-generated hashes
  verify in the actual Go verifier. Tokens and credential-email encryption also
  interoperate in both directions.
- Tests cover existing password preservation, refresh replay and concurrent
  rotation, logout, session ownership/revocation, stale edits, live role changes,
  privileged-role restrictions, deactivation/reactivation, username history,
  temporary-password onboarding/reset, recovery, email verification, transactional
  audits and rollback. All email HTTP calls were intercepted with synthetic data.
- Go Auth and architecture regression tests passed, including the test that
  migrated routes are unavailable in compatibility mode.
- **232 existing frontend tests passed**. TypeScript passed. ESLint reported zero
  errors and four existing warnings. Production compilation passed with the
  required server-only upstream configuration.
- Isolated Docker/browser verification passed using a Go-issued session captured
  before switching ownership: the session survived the switch, the unchanged
  synthetic password signed in through Next.js, Users and Roles rendered, sessions
  worked, Go projects/profile/organisation reads worked, logout cleared cookies,
  and the mobile viewport had no horizontal overflow or browser console errors.
  Screenshots were inspected locally. The existing client-navigation empty-title
  behavior was accommodated by additionally checking URL and visible app identity.

Run [the Auth verification runner](../../frontend/scripts/verify-native-auth.sh)
against a deliberately isolated QA database to reproduce the security/SQL checks.
The [browser verifier](../../frontend/scripts/verify-native-auth.mjs) has a
`--capture-before` phase for testing an actual ownership switch.

## Combined-stack verification and data preservation

The local ERP stack is healthy at `http://localhost:9092`. API/migrator, frontend,
document/package workers, TSC worker and reranker contain the integrated source.
The native frontend image passed a Linux clean install/build. A package download
stalled the full worker-runtime rebuild, so API/workers were freshly cross-compiled
from this checkout and assembled offline into the existing verified PDF runtimes.
The reranker similarly uses the cached pinned-model runtime plus current source.
This validates the local images, not a clean production rebuild on another host.

An amd64-emulation startup failure in the reranker was resolved by bounded writable
`/tmp` scratch space; its root filesystem remains read-only. The local synthetic
ranking smoke test passed. Four offline synthetic PDF table-extraction tests also
passed. These smoke tests do not establish complete acceptance of the other
integrated feature branches or live provider behavior.

Application writers were stopped before the private database backup. The archive
passed `pg_restore --list` and full SQL-stream decoding. The matching migrator ran
before services restarted. The database stayed at **migration 98, clean**; no new
migration, schema reset, version forcing or volume replacement occurred. A sorted
aggregate fingerprint of every user ID/password hash was identical before and
after activation. Existing signing/encryption/storage keys and provider-approval
settings were compared and preserved. There were no pending cloud-processing jobs
at restart, and no real email, paid provider request or private-document cloud
upload was submitted by this verification.

Private backup and rollback configuration:
`/private/tmp/epcerp-auth-before-20261104T065712Z`.
The directory is private and contains credentials/database material; keep it local.
Rollback uses the tested integration images with **both** applications set to
`EPCERP_AUTH_OWNER=go`; it does not restore an older database over newer writes.

## Remaining acceptance

The saved operational bootstrap password was already stale before the migration:
it failed against the old Go stack. No operational account password was changed
to make testing pass. Authenticated browser tests therefore used synthetic QA
accounts; the operational stack received readiness/access-denial checks and the
complete password-hash preservation check. A user who knows their current password
should perform the final operational sign-in acceptance.

Real email delivery remains disabled locally and unverified. Production has not
been deployed, and local integration is not production acceptance.
