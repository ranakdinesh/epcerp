# Independent local epcERP Docker stack

## Development code synchronisation (2 November 2026)

Use `codex/development` for the combined development source. This branch includes
the Auth/main baseline `7563358`, Tender FMS `1c7c4f6`, local MiniLM extraction
`75d3102`, and the subsequent Auth security, extraction-v5 and HTTP upload fixes.
The source migration head is **96**; the running database version must be checked
separately. Auth upgrade details are in `docs/AUTH_COMPLETION.md` and
`docs/AUTH_USER_STATUS.md`. Migration 96 requires everyone to sign in again.

Pushing source does not update an existing deployment. On another machine, fetch
and check out `codex/development`, provide its private environment configuration,
and follow the backup/migration/matching-image procedure below. Never copy keys or
credentials into Git, reset persistent volumes, or run older images against a
newer schema. Cloud approval remains false by default. `main` remains the reviewed
release target; development synchronisation is not production acceptance.

Verification on 2 November 2026 used application commit `fea00c5` (with shared
agent tooling at `b059859`). Both fetched feature tips listed above are ancestors
of that checkout. Backend `make check`, `make build` and `make verify` passed,
including disposable PostgreSQL migration and API checks with race detection.
Frontend: 228 tests, TypeScript, ESLint (zero errors; four warnings), and the
production build passed. The build used
`EPCERP_API_URL=http://127.0.0.1:8001/api/v1`. Shared skill scripts passed Node syntax
checks. Changed/staged source scans found no private-key/provider-token patterns;
environment files and backups were excluded. No Docker-stack smoke check or
authenticated browser rerun was performed for this sync because the Docker daemon
was unavailable. The deployed database version was not verified or changed.

## TSC draft worker

Migration 92 and matching API/frontend images provide unapproved 70-question TSC
drafts in each document draft's reading panel. Start the separate `tsc-worker`
service alongside existing document workers. Manual mode works without an LLM;
all retained sources must finish reading before generation. The Auth registry
adds tenders.tsc.read/write/export without expanding ordinary roles.

Optional development inference requires an already installed local model with
`EPCERP_TSC_MODEL` set to its exact tagged name and `EPCERP_TSC_MODEL_DIGEST` to its
SHA256 weight digest. `EPCERP_TSC_OLLAMA_URL` defaults to Docker's local host alias;
no model is downloaded and no cloud fallback is enabled. Missing model configuration
returns a visible error; users can explicitly choose manual mode. Preserve the
existing document-reading encryption key for TSC snapshots too.

The API image includes `/app/tsc-worker`. It shares the database-backed heavy job
slot, uses bounded claims/attempts and retains restart recovery. A new source
revision blocks editing/export of the previous snapshot. Exported DOCX files are
drafts; company eligibility, actual BOQ population and final approval are separate.
See `docs/GO_TSC_DRAFT_SLICE.md` for limits, test evidence and pending work.

## Synthetic company AI evaluator

The standalone `tsc-test.compose.yaml` builds the company/TSC fixture runner in a
non-root, read-only, resource-bounded container. It has a separate network/results
volume, no application credentials or database mounts, and no published ports.
It does not add a Common administration or TSC generation screen to the frontend.

From the repository root:

```powershell
docker compose -f deployment/epcerp/tsc-test.compose.yaml build
docker compose -f deployment/epcerp/tsc-test.compose.yaml run --rm tsc-ai-test
```

The default command validates fixture data without inference. For an explicitly
selected model already running on the host, use `--ollama-host host.docker.internal`
with `--model`, `--case` and `--output /results/NEW_FILENAME.json`. See
`backend/testdata/tsc_company/README.md` for the complete command and result copying.
No cloud fallback, model download or operational seed is performed.


The object-storage image uses `storage.Dockerfile` in this directory. Its Compose
build context is `.`; the Go stack no longer needs `deployment/docker/`.

API/migrator and worker build contexts are `backend/`, which includes the embedded
`sql/migrations/` package within the backend's single Go module. The builder follows
the existing backend `go.mod` (Go 1.26.3). SQL remains embedded in the migration
binary; do not mount a separate migrations directory into running containers.

Open http://localhost:9092. This is a development preview with a fresh, persistent
PostgreSQL 17 database, Go 1.26.2 API/migrator and the independent VTPL CORE frontend.
It uses Compose project epcerp-local, its own network and postgres_data volume.
Frontend port 9092, PostgreSQL port 5439 and object-storage API port 9002 are
published on host loopback. The API has no host port. Connect to Go PostgreSQL at
127.0.0.1:5439, database/user `epcerp`, using `EPCERP_DB_PASSWORD` from the private
`.env` file.

From the repository root, with Docker and OpenSSL available:

    ./deployment/epcerp/setup.sh
    ./deployment/epcerp/credentials.sh

