# Local TSC evidence selection and extraction slice

Status: implemented and locally verified on codex/development, 29 October 2026. Real-tender accuracy and 4-core/8-GB target acceptance remain pending.

Business outcome: reduce irrelevant LLM input and paid calls while retaining
source-backed draft facts, missing-information gaps and accountable review.
Users: uploader/Tender reviewer with existing tenders.tsc.read/write/export and
source draft/processing permissions. Trigger: Generate TSC using Local extraction,
or explicitly select a configured model for unresolved questions.

Workflow: authorise/freeze sources; extract conservative labelled particulars in
Go; rank bounded BM25 candidates with a pinned local MiniLM cross-encoder; expand
source context; persist selected evidence/diagnostics; propose only unresolved
facts with the same selected evidence. Manual mode remains blank. Local mode
never calls an LLM. Model ranking is relevance, not correctness or eligibility.

Records of truth: unchanged encrypted originals/readings, immutable TSC input
snapshots and encrypted result/event history. New retrieval diagnostics and
question evidence are versioned JSON within the existing encrypted result.
Migration 94 adds local job mode only; no plaintext vector index or company data.

Authorization: reuse current owner-scoped draft and TSC permissions, check live
source versions and access after inference. Private inference has no database,
cloud credentials, public port, outbound network or retained request bodies.
Model files are pinned and downloaded only at image build. Audit remains the
existing transaction-bound TSC job/result/change/export journal.

Risks: English-only relevance model; missed clauses in BM25 candidate stage;
OCR errors; truncated model windows; exceptions/corrigenda; model unavailability;
CPU/RAM contention. Preserve originals/full review path, attach warnings, retain
BM25 fallback, keep human decisions manual, bound local inference and never
automatically activate cloud or retry paid requests. Company matching, BOQ/table
reconstruction, final approval and measured real-tender accuracy are out of scope.

Acceptance: native migration and module-boundary checks; deterministic extraction
and conflict preservation; all model results map only to supplied candidate IDs;
exact selected evidence goes to provider and paid-call fingerprint; failed local
ranking has visible fallback and no automatic cloud switch; source/version/live
revocation guards; zero-cost local mode and persisted unresolved evidence; pinned
real-model CPU smoke test; frontend/build/Docker health and data preservation.

Reporting: per question, retrieval method/model/fallback, baseline/selected bytes,
candidates, excerpt count, source IDs and context gaps. These are resource and
coverage indicators, never an accuracy percentage. Report actual invoice spend
only when separately measured. Gold real-package evaluation remains pending.

## Verified implementation

Go TSCEvidence selects up to 30 unique BM25 candidate blocks. The fixed private
MiniLM adapter returns only scored candidate IDs. Go rejects incomplete/duplicate/
invented/nonfinite rankings, retains retrieved corrigenda and neighbouring fragments
(including adjacent pages), and applies a 12-excerpt/9000-byte selected context cap.
A context overflow leaves the question for source review and prevents automatic
model generation. This is bounded context expansion, not complete referenced-clause
resolution or proof that every contractual exception has been found.

Local mode fills the existing eight explicitly labelled particulars without an LLM,
retains disagreements, and persists prepared evidence for unresolved questions. It
never sets Accepted. Manual mode stays blank. Explicit configured cloud generation
skips deterministic candidates; per-question proposals send the selected evidence
recorded in their paid identity. No provider silently reruns retrieval. Existing
INR 100 reservations, no paid retry, encryption, owner scope and version/live checks
remain. No cloud provider was called or enabled during verification.

The service uses pinned MiniLM revision 233902d25c440f23af6f7d6e94d2946bac0bee0a,
publisher-supplied arm64/x86 AVX2 INT8 ONNX weights and a pinned tokenizer/runtime.
Build creates an integrity manifest; runtime verifies it, loads once, windows long
passages without silently dropping overflow, runs one request with one thread and
no request-body logging. It has no credentials, database, host port or outbound
network. The Compose ceiling is one CPU/1 GiB, not a measured requirement.

The shared Go processing slot covers inference, reusing an already-held worker
slot and visibly falling back to BM25 when the slot/service is unavailable. New
TSC inputs pin retrieval identity. Historical input snapshots remain intact.

Checks: make check build; make verify (native PostgreSQL/race); additional
selected-evidence identity and local-mode tests; frontend lint/types/225 tests;
production Docker builds; desktop/mobile synthetic browser fixture covering local
mode, prepared evidence, free search, adoption, accept/reopen, exports and stale
sources; pinned real-model EMD/fee smoke and real Go-to-model selection. The
21-candidate synthetic selection retained the EMD source and selected 6827 bytes
against a 13591-byte BM25 baseline in 0.63 seconds on this Mac. This is neither
real-tender accuracy, 8-GB target latency nor measured billing/token savings.

Use Step 1 to finish readings, then Step 2 -> Local extraction (no LLM). Inspect
Prepared source excerpts and unresolved answers. Use explicit question research
with an approved provider only when needed. Set EPCERP_TSC_RETRIEVAL=bm25 to
disable reranking in API/worker and recreate both; no schema downgrade is needed.
Downgrade from 94 refuses retained local jobs instead of deleting evidence.

Remaining: reviewed real-package corpus and held-out evaluation; Hindi/multilingual
relevance; semantic retrieval; richer clause/reference/table links; company evidence;
final approval; persisted actual provider token/invoice accounting; target-PC full
stack load testing. The frontend's pre-existing 4 lint warnings and npm audit
findings are unchanged by this slice.
