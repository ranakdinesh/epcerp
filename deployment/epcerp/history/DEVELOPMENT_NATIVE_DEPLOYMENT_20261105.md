# Development server native runtime deployment — 5 November 2026

Deployed the tested Next.js/Node application to the confirmed development server
`vtplerp@122.160.156.123`, preserving public HTTPS at
https://erp.vishaltechnopower.in and office access at http://192.168.1.50.
No changes were merged into main or pushed to Git.

## Deployed source

Application commit: `72215c9788b40b79167c95bb4349ddff4e9cd939` on the combined
`codex/development` checkout, plus the same 62 recorded concurrent Tender/provider
files tested in the local migration. Every fetched remote branch tip was checked
as an ancestor of the packaged application commit. Source overlay hashes and
image identities are in the [verification record](DEVELOPMENT_NATIVE_VERIFICATION_20261105.json).

The AMD64 frontend was built from the verified standalone output with AMD64
runtime dependencies. Native password hashing, image conversion, login and 14
application routes passed an isolated AMD64 container check before transfer.
The private document processor is the same verified AMD64 Node image. The release
archive SHA-256 was checked on the server before images were loaded.

## Data preservation and checks

The server started at clean schema 98. Before changing live services, a private
MVCC database copy was restored and upgraded to schema 101 with the Node CLI.
All 185 original tables remained unchanged through the migration. Twenty native
HTTP checks passed on that private copy, including decryption of retained TSC
data and document-format PDF rendering. Cloud and email delivery were disabled
for rehearsal. The retained Go API was also started against the upgraded copy
and passed health/authenticated checks, proving application rollback did not
require restoring or downgrading the database.

Live application writers were stopped before taking the final private backup.
The database archive was verified through pg_restore listing and full decoding.
Both original-file and object-storage volume archives were verified by comparing
every archived regular file against its source SHA-256. All six named database,
object, original-document, scanner and TLS volumes were retained.

The live migration used the same owned SQL history and left all existing table
contents unchanged. Final per-field auditing against the first live backup
confirmed all six users' password hashes, credential/security versions, account
flags, contact details and business fields were preserved. The only account
metadata change was the bootstrap administrator's timestamp/edit counter during
the temporary Go application rollback. Expected role edit counters and GET quota
counters also changed. No live database was reset, restored or downgraded, and no
business records were deleted. All 40 original files and 22 S3 objects remained
unchanged. Existing provider credentials, approvals, encryption keys and budgets
were retained. Verification made no paid provider requests or real messages.

Twenty live native HTTP checks passed after cutover. Public HTTPS readiness was
also verified from outside the server, office LAN readiness passed using the
actual office address, and the office-only CIDR restriction remained enforced.

## Final services

Running: frontend, office-LAN frontend, private Node document processor,
PostgreSQL, MinIO storage, ClamAV and the existing HTTPS gateway. The Node schema
service completed successfully at version 101. Unknown application routes cannot
fall back to Go (`EPCERP_GO_DISABLED=true`).

Removed from Compose and removed as containers, without removing their images or
volumes: Go API, document-reading worker, package worker, TSC worker, Go migrator
and local reranker. No separate Ollama service was running. Unrelated operating
system services were left untouched.

## Deployment recovery and records

The first cutover was automatically rolled back at the application level when a
loopback-based LAN check received the expected 403 from the existing office-only
access policy. The check was corrected to use the office IP; the retry passed
against the already-upgraded schema. Neither attempt restored or reset live data.

The historical release scripts are under [native-runtime](native-runtime/README.md).
They are specific to this reviewed schema/image transition. Private backup paths,
original Compose, credentials and runtime inspections remain on the server;
`native-cutover-backup-path.txt` in the existing release directory identifies the
successful backup. These private files are not committed. Original images remain
available for recovery. Temporary QA containers were removed; database copies and
all storage volumes were preserved.