Setup creates random local database/JWT/recovery-admin/object-storage credentials once, in the
ignored deployment/epcerp/.env file with mode 0600, then builds and starts the stack.
Never commit or share that file. Credentials are preserved during subsequent builds.
The credentials helper prints only the recovery login when you run it locally.
Username is superadmin. Business records start empty; no Python data is copied.
Use the directory screens to create active references/clients, then Projects to
register drafts or manage users/manager permissions. Manager handover needs
projects.master.read plus projects.manager_assignments.write. Assigned candidates
must have live project read/write permissions. Activation/operational workflows and
real notifications remain unfinished. Private document storage uses this stack's
independent MinIO service and storage_data volume; its API is exposed only on host
loopback port 9002 for native backend development.

All implemented API modules load by default; endpoint permissions still govern access.
EPCERP_DATABASE_URL points only to this stack's database. The migration service runs
the embedded Go migration history before the API starts; API startup never creates
tables. The frontend uses only http://api:8001/api/v1, independent epcERP cookies and
the explicit development HTTP cookie setting. Runtime images run as non-root users.
The explicit EPCERP_HTTP_ADDRESS=0.0.0.0:8001 listener is private to the Docker
network; the normal host API default remains 127.0.0.1:8001.

Operational commands from this directory:

    docker compose --env-file .env -f compose.yaml ps
    docker compose --env-file .env -f compose.yaml logs --tail=100 api frontend
    docker compose --env-file .env -f compose.yaml stop
    docker compose --env-file .env -f compose.yaml start

## Native IDE development

Run PostgreSQL and MinIO in Docker while starting the backend and frontend from the
IDE. From the repository root:

    ./deployment/epcerp/ide-infrastructure.sh start

This stops the Docker API, frontend and document worker to prevent duplicate app
processes, then starts and waits for only `db` and `storage`. It preserves both
volumes. Use working directory `backend` for `go run ./cmd/api/`, and working
directory `frontend` for `npm run dev`. The backend connects to PostgreSQL at
127.0.0.1:5439 and MinIO at 127.0.0.1:9002; the frontend connects to the native API
at 127.0.0.1:8001.

Rebuild/start with setup.sh after source changes. A normal stop/down preserves the
database volume. Do not remove volumes or regenerate credentials when retaining
data; reset is a separate destructive action. Local HTTP preview is not a production
TLS deployment. Frontend dependency audit findings inherited from the cloned
lockfile require separate dependency remediation before production acceptance.

## Me and workspace preferences

Use the circular avatar in the app bar to open Me, change your password or sign out.
Me edits display name, profile photo and email; changing email/password requires
current-password verification. Upload PNG/JPEG up to 512KiB and 2048×2048 pixels;
photos become square 256×256 JPEGs. Remove photo restores initials. The sun/moon
button changes theme. Desktop navigation collapses to icons; mobile opens a drawer.
Theme and sidebar preference are stored locally, without authentication tokens.
Password changes update sign-in credentials and revoke refresh sessions. Seed/test
credential files retain initial values; changing a password through Me does not
rewrite those files. Test credentials were restored after this slice's verification.

Opt-in browser regression runners live in frontend/scripts:
verify-personal-workspace.cjs (profile/name/photo restored) and
verify-personal-account.cjs (Director email/password changed and restored).
Run only against this local test preview: they deliberately exercise account writes
and sign test sessions out. They require Playwright/Chromium; set
EPCERP_PLAYWRIGHT_MODULE to an available Playwright package directory if it is not
installed locally, and EPCERP_TEST_CREDENTIALS_FILE to an alternate private test
credential file if needed. Screenshots are written to the OS temporary directory.
These runners are separate from the default helper/unit tests and never print
credentials. Go database integration tests use an isolated temporary database.

Personal profile ownership is `auth/user_profile`, stored in `user_profile`.
Migration 10 reversibly renames the former table and preserves existing profiles.
Frontend/API use `/api/auth/user_profile/...` and `/api/v1/auth/user_profile/...`.
The desktop collapse control is attached to the sidebar edge; mobile keeps the
header menu button. Rebuild with setup.sh to apply the migration and matching routes.

## Retired one-time imports

The old `data/` snapshots, reconciliation manifests and five `import-*.cjs`
operator scripts are no longer part of the active deployment. They are not used
by startup or normal application requests. On 23 October 2026 they were archived
under `deployment/backups/retired-imports-20261023-214020.zip`, with all 19 files
verified against their original SHA-256 hashes before removing the active copies.
The adjacent `.sha256` file checks the archive itself. Keep this private backup
for source-to-destination mappings and historical reconciliation; it is not a
PostgreSQL or object-storage backup and must not be used as automatic seeding.

Use normal Go application workflows for ongoing changes. Any future import is a
separately reviewed operation. The private `.env.test-users` file is retained for
the opt-in browser regression scripts, and `roles/` retains reviewed role definitions.
Preserve the database, object-storage volumes and all document/image encryption
keys independently; imported records no longer depend on these archived JSON files.

### Tender FMS document intake and reusable reader

