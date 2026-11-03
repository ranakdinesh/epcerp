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

## Table-preserving tender evidence — implementation in verification

The working slice adds native PDF cell geometry and atomic table rows to derived
TSC evidence, with on-demand enrichment of authorized legacy PDF readings.
Per-question Gemini original-PDF research requires explicit confirmation and the
new `tenders.tsc.originals.write` permission. See
[TSC_TABLE_PRESERVATION_SLICE.md](TSC_TABLE_PRESERVATION_SLICE.md) for bounds,
records, acceptance gates and limitations. Go/native integration and frontend checks/build passed; the production reader
passed the supplied-PDF offline regression. Server rollout remains pending. Next
unfinished task: deployment validation, held-out benchmarking and an authorized
user’s live Gemini review; do not infer universal extraction accuracy.
