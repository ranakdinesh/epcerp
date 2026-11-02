# Authentication and authorisation completion

Scope agreed on 31 October 2026: implement the remaining Auth administration,
security and profile work; MFA is explicitly excluded.

## Implemented

- Access-token digests are linked to refresh-token families. Logout and individual
  session revocation invalidate that family's access tokens immediately. Refresh
  replay revokes the whole family. Sign-out-everywhere revokes all access and
  refresh tokens for the authenticated account. Other accounts' sessions cannot
  be revoked through these self-service endpoints.
- Account settings lists active sessions, their start/expiry times and the current
  session. It supports individual revocation and sign-out-everywhere.
- Full platform bypass comes only from the actual superuser flag. A role's legacy
  `system:admin` entry no longer creates a frontend/backend authority mismatch;
  new role grants of this reserved sentinel are rejected. Non-superadmins cannot
  assign the reserved superadmin bundle or edit superadmin accounts/bundles.
- Login failures are throttled in PostgreSQL by canonical account identity, so
  username/email aliases and multiple API replicas share the same backoff. Five
  failed attempts in fifteen minutes block further login attempts for fifteen
  minutes. Expired attempt records are cleaned up in bounded batches. General
  request quotas use verified identities where available instead of pooling
  authenticated users behind the frontend's network address. Invalid credentials
  still use network quotas; forwarded addresses are not trusted.
- All browser API mutations, including login/logout, require same-origin requests
  in the Next.js proxy. Safe read methods remain available without an Origin.
- Migration 96 adds immutable administration history for account/contact/email
  changes, user-role assignments, role details/status and permission assignments.
  History excludes password hashes and credential versions, records actor and
  before/after values, and requires `admin.audit.read`. The UI shows the latest
  100 changes, with an optional user filter.
- Users and roles expose edit versions. Their forms submit the version they
  loaded; conflicting edits return 409. Explicit reload controls discard the
  outdated form and load current data for review. Update APIs retain compatibility
  with older callers that omit the optional expected version.
- Roles can be edited, activated/deactivated and have all permissions removed.
  Read-only users do not see write controls. Inactive roles grant no permissions;
  existing inactive assignments can be retained or removed during contact edits,
  but cannot be newly assigned.
- Self-service and administrator-requested email changes leave the existing
  address unchanged until the recipient confirms a one-time, thirty-minute
  verification link. Only a digest is stored. The link is bound to the previous
  address and credential version. Confirmation revokes existing sessions; replay,
  expiry, deactivation and intervening credential changes invalidate the proof.
  A correction request cancels pending temporary-password emails to the old
  recipient. Unconfigured/failed delivery is reported without applying the new
  address. Verification uses the existing email transport and approval controls.
- Missing legacy contact details have a profile-completion reminder and an
  existing editable completion form. Real names/mobile numbers must be supplied
  by the account holder or administrator; no values were invented or backfilled.
- Users, Roles, Organisation, Handovers and account security use shared semantic
  action colours and stronger solid-surface/input contrast, including dark mode.

## Migration and operational behavior

Migration 96 is Auth-owned. It advances credential versions once and revokes
existing refresh tokens because pre-upgrade access tokens were not tracked by
session family. Everyone must sign in again after upgrade. It does not change
passwords, activate/deactivate accounts, transfer work or alter encryption keys.
Once administration history exists, rollback is intentionally rejected; use a
forward migration to preserve audit records. An empty migration can be rolled
back and reapplied.

Local deployment must use the combined `codex/development` checkout. Stop writers,
make and verify private database/object/package/environment backups, apply owned
migrations, then start matching API/frontend/worker images. `main` remains the
reviewed release target. This work does not authorise production deployment or
real provider messages.

## Verification

- All backend unit tests with race detection (`go test -race ./...`).
- Auth/API/profile/password-recovery/onboarding/offboarding/organisation integration
  tests, including actual Go HTTP API and migration lifecycle tests, run against
  disposable PostgreSQL clusters.
- New session, replay, revocation, stale-edit, role status, audit immutability,
  email-proof, recipient-cancellation and login-throttle regressions.
- Auth permission matrix: reader, user manager, role manager, auditor, legacy
  sentinel grant and real superadmin.
- 228 frontend tests; TypeScript; targeted ESLint; production Docker builds.
- Browser checks with actual source components and synthetic fixture data:
  prefilled role editing, role deactivation, read-only controls, immutable-history
  presentation, stale-edit rejection/reload, removing all role permissions,
  session listing and individual revocation, light/dark and 390px layout/focus.

Private deployment manifests record exact branch tips, source hashes, image IDs,
backup verification, migration state and checks. Test logs are local under
`/tmp/auth-completion-*`. Real email delivery was deliberately not exercised.
Browser checks on the upgraded authenticated preview require a fresh sign-in;
fixture UI checks and disposable-database checks are distinct from stack health.