After migration/deployment, Tender FMS is served at `/dashboard?screen=tender-fms` with authenticated, authorized access. Go migration 21 owns intake, queued readings, encrypted immutable originals/results and local evidence history. The dedicated `document-worker` uses only its internal database network, no internet/provider API. Its process receives no database credentials/encryption key, and applies memory/time/output limits. Local native PDF/XLSX/DOCX readers, English/Hindi OCR and headless-only LibreOffice support PDF, XLSX/XLS and DOCX/DOC. One source upload is at most 32 MiB; text/structured results are at most 8 MiB. Low OCR confidence requires comparison with the original. Legacy XLS conversion may recalculate formula values and warns explicitly.

`setup.sh` creates `EPCERP_DOCUMENT_READING_KEY` only if missing. Preserve this private key alongside database backups: original and reading ciphertext require it across rebuilds. Do not rotate it by editing the environment. Worker processes one document at a time; queued work persists in the database, interrupted claims recover after the 20-minute lease, and failed work has at most three attempts. Stop/restart the worker through Compose; never edit lease/status/results directly in an operational database.

Run `GOCACHE=/private/tmp/epcerp-go-cache ./backend/scripts/verify-document-reader.sh` after building the worker image to test the actual production reader binary, OCR and legacy converters offline in a disposable container. Fixtures contain synthetic test text and do not use any operational database. The normal Go check/verify gates cover hexagonal boundaries, owner/portfolio authorization, encryption, state/version rules, deduplication, lease fencing and transactional audit rollback. Tender-specific business-field/BOQ extraction is the next increment. Other-process integration, retention/capacity policy, advanced scan/table accuracy and richer progress remain future work.

## Purchase FMS project dashboard (migration 22)

`/dashboard?screen=purchase-fms` is the normal purchasing landing page. It lists
current purchaser assignments and PM oversight, or the explicit permissioned
portfolio. Selecting a project opens its scoped purchase menu. Available owner
assignment/history is independent of project-manager assignment. Purchasing view
and read are required; Auth reconciles assignment/portfolio permissions on startup.
The module is always registered in this isolated stack.

Rebuild the API, document-worker and frontend together, run the migrate service,
then start the matching images. Migration 22 adds purchasing workspace ownership
and immutable history; it does not import operational Python purchase records.
Source-derived BO/indent/sourcing/PO/inspection/delivery execution and progress are
subsequent migrations. Current empty execution displays “No purchase records”.
Reference steps are Planned. See `docs/GO_PURCHASE_FMS_DASHBOARD_SLICE.md`.

## Expanded Project Master capture (migration 23)

Projects → Create draft project now captures business/site/location, scope,
operations user and assigned PM, exact project value/currency, LOI/PO references,
basis/milestone/planned dates, duration/buffers and remarks. Saved values appear in
View → Project details. Location dropdowns need the existing location-reference
read permission. Project creation still requires live master read/write and
portfolio write; no new role-name bypass is introduced.

Deploy API/worker/frontend images together with migration 23. Existing minimal
drafts are preserved without dummy values; populated capture cannot be silently
rolled back. Working-calendar/holiday migration is pending, so net running days
are null rather than an inaccurate total. Source fields use exact decimal/date-only
string contracts. See `docs/GO_PROJECT_CAPTURE_ALIGNMENT.md`.

Client import removal (11 October 2026): rebuild API/worker/frontend and run the
migrate service before restart. Migration 59 supplies empty defaults for retired
client source columns so form creation works while historical provenance is kept.
The client/project legacy import scripts and routes are retired.

### Auth-owned profile images — 11 October 2026

Migration 60 adds image references without rewriting existing database photos.
`setup.sh` generates `EPCERP_IMAGE_ENCRYPTION_KEY` once in ignored `.env`; keep this
32-byte hex secret with backups and preserve it across rebuilds. New profile uploads
are processed by shared helpers and encrypted before private MinIO storage. Auth
owns metadata/audit transactions and authenticated reads; browser URLs remain
unchanged. Replaced/removed objects remain encrypted pending a separate retention
cleanup policy. See ../../docs/GO_AUTH_PROFILE_IMAGES.md.

### Offline transfers only — 18 October 2026

Supplier and manufacturer import tools no longer connect to a source application,
container, interpreter or database. They are optional explicit operator tools;
ordinary epcERP startup, build and workflows do not run them. Set
`EPCERP_SUPPLIER_IMPORT_FILE` or `EPCERP_MANUFACTURER_IMPORT_FILE` to a privately held,
reviewed JSON snapshot and optionally `EPCERP_FRONTEND_URL` (default
`http://localhost:3001`). Existing operator credentials and import permissions remain
required. Manufacturer input is the reviewed array of master rows; supplier input is
an object containing `suppliers` and `documents`, with each document's reviewed bytes
in base64 `content`. Retain source IDs/timestamps for mapping and idempotency. Scripts
verify copied values, stable source hash, repeat imports and downloaded file hashes
before writing reconciliation manifests. A source snapshot containing document bytes
is sensitive: keep it outside Git and use the existing private snapshot handling.
Historical source names in older snapshot files/manifests describe provenance only;
they are not live connections. No import or new approval was run during this repair.
# Local ingestion verification update (26 October 2026)

