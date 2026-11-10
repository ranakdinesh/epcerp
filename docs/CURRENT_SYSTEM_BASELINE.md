# Current system baseline — local TSC increment

29 October 2026. This entry inventories the local TSC increment in this independent
epcERP checkout; it is not a replacement inventory for all historical modules.

- codex/development combines current main/Auth baseline and Tender FMS branch.
- Go migration 94 adds local TSC job mode; immutable snapshots/history and source
  access remain owned by Tender and the reusable document services.
- Go deterministic labelled-fact extraction, local MiniLM reranking, selected-only
  model inputs, prepared per-question source evidence and review UI are implemented.
- Pinned offline INT8 model runs in a private CPU-limited Docker service.
- Native tests, synthetic browser checks and actual local-model integration passed.
- Local Docker was upgraded with a verified database backup. Existing user, role
  and client counts were preserved. No operational data or staff were seeded.

See GO_TSC_LOCAL_RAG_SLICE.md for exact scope, verification and limitations.
Real-tender quality, target-PC capacity, company matching, complete clause/table
understanding, final approval and actual model billing measurement remain unfinished.
Next task: annotate representative real tender packages and benchmark held-out
clause coverage, critical errors, reviewer effort, actual usage and target memory.


## TSC processing guardrails — 3 November 2026

Implemented in the independent Go source based on codex/development f896cf1
(matching the deployed schema98/backend tree). See TSC_PROCESSING_GUARDRAILS.md.
Five-second local ranking budget plus one-minute circuit fallback; responsive
private model health, single-flight inference and ten-second native watchdog;
twelve-minute draft budget with separately bounded, authorization/claim-fenced
completion; retained partial answers with explicit source-review warnings; safe
provider error categories and per-attempt cloud pause on rate/access/availability
failures. Existing INR100 reservations, no automatic paid retries, encryption,
source/version checks and human acceptance rules remain.

Verification passed: full SQLC/gofmt/vet/architecture/race checks, production Go
builds, native disposable PostgreSQL integration (166.903s), database (3.186s) and
schema (8.764s) suites. Four new application guardrail scenarios plus failure-code
checks, adapter deadline/circuit/provider-code regressions, and three Python
single-flight/watchdog tests passed. Native server image preflight passed health
while blocked, immediate busy rejection, watchdog exit70 and real pinned-model
synthetic EMD relevance. No private-document upload or cloud generation was used.
Deployment is underway using a verified image archive with private configuration
and database rollback backup. No migration or frontend rebuild is needed; existing
UI renders terminal failures and partial-result warnings. Continuous per-question
UI progress/checkpoint persistence is not claimed by this change.

Remaining: live deployment smoke/health verification and a deliberately requested
real-document run by an authorized user; real-tender answer accuracy remains a
review responsibility. Historical failed/charged jobs are not reset or replayed.


Deployment verified — 3 November 2026: updated the existing dev-20261102-1-amd64
release to epcerp-api:guardrails-20261103-1-amd64 (API/TSC worker) and
epcerp-tsc-reranker:guardrails-20261103-1-amd64. Private custom-format database dump
and configuration rollback copy retained under
pre-tsc-guardrails-20261103T084211Z-75280; pg_restore archive listing validated.
New services are healthy, provider approval/key presence is retained, schema98 is
clean, and both LAN HTTP and external trusted HTTPS return200. Both frontend
images and public/LAN cookie settings are unchanged. Image archive SHA256:
e371cd7c564c800b36bef0756002cf57dbbfc8d8c6ac15f20e996771c46436b6.
Persistent Compose image settings and the enabled startup service retain the
update across restart. No full host reboot or real-document paid run was used
for verification. Historical jobs remain four ready/three failed, with no manual
reset/replay. Remaining acceptance: owner-requested real-tender run and review of
fallback/partial-result warnings, not automatic commercial acceptance.

## Table-preserving tender evidence — deployed 3 November 2026

The working slice adds native PDF cell geometry and atomic table rows to derived
TSC evidence, with on-demand enrichment of authorized legacy PDF readings.
Per-question Gemini original-PDF research requires explicit confirmation and the
new `tenders.tsc.originals.write` permission. See
[TSC_TABLE_PRESERVATION_SLICE.md](TSC_TABLE_PRESERVATION_SLICE.md) for bounds,
records, acceptance gates and limitations. Go/native integration and frontend checks/build passed; the production reader
passed the supplied-PDF offline regression. Development rollout, verified database/config backups, native image preflight
and enabled deployment-service restart passed. API/worker image revision 2
includes a bounded research response lifetime around the 75-second provider
limit. Next unfinished task: held-out benchmarking and an authorized user’s live
Gemini review; do not infer universal extraction accuracy.

## Development deployment — 6 November 2026

The current native Next.js/Node working snapshot was deployed at clean schema 102.
The migration rehearsal and live upgrade preserved 192 existing tables; verified
database and business-file backups are retained privately. The new and recovery
images passed rehearsal readiness. Source, checks and limits are recorded in
`deployment/epcerp/DEVELOPMENT_UPDATE_20261106.md`. This records operational
delivery of the current UI/schema; company qualification accuracy, live-provider
quality and end-to-end bidding acceptance remain separate unfinished validation.

## Tender bid workflow and handoff (9 November 2026)

The native workflow now has screens and routes for document intake, bidding
approval, pre-bid queries, working BOQ, vendor quotation selection, costing,
submission, and award. Migration 103 adds the four bid workflow tables.
Award uses the authenticated project and purchasing permissions, requires a
recorded submission and an exact active client match, and transfers tender
items into a draft purchase BOQ. Procurement completes catalogue and date
review in its existing workspace. The transaction links the award to the
created project so repeat requests reuse that project.

Synthetic database verification covers pre-bid entry, L1 selection, costing,
submission, concurrent award requests, and the transferred draft BOQ. TSC
provider simulations cover fresh generation, interruption, and retained
response evidence. Next unfinished task is tender-to-procurement business
acceptance with representative user roles and tender data.

The development server runs application revision `b79adc0` at clean schema 103.
The rehearsal and live migration preserved all 199 existing tables. The database
archive and both business file backups verified, and the deployment service
restart retained working office HTTP, public HTTPS and provider configuration.
The server rollout record is `/home/vtplerp/deployment-20261109/verification.json`.

## Tender intake and procurement administration (10 November 2026)

Application revision `c8b0ac4` introduced schema 104 on development.
Tender draft rename and removal retain audit events. A Superadmin sets a new
password immediately by submitting it with matching confirmation. The reset
revokes existing sessions and records a credential audit entry with either
the supplied reason or the default audit reason. The reset clears the account's
sign-in lockout and queues no email.
When email delivery is unavailable, administrators create users with a
confirmed temporary password that expires after 24 hours. The user must
choose a personal password at first sign-in. Configured email delivery
continues to support welcome invitations.
Purchase FMS workspaces can start unassigned, and project creation records notices for active users
with purchase assignment permission. Auth initializes the Procurement Manager
role after registering its permissions.

The AMD64 release and recovery images built successfully. Verification covered
247 frontend tests, 48 simulated TSC tests and 18 native workflow and migration
checks. The rehearsal and live migration preserved the rows in all 203 existing
tables. The private database archive and business file backups passed checks,
and both the office HTTP and public HTTPS readiness endpoints returned 200.
The server rollout record is `/home/vtplerp/deployment-20261110/verification.json`.
Representative user acceptance for the tender-to-procurement workflow remains open.
