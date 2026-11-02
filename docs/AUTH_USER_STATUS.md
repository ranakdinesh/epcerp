# Immediate account access revocation and deferred handover

Account status is an independent administrative security action. From Users,
choose Deactivate or Reactivate and record a reason. The status action does not
require completed personal details, assigned roles, a reporting profile, manager
approval, or any source module's work inventory.

`POST /api/v1/users/{user_id}/status` accepts `is_active` and `reason` (10–1000
characters). It requires `admin.users.write`; only a superadmin may change a
superadmin's status. Self-deactivation and removal of the last active superadmin
remain blocked. The legacy `/deactivate` endpoint uses the same security flow.

A status transition locks the user, advances the credential version, revokes all
refresh sessions, and writes immutable status evidence. Deactivation also opens
one pending handover if no open case exists. These changes commit atomically.
Existing access tokens fail the live account/version check. Reactivation requires
fresh sign-in and does not revive old access tokens or refresh sessions.

Pending handovers may have no manager and no inventory yet. Administrators can
correct an inactive employee's reporting profile in Organisation, then load the
inventory in Handovers. Managers select eligible replacements; work is transferred
in its source workspace. Approval and completion retain the existing checks on
current assignments, versions and recipient eligibility. Completing or cancelling
a handover never changes account access. Historical actors/documents stay intact.

Migration 95 belongs to Auth. It permits pending cases without a manager and adds
immutable `auth.auth_user_status_changes`. Downgrade is allowed on an empty
fixture, but refuses to erase retained status evidence or pending cases.

## Verification on 29 October 2026

Combined checkout: `codex/development`, base `75d3102`, with the uncommitted Auth
changes and pre-existing tender work. Fetched remote tips `origin/chor/auth`
`756335819939beb72596e1200a7c9c510660f265` and `origin/tenderFMS`
`1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` are both ancestors of the base.

Feature checks passed:

- Auth unit tests, architecture boundaries, and schema metadata tests.
- Isolated PostgreSQL tests for immediate access revocation without reporting
  setup, refresh/session revocation, reactivation without session revival,
  concurrent retries, authorisation, legacy endpoint compatibility, immutable
  evidence, and rollback on audit failure.
- Delayed handover tests with live work transferred while the user is inactive,
  including reporting setup after deactivation and completion without reactivation.
- Migration lifecycle and canonical contact editing tests, at version 95 clean.
- TypeScript and targeted lint; all 226 frontend tests.
- Playwright against an isolated Next.js fixture with synthetic records at
  1280×900 and 390×844: standalone status actions without profile/role requirements,
  pending handover, cancellation leaving access inactive, inactive profile edit
  availability, reactivation, and no runtime/console errors. Browser plugin was
  unavailable. It may be installed for future in-app browser QA.

Combined-stack deployment checks are separate from the feature tests:
API/migrator, frontend, and workers were built from the same checkout. Writers
were stopped and private database/object-volume/key backups verified before the
preview upgraded from 94 to 95. All nine local services are healthy. Credentials,
encryption keys, persistent volumes, and provider settings were preserved.
Authenticated post-deployment browser smoke was not completed because the saved
bootstrap credential was rejected; no password was reset or changed. No real
user status was modified for testing. Production was not deployed.

Private deployment manifests and backup verification are under
`deployment/epcerp/backups/auth-status-20261029T152601Z/` (gitignored). They record
source hashes and the running image identities. This local deployment does not
mark the changes reviewed or ready to merge into main.

The subsequent Auth security completion is documented in [AUTH_COMPLETION.md](AUTH_COMPLETION.md). Migration 96 adds session tracking, audit history and email verification while retaining the immediate-deactivation/deferred-handover behavior.