The local ingestion-v1 API, frontend and document-worker images include structured
source preparation and TSC retrieval diagnostics. Generate a **new** TSC with
Local AI proposals to apply normalization to retained readings; existing snapshots
are preserved. Inspect Source preparation and Evidence retrieved, and download
the source evidence Markdown alongside the draft DOCX. No migration is required.
See `docs/GO_TSC_INGESTION_IMPROVEMENT.md` from the repository root. Local rollback
images retain the `before-ingestion-v1` tag for all three services. Production
semantic accuracy/capacity acceptance remains pending; no cloud fallback is enabled.
# Low-cost TSC providers and question research (27 October 2026)

Source search is CPU-only and free of model charges. Production does not require
Ollama. The current development weights (20/40 GiB) cannot fit an 8-GiB server.
Keep `EPCERP_TSC_MODEL` and `EPCERP_TSC_MODEL_DIGEST` empty in production.

Cloud proposals use fixed HTTPS endpoints, no tools/web search, no automatic
provider switch or paid retry, at most 32 KiB/request and 2048 output tokens.
The allowlist is Gemini `gemini-3.1-flash-lite` and OpenAI `gpt-6-luna`.
OpenAI API billing is separate from a ChatGPT subscription.

Configure secrets in the private deployment environment/secret store, never in
Git or browser components:

- `EPCERP_TSC_GEMINI_API_KEY`: Gemini API key with an approved paid/private data policy.
- `EPCERP_TSC_OPENAI_API_KEY`: optional separate OpenAI API key.
- `EPCERP_TSC_CLOUD_APPROVED=true`: explicit approval for sending authorised
  retained source excerpts to configured providers. Default false; leave false
  until the organisation has approved the provider, document scope and retention.

Apply matching environment to API and TSC worker; recreate both after changing
configuration. Keys are never returned by the options API. Absent keys leave their
choices visibly unavailable. Manual completion and source search still work.

Migration 93 adds a transactionally reserved INR 100 ceiling per tender draft,
shared across all generations/research requests and source revisions. Each Gemini
request retains INR 1.50; OpenAI retains INR 0.60. These are conservative allowances,
not invoiced spend. Unknown/failed outcomes retain allowances and are not resent
on replay. Successful validated responses are encrypted and reused for the same
operation. Review prices/FX before enabling; the bounds use published standard
text pricing on 27 October and INR 100/USD. Larger/new models are intentionally
not configurable until their budget bounds are reviewed. Cloud OCR is separate
and is not enabled by this flag.

Question flow: open Generate TSC -> question -> Search sources for this question
-> optional extra terms -> free source search -> optional provider proposal ->
Use proposal in answer -> review citations -> check Accepted to save and lock.
Uncheck Accepted to reopen for editing/re-query; the application records the actor
and audit reason automatically.
No proposal auto-saves or approves a decision. Existing source/answer versions and
live permissions are checked before and after provider I/O. Completed-checklist
filenames are excluded from new TSC snapshots; historical snapshots are immutable.

Before migration, stop source writers and take/validate a private full backup.
Migration 93 preserves historical rows; downgrade refuses retained cloud evidence.
Native disposable tests cover upgrade/rollback, budget concurrency, encrypted
replay, request rollback, live revocation and stale sources. A full 8-GiB stack
load/ClamAV signature-reload test is still required before a client deployment;
an isolated OCR pass is not full-stack capacity acceptance.


## TSC review and export completion (27 October 2026)

The upload area supports visible Add documents, incremental file selection,
drag/drop, removal and the Upload action. Step 2 provides side-by-side question
editing and consolidated source Markdown, per-question source search/model
proposals, exact citations, answer filters and draft DOCX/source-MD downloads.
Accepted answers are persisted and locked server-side until explicitly reopened;
source changes invalidate editing/export of previous snapshots.

Generated answers request concise checklist lines, with explanations retained in
supporting evidence. DOCX cells contain actual supported answer text or remain
blank; status-only responses such as answered/Not found/Needs review are never
printed as answers, and future status-only answered proposals are rejected.
App theme switching controls dark/light utilities independently of OS preference.

Native Go tests cover scope, versions, rollback, encryption, worker leases,
accept/reopen audit, cloud reservation concurrency and encrypted replay. Frontend
types/lint/tests, production builds, desktop/mobile browser checks and rendered
DOCX verification were performed locally. Company test fixtures remain invented
and disconnected from operations. Real Common/company evidence matching, BOQ
population and final review/approval are unfinished.

An authorised Gemini comparison showed useful completion/pre-bid retrieval
improvements, but provider failures and fee-versus-EMD confusion remain limitations.
Reserved allowances are not actual invoices. Provider HTTP error classification,
request pacing and persisted token-usage accounting are pending; do not advertise
this sample as production accuracy or measured invoice cost. Operational sample
files, API keys and private local deployment configuration are excluded from Git.

