# Delivery backlog — local TSC increment

This checkout entry covers the 29 October 2026 TSC slice only. Other module plans
are not declared complete by it.

- [x] Add Go-owned zero-cost local extraction mode and migration 94.
- [x] Add narrow inference port, focused evidence-selection service and private
  pinned MiniLM adapter; preserve architecture boundaries and existing permissions.
- [x] Persist prepared evidence/diagnostics; use identical selected evidence for
  paid identity/provider; keep source/acceptance/audit/budget safeguards.
- [x] Add local review controls and complete focused/full native and synthetic UI
  checks, production builds and a real-model CPU smoke test.
- [x] Back up, migrate and update the combined local Docker preview.
- [ ] Next: representative real-package gold corpus and held-out accuracy/cost
  benchmark, including fee/EMD, exceptions, amendments and low-OCR cases.
- [ ] Measure complete 4-core/8-GB PC stack memory, contention and latency.
- [ ] Add semantic/multilingual retrieval only after measured benefit.
- [ ] Complete reviewed referenced-clause/amendment applicability and table context.
- [ ] Implement reviewed company evidence and separately authorised finalisation.
- [ ] Add persisted provider token usage, versioned rate accounting and invoice
  reconciliation before claiming measured commercial savings.


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

## Table-preserving tender evidence — 3 November 2026

- [x] Implement native ruled-table cells, spans, headers and atomic row evidence.
- [x] Implement local-first facts and explicit permission/consent-gated per-question
  Gemini original-PDF review, bounded package, allowance and verified citations.
- [x] Complete full native/frontend verification and development deployment,
  including offline image preflight, backups, schema/route checks and an enabled
  deployment-service restart.
- [x] Bound synchronous research context/response deadlines around provider work;
  verify with a real short-timeout HTTP server and rerun Go checks/builds.
- [ ] Benchmark held-out native, scanned, borderless and cross-page tender packages.
- [ ] Review a real Gemini original-document proposal with an authorized user.

See TSC_TABLE_PRESERVATION_SLICE.md. Originals fallback is PDF-only and does not
fetch external references or resolve source precedence automatically.

## Development deployment — 6 November 2026

- [x] Package the combined current native working snapshot and correct deployment
  blockers while preserving concurrent source changes.
- [x] Pass native/frontend/provider-isolated checks, production build and browser
  smoke tests. Rehearse 101→102 and recovery on a private server database copy.
- [x] Take verified DB/business-file backups, migrate and update both development
  frontends while preserving private controls and existing LAN/HTTPS routes.
- [ ] Complete representative company-qualification and bidding workflow business
  acceptance, plus an explicitly authorized live provider quality evaluation.

See `deployment/epcerp/DEVELOPMENT_UPDATE_20261106.md` for the actual snapshot and
checks; deployment does not mark every pending business workflow complete.
- [x] Correct the retired-API boot-hook reference, then verify the enabled native
  deployment service restart, provider settings and both access routes.

## Tender bid workflow (9 November 2026)

- [x] Implement the bid workflow screens, routes, and migration 103.
- [x] Verify the native tender-to-project handoff with synthetic records,
  including concurrent award requests and a reviewable purchase BOQ.
- [x] Deploy the verified revision to development at schema 103 and check
  backup integrity, provider configuration, access routes and service restart.
- [ ] Complete catalogue matching and procurement acceptance on a representative
  awarded tender, and validate role assignments with operational users.

## Tender intake and procurement administration (10 November 2026)

- [x] Verify tender draft rename and audited removal with synthetic records.
- [x] Verify that Superadmins can set a confirmed password immediately, with
  session revocation and an audit entry. Reject requests for email-only resets.
- [x] Verify new Purchase FMS workspaces can start unassigned and notify eligible
  procurement users.
- [x] Rehearse migration 104, back up the development data and publish revision
  `c8b0ac4`. Check office HTTP and public HTTPS readiness.
- [ ] Exercise the handoff and role assignments with operational users on a
  representative awarded tender.
