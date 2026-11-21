# Recorded development cutover

These root-only scripts record the 5 November 2026 `epcerp-dev` transition from
schema 98 and the Go application to the verified schema-101 Next.js/Node images.
They contain strict release/project/image checks and are not a generic future
release command. Read the deployment record and inspect current runtime state
before adapting them.

- `prepare.py`: checks image archive integrity, preserves the current routing and
  credentials, prepares native Compose, restores a private database copy, verifies
  the additive upgrade and runs native HTTP checks with cloud/email disabled.
- `check-rollback.py`: tests the retained API against the upgraded private copy,
  without running the old migrator or changing the live application.
- `cutover.py`: stops writers, verifies database and full storage backups, applies
  owned Node migrations, starts native services, checks data/keys/HTTPS/LAN and
  removes retired containers only after success. Application rollback preserves
  the database at its current version; there is no force/reset/downgrade/restore.

Runtime credentials and private Compose snapshots remain owner-only on the
server. Never print expanded Compose environment values. Original encryption
keys, volumes, TLS configuration and provider controls must be preserved. No
script deletes a database or removes a volume.