## Combined local development preview

During development, build this stack from `codex/development`, which integrates
the current feature branches for application testing. Keep `main` as the reviewed
merge target. Fetch and integrate new feature commits before rebuilding; verify
that each intended remote branch tip is an ancestor of the integration checkout.
A healthy combined stack is a smoke check, not feature acceptance or merge approval.
Keep branch-specific tests and real-source TSC review as separate acceptance evidence.

The local integration established on 29 October 2026 includes Auth/main baseline
`7563358` and Tender FMS `1c7c4f6`, with Go migration head 93. It requires the API,
frontend, document reader, document-package worker, TSC worker and ClamAV scanner.
The pinned ClamAV image is amd64-only; Compose explicitly selects that platform
so Docker Desktop can emulate it on Apple Silicon. Initial image/signature downloads
and scanner startup may take several minutes; package processing waits for scanner
health. Scanning remains required.

Use the existing `setup.sh` to rebuild the checked-out integration code. Preserve
credentials/encryption keys and volumes, stop application writers and verify a
private backup before schema upgrades. Do not run older main images against the
newer integration schema without a separately planned compatible restore/rollback.
Paid-provider/private-source transmission controls remain opt-in; combining code
does not enable providers or prove end-to-end model accuracy.

## Local TSC extraction and MiniLM (migration 94)

The combined preview includes `tsc-reranker`, a private, offline INT8 MiniLM service
limited to one CPU/1 GiB. Model/tokenizer files are pinned and fetched at image
build only. The amd64 model artifact targets AVX2; verify the target PC CPU supports
it. Runtime uses only the internal reading network and has no database credentials
or host port. API/worker use `EPCERP_TSC_RETRIEVAL=minilm` by default in this local
Compose file; `bm25` is the visible-fallback/disable option. Do not infer target-PC
capacity from this Mac's checks. Preserve antivirus/resource controls.

After readings finish, choose Local extraction (no LLM) in Step 2. Go fills
explicitly labelled facts and keeps prepared excerpts for pending questions.
Ranking relevance is not correctness, completeness or approval. Configured cloud
providers remain explicit and gated; selected evidence is bounded and tied to
the same paid-call identity. No automatic provider activation or paid retry occurs.
Run `docker compose --env-file .env -f compose.yaml exec tsc-reranker python smoke.py`
for the synthetic real-model check. See ../../docs/GO_TSC_LOCAL_RAG_SLICE.md.

## Table and multiline TSC facts (extraction v5)

Local extraction now reads aligned PDF rows, Markdown/tab-separated cells,
numbered labels, wrapped labels/values, and adjacent same-page table fragments.
Labelled completion clauses can span a heading and its immediately adjacent
paragraph. Exact row excerpts retain original passage IDs; long source passages
no longer invalidate a short fact by exceeding the citation limit.

Twenty-one explicit field families are recognised. GST-inclusive/exclusive values
require a stated tax basis; no tax calculation is inferred. Tender fee and EMD
remain separate. Compound fee/payment/exemption questions and financial/security/
payment clauses are proposed as needs-review text, without deciding applicability,
company qualification, or approval. Different values remain conflicts; repeated
copies cannot crowd an amended value out of the citation budget.

Rebuild API/TSC and document workers/frontend from the same integration checkout.
Schema remains 94; no database migration or key change is needed. Generate a NEW
TSC with Local extraction (no LLM) to use proposal version tsc-local-rag-v5.
Completed snapshots stay unchanged. Queued jobs pinned to an older explicit
extraction version fail visibly instead of silently using changed extraction.
Cloud proposals remain optional, separately configured and explicitly approved;
this change does not activate providers or download a generative model.

### HTTP tender creation correction (29 October 2026)

Plain HTTP IP origins do not expose `crypto.randomUUID` or `crypto.subtle`.
Tender creation previously generated its operation key outside `try/finally`,
leaving the form on `Creating...` before a request was sent. Tender operations
now use the existing cryptographic `getRandomValues` UUID fallback. File checksums
use WebCrypto where available and a lazy, pinned SHA-256 implementation otherwise;
HTTP fallback hashing is chunked and retains server checksum verification.
Creation errors release the busy state. A 30-second response deadline offers a
retry with the same operation key and frozen name when the outcome is unknown.

Feature verification: 226 frontend tests, lint and TypeScript checks; Chrome on
an actual non-secure QA HTTP origin at 1440px and 390px reproduced the original
failure, then passed RNG-error recovery, a real 30-second timeout, same-key retry,
draft opening and synthetic file checksum/upload without page errors. Browser
plugin unavailable, so `frontend/scripts/verify-tender-http.mjs` uses the existing
Playwright runtime and disposable preview route. No operational records or
provider calls are made by that script. Both active remote feature tips remain
ancestors of `codex/development` base `75d3102`; working-tree hashes and deployment
smoke checks are recorded separately in the development release manifest.

### Development Gemini activation

