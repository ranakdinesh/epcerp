# Tender table evidence and explicit original-PDF review

Implementation scope, 3 November 2026. Status: verification in progress.

## Business requirement

Outcome: prevent tender facts from being paired with adjacent columns or losing
wrapped scope text, and give reviewers a controlled way to inspect source PDFs
with Gemini when retained evidence is incomplete. Primary users are tender
preparers and reviewers. Trigger: upload/read source attachments, generate a TSC,
then investigate an unresolved question. Human acceptance remains mandatory;
AI never decides eligibility, commercial precedence or GST basis.

Records of truth are immutable originals and their verified hashes. Structured
cells and AI proposals are derived evidence; an accepted TSC answer remains an
explicit reviewer edit with its existing history. Existing draft ownership and
TSC read/write controls apply. Sending originals additionally requires
`tenders.tsc.originals.write`, explicit request confirmation and an approved,
configured Gemini provider. Auth reconciles the permission registry normally.
Cloud reservation identity records source IDs/hashes, confirmation, model and
question, without embedding document bytes in audit records. Result encryption,
source/version fencing and permission rechecks apply before and after inference.

## Delivered implementation being verified

- Isolated reader uses pinned pdfplumber for native ruled tables. Page/cell
  coordinates, row/column spans, headers and units accompany atomic row evidence.
  Original page text and old readings remain intact. New snapshots can enrich
  authorized legacy PDF readings from their originals without rewriting history.
- Recognized labels pair only with cells in the same row. Unknown labels remain
  evidence. Plain unverified PDF table text cannot automatically provide facts.
  Complete scope text survives former paragraph limits. Optional nearby context
  can be omitted without falsely blocking a usable anchor; mandatory amendments
  and recognizable conditional continuations still produce review gaps.
- Per-question research offers explicit Gemini original-PDF review. Every eligible
  retained source attachment must be a PDF: 1–8 PDFs, total 50 pages/8 MiB,
  32,768 input tokens, 2,048 output tokens, 75-second provider deadline. Counted
  input and generation each receive the same inline package. Completed reference
  checklists remain excluded by existing policy. Unsupported/oversized packages
  fail before generation, never silently lose a source. No automatic paid retry.
- Each original request reserves INR 5 from the existing cumulative INR 100
  application allowance. This is conservative accounting, not invoice reporting.
  Replays reuse results/reservations. Uncertain outcomes retain their reservation.
- Quotes must map to the cited source PDF and page in retained evidence; fabricated
  aliases/pages/quotes fail closed. Proposals remain unaccepted. Unsupported
  numbers are flagged for review. A valid quote does not establish interpretation.

## Risks and explicit remaining scope

Ruled native tables are covered; borderless/scanned/rotated/cross-page logical
structure is not universally reconstructed. No guessed cell geometry or automatic
cross-page merging. Vision-only text absent from retained OCR cannot yet be
accepted as a machine-verifiable citation. Originals fallback is PDF-only; DOCX,
spreadsheets and other source types require PDF versions. Referenced external
manuals are not fetched. Conflicting conditions remain reviewer decisions.

Acceptance gates: synthetic varied-layout/cell tests; unknown GST and adjacent
column regressions; consent, ownership, revocation, source hash, replay and budget
checks with fake cloud transports; native Go database integration; architecture,
race, frontend checks/build and rendered consent-flow QA; offline supplied-PDF
regression. No real Gemini request is part of automated verification. Broader
held-out tender benchmarking and an authorized user's cloud result review remain
unfinished. Existing job status/diagnostics expose gaps and provider failures;
no new monitoring service is introduced in this slice.

## Verification record

- `GOTOOLCHAIN=go1.26.3 make -C backend check build verify` passed: sqlc vet/diff,
  gofmt/vet, race tests, architecture boundaries, binaries and Go-owned native
  database tests (integration 176.7 seconds). Legacy PDF enrichment uses a fake
  layout port and proves stored reading artifacts are unchanged.
- Four offline synthetic PDF layout regressions passed: centred/wrapped values,
  paired columns/headers/merged cells, clipped continuing borders and borderless
  fail-safe behavior. Pinned helper dependencies are image-local tools; the
  Python client project and database are not dependencies.
- Frontend lint passed with four existing warnings; TypeScript and all 232 tests
  passed. Production build passed with `EPCERP_API_URL=http://api:8001/api/v1`.
- Playwright fallback was used because the Browser plugin is unavailable. On
  `http://127.0.0.1:3001/qa-tsc`, a temporary fixture using the real research
  component and a mocked API passed desktop/mobile consent, request payload,
  consent reset, package-error recovery and page-error checks. The temporary
  route was removed before the production build. Screenshots are private local
  QA artifacts, not customer documents. The provider label uses its own line.
- The supplied 13-page PDF was read offline in the Linux production reader
  image with networking disabled: 21 table regions, eight expected candidate
  fields, full scope preserved, adjacent contract category excluded, unknown GST
  basis left unresolved. The PDF and extracted contents are excluded from Git.
- Original-PDF provider tests use fake HTTP transports. They check complete inline
  PDF packages, token limits, hashes, source/page/quote validation, no generation
  after oversized token count, dedicated permission, confirmation, live
  revocation and idempotent replay. No paid cloud call was made.

The completed verification supports this bounded slice, not universal extraction
accuracy. Held-out packages and a user-reviewed live Gemini proposal remain next.
