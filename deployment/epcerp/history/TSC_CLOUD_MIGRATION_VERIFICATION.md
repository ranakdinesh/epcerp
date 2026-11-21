# TSC cloud/manual migration — local verification

Verified on 2026-11-04 on `codex/development`. Application commit:
`940dbbdb4788d32a09f87fff50fa46da0f95ef79`. Runtime/test source fingerprint:
`b31ed7c958388e340eb69bce473b1a9e28770a1c5dc919b17bbe08feeb37e3d4`.

## Scope

Next.js owns the new TSC cloud/manual workspace, durable model tasks, answer review,
response retention, cost accounting and Word export. Gemini is preferred; OpenAI is
implemented and tested with fixtures. Manual writing remains available. Only verified
selected PDFs are submitted; Office sources need PDF export. No local OCR, local LLM
or retrieval is required for this path. Other Tender FMS processes stay in Go.

Each tender retains its own immutable input/model responses and separately versioned
manager answers. A subsequent proposal never overwrites the working answer. Model
page references require human checking. Historical Go TSC jobs and exports remain
retained. See [module structure and configuration](../../frontend/src/server/tenders/README.md).

## Feature verification (isolated QA)

- 16 focused TSC tests passed, using injected synthetic Gemini/OpenAI responses.
  Covers manual work without OCR/sources, source integrity/scoping, permissions,
  revoked actors, idempotency, immutable raw/parsed originals, manager corrections,
  independent refinement, optimistic editing, reviewed-answer reopening, audit history,
  Word export, uncertain requests without resend, and tender/daily budget fences.
- 105 existing Auth/Master integration and Go compatibility tests passed, including
  supplier documents, storage, Finance and purchasing eligibility. A separate QA
  storage bucket was used; no operational objects were used as fixtures.
- 232 existing frontend tests passed. TypeScript and SQLC compile passed.
- Focused Go Tender FMS, document-reading, application and owned-schema tests passed.
  Includes confirmation that the Go paid TSC commands are absent under the Next owner.
- Full frontend lint: no errors; four pre-existing warnings in login, purchasing
  image usage and shared radio validation. No new lint warnings in the TSC files.
- Production Next.js image built successfully. Browser tests on isolated port 59092
  exercised generation, manager correction, preserved originals, independent OpenAI
  proposal, raw JSON/Word downloads, unsaved question-navigation protection, history,
  mobile layout and browser errors (none).
- The exported synthetic Word document opened in LibreOffice, rendered to PDF, and
  its first-page layout was inspected. It contains the corrected manager wording.

The provider requests and token counts in these checks are synthetic. **No paid
provider calls occurred. Live model extraction quality on a real tender is not yet
benchmarked.** QA containers and temporary credential files have been removed.
Synthetic QA data remains isolated from the operational database/storage bucket.

## Combined local Docker smoke and preservation

Preview: [Tender FMS](http://localhost:9092/dashboard?screen=tender-fms).
All nine operational services are healthy. Schema **99**, clean. API/migrator,
frontend and document workers came from the integration checkout. The Go TSC worker
is idle under the new owner; document reading skips `tender_draft` only.

The local writer services were stopped before the additive schema upgrade. A private
custom-format database backup was checked with `pg_restore --list`, fully read with
`pg_restore --file=/dev/null`, and SHA-256 recorded before migration. No database
reset, migration-version forcing, password change or volume removal occurred.

Preservation compared **185 existing tables** and **22 operational storage objects**.
All existing business data, password hashes, role permissions, original tender jobs,
documents and histories are unchanged. The existing Auth bootstrap performed its
normal no-op superadmin role upsert: that role's **edit_version incremented once**.
Comparison against the backup confirmed this was the only changed existing field.
It did not change permissions or credentials.

Original encryption keys, JWT/storage credentials and provider approval/key settings
were checked before/after deployment and preserved. Operational native TSC job count
remained zero during verification; no live documents were submitted to providers.
Gemini is approved/configured on this preview, using the paid-tier estimate. **OpenAI
still requires its API key**; the adapter is available once configured. Free-tier
quota is not assumed to be free daily cash credits on a billing-enabled project.

Read-only combined-stack smoke covered health, session and same-origin protections,
retired Go TSC paid commands, retained Tender FMS endpoints, existing Master routing,
and matching owner flags. This is distinct from the isolated feature tests above.
No production deployment or merge into `main` was performed.

Private backup / detailed checks:
`/private/tmp/epcerp-tsc-before-20261104T120659Z`.

Browser artifacts (synthetic data):
`/private/tmp/epcerp-tsc-browser-proof/`.

## Integrated remote tips

All fetched branch tips below were confirmed ancestors of the tested application
commit before the combined-stack test:

| Ref | Commit |
| --- | --- |
| `origin` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/chor/auth` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/codex/development` | `f896cf1c5cc513c7721b4354a8441c102b1947c9` |
| `origin/codex/table-aware-tender-evidence` | `7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2` |
| `origin/codex/tsc-processing-guardrails` | `17868507ffaa76221dbb8574856a819149973cac` |
| `origin/main` | `756335819939beb72596e1200a7c9c510660f265` |
| `origin/tenderFMS` | `1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9` |

## Running images

| Service | Image digest |
| --- | --- |
| `api` | `sha256:cbc6972d24eecf0af2bc6673791b6363de0cfc8c2e26446ec3e8e23689c36a72` |
| `frontend` | `sha256:7349abee1227f42f57ba688e6b57e171538c8289d1f23139370a7943350b9458` |
| `document-worker` | `sha256:22041d1e17b0506cc491f6f122491e84b4e3f11565f33af0ba22ce466da5fa65` |
| `document-package-worker` | `sha256:22041d1e17b0506cc491f6f122491e84b4e3f11565f33af0ba22ce466da5fa65` |
| `tsc-worker` | `sha256:cbc6972d24eecf0af2bc6673791b6363de0cfc8c2e26446ec3e8e23689c36a72` |
| `tsc-reranker` | `sha256:da855a5365565ff8f56b2b595b8ac85015f064ba14d8555ddfcae1b76267a7c9` |

INR costs use saved provider usage/rates and a configurable FX approximation (currently ₹95/USD), excluding taxes, payment fees and account credits. Unknown usage retains a separately displayed allowance and is not presented as a zero charge. Defaults are ₹100 per tender and ₹500 per UTC day; historical Go reservations also consume the tender allowance.