`gemini-development.compose.yaml` is an explicit development-only override for
image-only releases that otherwise pin cloud inference off. It enables Gemini in
both API and TSC worker, retains local extraction/relevance ranking, and leaves
OpenAI disabled. It does not install a generative local LLM.

Store the Gemini variable in a separate owner-only `.env.gemini` beside the active
server release; keep the original `.env`, all encryption keys and volumes intact.
Supply both environment files and both Compose files when recreating **only**
`api` and `tsc-worker`. Validate with `config --quiet`; ordinary `config` expands
secrets and must never be logged. Do not embed the key in YAML, shell arguments,
Git, browser code or API responses. Preserve this private environment and override
when moving to a later development release, otherwise the release defaults disable
cloud inference again. No schema migration is needed for activation.

Gemini 3.1 Flash-Lite has a free tier, but project-specific quotas apply. Confirm
the Google project is on the Free tier with billing disabled when zero provider
charges are required; an API key alone does not prove billing status. The app's
INR 100 per-tender allowance accounting remains conservative and is not a provider
invoice or a guarantee that a billing-enabled project cannot be charged. There
is no automatic provider switch or paid retry. Use explicit question proposals
for small comparisons before attempting a whole checklist.

Google's unpaid-service terms permit use of submitted content and responses for
product improvement and prohibit sensitive/confidential/personal information.
Use public or synthetic evidence for free-tier evaluation; review paid API data
handling before using private documents. References:
https://ai.google.dev/gemini-api/terms
https://ai.google.dev/gemini-api/docs/pricing
https://ai.google.dev/gemini-api/docs/rate-limits

### Private-document residency decision (29 October 2026)

The user requires private tender documents and their inference processing to remain
in India or on their own server. This overrides the proposed standard Gemini API
activation above. Cloud inference remains disabled; the supplied key is retained
only in an owner-only, Git-ignored local file and has not been sent to the server
or used for a provider request. The prepared override now defaults approval to false.

Paid API no-training terms do not establish India-only processing. Google's current
Gemini 3.1 Flash-Lite Cloud model page lists global/US/EU availability and US/EU ML
processing, so it has not been approved for this deployment. A future India-hosted
provider must be reviewed for both storage AND inference processing, including
logs and any external fallback, before activation. Selecting a Mumbai endpoint name
alone is not evidence of residency. References checked 29 October 2026:
https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/gemini/3-1-flash-lite
https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/data-residency

Current on-server processing is rule-based extraction and private MiniLM ranking;
there is no generative local LLM loaded. Adding generative inference requires a
model/server capacity decision; no GPU purchase or paid India cloud resources are
authorised by this residency statement alone.

### Gemini on this Mac and economical client operation (2 November 2026)

The user subsequently authorised the public Gemini API for this Mac's local
preview. Its private `.env.gemini` enables cloud approval and supplies the Gemini
key to API and TSC worker; OpenAI stays disabled. This supersedes the earlier
cloud-off decision for this preview only. Google metadata accepted the saved key
and confirmed `gemini-3.1-flash-lite`; key format alone was not a validity check.
This is public cloud processing, including when the app itself runs locally.
It does not prove India-only residency or authorise deploying another server.

`setup.sh` loads `.env.gemini`, when present, after the original `.env` so later
rebuilds retain the explicit provider settings. Keep both files owner-only and
outside Git. For a provider-only change with unchanged images/schema, validate
and recreate only the API and TSC worker from this directory:

```sh
docker compose --env-file .env --env-file .env.gemini -f compose.yaml -f gemini-development.compose.yaml config --quiet
docker compose --env-file .env --env-file .env.gemini -f compose.yaml -f gemini-development.compose.yaml up -d --no-deps --wait api tsc-worker
```

Gemini generation already extracts explicit labelled facts first and skips cloud
calls for those candidates. Only unresolved questions with usable bounded evidence
go to Gemini. Source text is capped at 14,000 bytes/16 passages per proposal;
output is capped at 2,048 tokens. Human decisions and missing/context-incomplete
evidence stay for review. The same operation reuses its encrypted successful
result; uncertain/failed requests keep their allowance and are never resent
automatically. A new deliberate operation may incur another charge.

For minimum API spend, generate with **Local extraction · no LLM**, then request
Gemini proposals only for unresolved questions. Selecting **Gemini Flash-Lite**
for generation uses the same local-first rule, then proposes remaining answers.
The INR 100 per-tender cap remains, with INR 1.50 reserved per Gemini request.
These allowances are conservative limits, not measured invoices or a promise of
free-tier availability. Current standard Flash-Lite text pricing is USD 0.25 per
million input tokens and USD 1.50 per million output tokens:
https://ai.google.dev/gemini-api/docs/pricing

