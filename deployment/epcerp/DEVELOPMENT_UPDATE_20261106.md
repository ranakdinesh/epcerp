# Development update — 6 November 2026

Updated the confirmed development server at `vtplerp@122.160.156.123` from
clean schema 101 to clean schema 102. The deployed application is the current
Next.js/Node runtime; no Go service was restored.

## Source and verification

Source: `codex/development` commit `62aa01e` plus its current tracked/untracked
working changes and six verified deployment fixes. All fetched remote branch
tips were ancestors of the packaged commit. The original working tree was
preserved; no merge, commit or Git push was performed for this deployment.
Snapshot SHA256: `1cb252450241232037ed582e0beb74bdbf171da4247209a720c88a11b7a8f68a`.
The complete source manifest is retained privately with the server release.

The snapshot includes the pending company eligibility/profile and tender
bidding changes, revised document selection/reading and TSC format editor,
profile persistence/provider configuration repairs and shared business
authorization guards. Deployment fixes corrected React effect/type/shortcut
lint blockers and migration102's new hash/table-count verification. Only those
six files were copied back after proving there were no concurrent source edits.
This is an operational deployment record, not complete business acceptance of
all pending eligibility and bidding workflows.

Passed: lint (zero errors, 24 warnings), TypeScript, 244 application tests,
48 injected-provider TSC tests, 14 native workflow/migration tests, and the
business-authorization concurrency/revocation test. Production builds passed.
Linux AMD64 password hashing and image conversion passed offline in the image.
Browser plugin not available: bundled Playwright tested synthetic login,
company profile tabs and adding an unsaved TSC-format question on desktop and
390px mobile, without runtime errors. Local QA used synthetic data with cloud
and email disabled. No live paid model request or real message was submitted.

## Migration, deployment and recovery

Current image for both web frontends and the one-shot schema service:
`epcerp-frontend:dev-20261106-1-amd64`. The unchanged private document processor
remains `epcerp-document-processor:retire-72215c9`; its runtime source is unchanged.
Image archive SHA256:
`a8a1c90a6eb89ac7e723eb30e75cf9d21e7363cfab7cb95e78396fe43a8c71e6`.

Migration102 adds seven company/eligibility/bidding tables. Its rehearsal on a
private copy preserved all 192 pre-existing tables exactly. Both the new image
and recovery image passed readiness against the upgraded copy. Recovery image
`epcerp-frontend:rollback-20261106-102-amd64` uses the preceding committed
application with the additive migration, avoiding a database downgrade/restore.
The rehearsal container uses explicit listen settings and the app's private
networks, excluding its cloud network.

Live writers and object storage were stopped for a consistent final backup.
The custom PostgreSQL dump passed listing and full decoding; original-document
and object-storage volume archives were verified against every regular file's
SHA256 (40 original-document files and 49 object-storage files including metadata). Live migration again preserved the 192 existing table contents before
application startup. Existing encryption/database credentials, provider keys,
approval and allowance settings were compared before/after and retained.
The Compose gateway, TLS, office CIDR rules and named volumes were preserved.

Private server records and recovery tools:
`/home/vtplerp/deployment-20261106/`, with verified backup
`backup-20261106T060802Z`. No live database was reset, restored or downgraded.
The script restores an appropriate application configuration on failure while
retaining the upgraded database. Credentials and backup data are not in Git.

The boot hook still referenced the retired Go API. It was backed up and
corrected to start only services present in the selected release, preserve all
private provider env files and exclude migrations on boot. The `epcerp-dev.service` restart passed after rollout and the service remains
active and enabled. Both web frontends retained the new image. A full host
reboot is not part of this deployment check.

Public HTTPS readiness returned 200 from outside the server. Both public HTTPS
and office HTTP readiness also passed from the server during rollout.
