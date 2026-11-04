# Native TSC cloud/manual workflow

This module owns the TSC workspace only. Tender intake, upload safety verification,
format administration, assignment, provisioning, supplier and purchasing processes
retain their existing owners. `EPCERP_TSC_OWNER=nextjs` selects this writer.

- `documents.ts`: current permissions, owned tender, verified original document bytes.
- `tsc/service.ts` and `http.ts`: scoped commands, optimistic edits, retained history,
  original-response downloads and Word export.
- `tsc/worker.ts`: PostgreSQL tasks, expiring claims, live authorization/source checks.
- `tsc/providers.ts`: fixed-origin Gemini Files/generateContent and OpenAI Files/Responses.
- `tsc/pricing.ts`: versioned rate cards, token accounting, INR estimates and allowances.
- `tsc/repository.ts` and `crypto.ts`: encrypted persistence and append-only evidence.
- `tsc/validation.ts`: request, source, answer and citation boundaries.

Migration 99 belongs to the existing Go migrator. Apply it before starting matching
services; the Next.js runtime never creates or upgrades schema.

## Data and review

The immutable snapshot identifies one tender, format version, selected source IDs
and hashes. Verified PDF bytes are sent directly to the selected provider without
waiting for OCR, local models or local retrieval. No response cache is shared
across tenders. Up to 30 explicitly selected PDFs / 40 MiB / 1,000 pages are supported.
Office files remain retained but must be exported to PDF for this cloud path.

Each received model response is encrypted and retained exactly as received, with
its parsed answers, usage, provider/model, request ID and saved pricing. Database
triggers prohibit changing responses, source snapshots and answer-history events.
Manager edits update a separate encrypted answer set and append a versioned history.
Question refinement adds a proposal and never replaces the working answer. Reviewed
answers must be reopened before their wording can change. Model quotes/page references
are source/page checked but are not independently verified quotations. Management
judgement questions are excluded from model requests. A TSC does not approve a bid.

A normal 70-question format uses one model request; larger formats use batches of
80 questions and reuse uploaded files within that task. Optional stronger Gemini
processing handles at most 20 unresolved questions once. Failures, rate limits and
uncertain outcomes never trigger an automatic paid retry or provider switch.

A crashed task is recovered to manual review after lease expiry; uncertain requests
retain their allowance. Received output is preserved even when source/permission
changes prevent applying it. OpenAI files expire after 24 hours; both adapters also
attempt deletion after processing. A failed deletion is logged without document data.

Historical Go jobs and their original exports remain available through explicit
legacy ownership lookup. Their encrypted results and events are untouched. Go TSC
creation/research and the Go TSC worker are disabled under the Next.js owner. Local
reading claims skip `tender_draft` under this owner; other document-reading modules
continue unchanged. Switching ownership is an operational fallback, not a data merge.

## Configuration and cost

Keep keys server-side: existing `EPCERP_TSC_CLOUD_APPROVED`,
`EPCERP_TSC_GEMINI_API_KEY`, `EPCERP_TSC_OPENAI_API_KEY` and
`EPCERP_DOCUMENT_READING_KEY`. Manual writing needs no provider approval or key.
Existing provider approval is preserved by deployment; local tests use synthetic keys
and injected responses, never paid requests.

Default models/rates checked on 2026-11-04:

| Model | USD / million input | USD / million output |
| --- | ---: | ---: |
| Gemini 3.1 Flash-Lite | 0.25 | 1.50 |
| Gemini 3.8 Flash, optional stronger pass | 0.75 | 3.75 |
| OpenAI GPT-6 Luna | 0.10 | 0.50 |

Sources: [Gemini pricing](https://ai.google.dev/gemini-api/docs/pricing),
[GPT-6 Luna](https://developers.openai.com/api/docs/models/gpt-6-luna),
[OpenAI file inputs](https://developers.openai.com/api/docs/guides/file-inputs),
[OpenAI file expiry](https://developers.openai.com/api/reference/resources/files/methods/create),
[Gemini files](https://ai.google.dev/api/files).

Cached tokens, Gemini thought tokens and OpenAI long-context price multipliers are
accounted for. Gemini 3.8 rates follow its published 2027-01-01 increase. Pricing
snapshots are retained with each request; existing history is never repriced.

`EPCERP_TSC_USD_INR` defaults to a **configurable approximation of 95**, dated by
`EPCERP_TSC_FX_DATE` (default 2026-11-04), not a live exchange-rate feed. Estimates
exclude taxes, payment fees and account credits. Usage missing from a response is
shown as unconfirmed, with its conservative hold separately displayed. Costs use
micro-INR internally to avoid rounding small calls to zero.

`EPCERP_TSC_GEMINI_TIER` defaults to `paid`. Set `free` only for a project actually
using the provider's free tier. Daily quotas are not guaranteed cash credits, and
billing-enabled usage must not be assumed free. Free-tier selection never automatically
falls through to another paid provider.

`EPCERP_TSC_TENDER_LIMIT_PAISE` defaults to 10000 (₹100 hard maximum, shared with
historical Go reservations). `EPCERP_TSC_DAILY_LIMIT_PAISE` defaults to 50000 (₹500,
UTC day). Requests reserve their model's maximum input/output cost atomically before
uploading, then release the excess when reported usage is known. These holds can be
much larger than the resulting charge. The limits are application allowances, not
provider invoice hard caps; update prices when provider terms change.

## Verification

Run `bash frontend/scripts/verify-tsc.sh` with `EPCERP_DATABASE_URL` targeting a disposable
`epcerp_auth_qa_tsc_*` database, after stopping any QA worker using that database.
The script applies owned migrations and uses synthetic credentials and injected fetch.
It refuses the operational database. Native Auth/Master compatibility checks use
`frontend/scripts/verify-master.sh` and a separate QA storage bucket for full coverage.

The fixture URL override additionally requires both `EPCERP_TSC_TEST_MODE=true` and
an isolated TSC QA database; only loopback or the named local Docker fixture is allowed.
It must never be configured on the operational preview or production.