Client machines need no generative local model. After separately configuring and
approving their server key, set `EPCERP_TSC_RETRIEVAL=bm25` in its private provider
environment file to use Go keyword retrieval instead of MiniLM. Recreate API/TSC
with matching settings; scale `tsc-reranker` to zero only when no queued jobs are
pinned to its retrieval model. Keep document workers and antivirus. This reduces
model memory requirements but is not full-stack capacity acceptance. This Mac
retains its existing MiniLM ranking; it is a small relevance model, not an LLM.

Activation verification: both local API/TSC services loaded the saved key and
explicit approval, remained healthy, and kept schema 96 clean. Images and
persistent volumes were retained. The production Go adapter completed exactly
one synthetic EMD proposal with HTTP 200, a valid exact-source citation, and
398 input/53 output tokens. No private document was used for that test. This
proves connectivity and structured proposal handling, not real-tender accuracy,
the project's billing tier, or an invoiced cost. Tender domain/application/adapter
race tests passed; isolated setup-script checks passed with and without provider
configuration and preserved original credentials. Remote deployment was not changed.

### TSC editing workspace (2 November 2026)

The checklist opens with sources collapsed to give editing the full width. Use
Show sources to open the reference column; on desktop the editor keeps the larger
share. On smaller screens the same control switches between full-width editing
and source reading. Closing sources returns keyboard focus to the toggle and
retains manual edits, source filters and mounted reader state. Answers appear
before supporting tools, grow with longer text, and remain manually resizable.
Compact filters, acceptance progress and unsaved counts stay visible above the
question list. Acceptance, citation, permission and source-version rules are
unchanged; collapsing a panel does not save or accept an answer.

Verification: 228 frontend tests, TypeScript, targeted ESLint and the production
Docker frontend build passed. Existing Playwright/Chrome ran the synthetic
70-question workflow on the isolated development frontend and rebuilt Docker
preview: collapse/reopen, draft/filter preservation, restored focus, larger editor,
long-answer growth, source selection/citations, proposal adoption, accept/reopen,
exports and stale-source guards. Desktop 1440px/mobile 390px and both themes passed
with no page errors, console warnings/errors, framework overlays or horizontal
overflow. Browser plugin was unavailable; existing Playwright was used. All nine
local services stayed healthy, schema stayed 96 clean, and Gemini stayed enabled.
This is synthetic UI verification and combined-stack smoke evidence; no real
document or provider request was used for these checks. Remote server unchanged.

### Named TSC formats in Global masters (migration 97, 2 November 2026)

Open Global masters → TSC formats. Every format has a unique name, description,
availability and revision. Create a blank format or copy an existing one; edit,
add, remove/undo and reorder its questions, then Save format. Each question has
a section, optional display number and document/AI or human-decision preparation.
Formats contain 1–200 questions. Inactive formats remain readable but cannot be
selected for a new generation. There is no destructive format-delete endpoint.

The existing uploaded checklist is the first active format, **EPC Works —
Standard TSC**, with all 70 questions. Repeated short prompts now identify their
tender-fee, EMD, price-escalation or penalty context. Case, whitespace and
punctuation cannot create duplicate question wording within one format; duplicate
wording across different formats is allowed. UI feedback, API validation and a
format-scoped PostgreSQL unique constraint enforce this. Format names are unique
under the same normalization. This does not perform semantic paraphrase matching.

Permissions are `settings.tsc_formats.view/read/write`; they are registered without
granting ordinary roles extra authority. The API rechecks live read/write access.
Saves require the loaded revision, lock the format, and atomically update questions,
immutable before/after history and the central event journal. Stale edits return
409 and leave the draft available for review/reload. Copies retain reviewed local
extraction identities only for unchanged uploaded questions; changed legacy
questions receive new identities so an unrelated extraction rule cannot fill them.

Choose TSC format alongside draft preparation in Generate TSC. New jobs freeze its
ID, name, revision, question order/text and preparation rules in their encrypted
input. Master changes and question removal never rewrite existing jobs or answers.
New named-format DOCX exports use that frozen variable-length question set; older
jobs keep their original fixed-form export path. Cloud approval and the INR 100
per-tender cap remain unchanged and shared across formats/generations.

Migration 97 is Tender-owned, imports only the existing checklist master and adds
format/history tables plus nullable immutable format references on jobs. Existing
encrypted snapshots are untouched. Downgrade refuses retained format history or
named jobs. Before upgrading, stop application writers and verify private backups
of database, objects, document packages and environment/keys. Build API/migrator,
frontend and document workers from the same combined checkout before startup.

Verification: backend check/build and full disposable PostgreSQL integration gates
passed, including native permissions, concurrent/stale saves, database duplicate
bypass attempts, audit rollback/immutability, revisioned generation, replay,
snapshot preservation, inactive formats and custom-length exports. Frontend: 230
tests, TypeScript, targeted ESLint and production Docker builds. Actual components
passed synthetic Chrome checks on development and Docker previews for create/copy,
edit, duplicates, reorder, remove/undo, stale draft preservation/reload, read-only,
active/inactive filters, selected format context and the existing full TSC flow.
Desktop 1440px/mobile 390px and light/dark views were inspected. A synthetic custom
DOCX was rendered and inspected. Browser plugin was unavailable; existing
Playwright was used. No operational document or paid request was used in QA.

