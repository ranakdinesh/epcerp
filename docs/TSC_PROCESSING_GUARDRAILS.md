# Bounded TSC draft processing

Business outcome: a stalled local model or cloud provider must not leave an
uploader waiting through repeated invisible processing attempts. Existing draft
owners/reviewers use the same Generate TSC action and permissions. Source originals,
immutable input snapshots, encrypted results and append-only job/cloud events remain
the records of truth. No new commercial decision or automatic approval is added.

Controls:

- Local ranking has a five-second end-to-end budget, including CPU-slot acquisition.
  Failure opens a one-minute process-local circuit; other questions retain bounded
  BM25 source evidence with an explicit warning instead of repeating the wait.
- The private model server serves health independently, accepts one inference at a
  time and rejects concurrent inference. A ten-second native-inference watchdog
  exits the isolated container (70); Docker's restart policy reloads the pinned
  model. No document, prompt, token or key is logged.
- A draft attempt has a twelve-minute useful-work budget. Deadline completion uses
  a fresh fifteen-second context, rechecks current authorization/source versions
  and preserves repository claim/lease fencing. Completed answers are retained;
  unfinished answers remain empty, unapproved and explicitly require source review.
  Worker shutdown is persisted as processing_interrupted rather than left spinning.
- Provider HTTP 401, 403, 429, other 4xx and unavailable/timeout responses have fixed
  safe categories. Provider response bodies never become errors or logs. After a
  rate/access/auth/availability/timeout failure, further cloud requests and allowance
  reservations pause for the rest of that attempt. Local evidence remains available.
- Existing cloud approval, exact evidence fingerprints, live permission checks,
  encryption and cumulative INR100 conservative allowance remain. Paid calls are
  never automatically retried. Ambiguous outcomes retain their allowance.

Acceptance: cancellation/deadline tests must prove durable completion and retained
source review, including revocation during processing; blocked ranker tests must
prove a bounded first wait and immediate subsequent fallback plus later recovery;
provider tests must prove safe classification and no extra calls/reservations;
sidecar tests must prove busy rejection, watchdog termination and health availability.
Native database/architecture/race checks and production-image smoke checks must pass.

This change does not alter schema, replace source data, approve answers, restart
failed paid operations or establish tender-answer accuracy. Existing historical
failed jobs are retained. Users deliberately request a new generation/research if
required; no operator resets attempts or rewrites audit history.


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