The local stack was upgraded after a verified isolated PostgreSQL restore and
object/package archive checks. Existing user/role/client/draft/job counts and TSC
input identities were preserved; all nine services are healthy at schema 97 clean.
Credentials, encryption keys, volumes and Gemini settings were preserved. Private
backup/release manifests record exact source and image identities. Remote server
and `main` are unchanged. Authenticated operational browser writes were not used
as deployment smoke checks.

### Separate TSC format workspaces (2 November 2026)

The format library is now a dedicated full-width view. Opening a format leaves
the library and opens its own workspace, with separate Format details and
Questions tabs. Questions are readable summary rows, not expanded input forms.
Search the list and open one question in a wide, centered editor. Numbering and
answer-preparation controls are disclosed only when needed. Apply to format
returns to the list; Save format persists the revision. The compact save bar
stays available while scrolling. Copy and reorder/remove actions sit in their
respective action menus. Existing validation, permissions and snapshots remain.

Checks: 230 frontend tests, TypeScript, targeted ESLint and production build;
Chrome synthetic flows verify separate library/details/list/editor views, zero
expanded forms in the question list, a desktop question field wider than 800px,
one-question editing, optional controls, tab keys/focus restoration, copy/create,
duplicate prevention, reorder/remove/undo, stale drafts, inactive filtering and
read-only access. Desktop 1440px/mobile 390px and both themes were inspected.
The local frontend alone was replaced; schema remains 97 and provider settings
are unchanged. Browser plugin unavailable; existing Playwright used. These checks
use synthetic records and make no provider calls.

### TSC format card grid (2 November 2026)

The library uses clickable cards with names and question counts. The original
format displays as EPC Works; its saved full name remains in the editor.
Descriptions stay in Format details. Library instructional paragraphs and
revision metadata are removed. Keep this browsing view free of explanatory copy.

Verified: TypeScript, targeted ESLint, production build and synthetic Chrome
flows on the deployed local frontend, including a three-card desktop grid,
single-column mobile layout, keyboard opening and existing editing/validation
flows. Schema remains 97 clean; all nine services are healthy. No operational
records or provider calls were used. Remote server and main remain unchanged.

### TSC formats table and document preview (migration 98, 2 November 2026)

The library now uses a table: S.No., Name, No. of questions, Purpose, Created on,
Status and Actions. View, Edit and Activate/Deactivate use labelled icon buttons.
All formats are shown initially. Search includes purpose. The existing default
name displays as EPC Works, while its saved full name remains unchanged.

Purpose is a separate persisted field (Generic by default; editable with
Electrical/Solar suggestions). Creation dates come from the database. Status
changes preserve all questions and metadata and reject stale versions. Read-only
users have View only. Existing historical checklist snapshots are unchanged.

View renders the generated blank-answer DOCX offline with LibreOffice and shows
its PDF pages in the existing PDF.js viewer with pagination, zoom and Word/PDF
downloads. Both document endpoints enforce current format read permissions.
Rendering uses private temporary files, one concurrent process and a 45-second
timeout, with no cloud/provider requests. Deploy the updated API image containing
LibreOffice; the document-reading worker image already includes its own renderer.

Verified: 230 frontend tests, TypeScript, targeted ESLint, Go Tender FMS/module
and migration metadata tests, isolated PostgreSQL migration/format permission and
metadata tests, production API/frontend/worker builds, synthetic desktop/mobile
and light/dark browser flows, visible PDF page rendering and DOCX-to-PDF visual
inspection. Browser plugin unavailable; existing Playwright used. Feature QA
uses synthetic records; combined-stack checks cover health and schema separately.

Before the local upgrade all application writers were stopped. A private full
backup was restored into a disposable network-isolated database and every table
count matched; object/package archives and credential copies were verified.
Matching services run at schema 98 clean with preserved volumes, keys and Gemini
settings. Private backup/verification manifests record commits, image identities
and checks. This is a local preview update; main and the remote server are unchanged.

### Gemini on the development server (2 November 2026)

The user subsequently approved activation on the development server. Its API and
TSC worker now load the supplied key from an owner-only `.env.gemini` through
service `env_file` entries. Normal Compose restarts retain it without embedding
the key in YAML. Preserve that private file alongside the original `.env` when
moving to another release. OpenAI remains disabled.

Both services join `cloud_outbound` for provider HTTPS; existing database,
storage and document-reading networks remain internal. No host ports were added.
The reusable Gemini override now includes this outbound network too, because
loading a key alone cannot enable providers on an internal-only deployment.

Google model metadata accepted the saved key from the host, API and TSC worker
for `gemini-3.1-flash-lite`. No generateContent request or document upload was used
for verification. Images, schema 98, credentials/encryption keys, local-first
extraction and the INR 100 conservative allowance cap were preserved. Refresh the
TSC workspace and choose Gemini Flash-Lite under Draft preparation.
