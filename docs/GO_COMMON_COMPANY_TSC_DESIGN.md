# Common company evidence and answered TSC documents

Date: 25 October 2026. Status: implementation specification. Common, company
evidence administration and answered TSC generation are not yet implemented.

Synthetic evaluation data is available at `backend/testdata/tsc_company`: an
invented company profile/evidence set, ten independent tender-clause cases and
an opt-in loopback Ollama harness. This fixture is separate from operational
company records and does not implement the Common module or FMS integration.

Produce a Tender Selection Checklist containing every question in the supplied
company form, answers grounded in the tender sources, and relevant company facts.
Maintain company facts once and reuse their reviewed, dated evidence across
tenders. Tender conditions and company capability must remain distinguishable.

## Outcome and workflow

Primary users are the uploader/Tender Executive, assigned Tender Manager, company
record administrators and authorised Finance/engineering evidence reviewers.
The trigger is a document-intake draft with completed readings, or a request to
correct an existing TSC after new tender/company evidence becomes available.

1. Select the existing uploader-owned tender draft. Enumerate all batches,
   originals, readings, warnings and missing material. Record an immutable input
   manifest; successful parsing alone does not prove package completeness.
2. Load the versioned TSC question registry derived from the repository-root
   `TSC Format.docx` (the supplied company form).
   There are 70 questions/subquestions and eight initial BOQ rows. IDs combine
   section and stable keys; the two printed question 15 entries remain distinct.
3. Retrieve only currently authorised company evidence relevant to each question.
   A licence, certificate or annual account is a dated record with provenance,
   not a permanent company-level Yes/No answer.
4. Prepare tender requirement, company evidence and proposed response separately.
   Optional local-model extraction may propose cited answers. Manual entry with
   evidence remains available. Missing facts remain unknown.
5. Review conflicts, applicability, validity and missing evidence. Assign the
   Tender Manager through the separately versioned assignment workflow; merely
   exporting a draft must not invent an assignment or approve facts.
6. Freeze a reviewed answer snapshot and generate the answered DOCX using the
   original template, with an evidence appendix and explicit continuation pages.
   Draft exports are labelled Draft and retain unresolved statuses. Finalisation
   requires current authority and resolution of blocking gaps.
7. New source/company revisions flag affected answers for review. Retain previous
   snapshots and exports; do not silently rewrite them or choose the newest
   contradictory document as authoritative.

## Module ownership

Create `backend/internal/modules/common` with focused company-profile,
company-evidence and location-reference services. Keep `domain`, `ports`,
`application`, `contracts`, adapters and a module composition facade. Core layers
remain infrastructure free and have Go AST boundary tests.

Common owns shared application reference data and company evidence in PostgreSQL
`public`. PostgreSQL schema placement alone does not determine module ownership.
Auth, Projects, Purchase, Finance and shared document services keep their business
tables and published contracts, including tables currently in `public`.

The user-approved Common ownership supersedes the earlier Master grouping for
state/city references only. Master retains catalogue, clients, vendors,
manufacturers, client types and industries until a specific additional transfer
is defined. Existing `public.indian_states` and `public.indian_cities` are already
deployed and are currently managed by Master's reference service. Transfer their
application/query ownership; do not copy or recreate rows. Preserve IDs, foreign
keys, names, active flags, audit data, existing API routes and
`settings.locations.read/write` semantics. Master consumers use a Common contract.
Keep a compatibility facade during the transition, with exactly one write owner.
Do not edit applied migration 2 or remove its global history entry.

Company evidence is not Client Master and not the Auth organisation hierarchy.
Retain the existing one-organisation-per-deployment model: a server-selected
company profile, no browser-selected tenant or generic multi-company ERP.

Tender owns the question registry, answers, comparisons, review decisions and
generated TSC snapshots under schema `tender`. It consumes Common's published
evidence contract; it must not directly query or modify Common tables. Common
does not own tender-specific exemption, eligibility or bid/no-bid decisions.

## Proposed records of truth

Names below are proposed, not applied migrations or active APIs. Register new
versioned module-owned migrations in the existing coordinated embedded runner
and globally ordered sqlc manifest. Qualify new SQL with `public` or `tender`.

| Proposed table | Responsibility |
| --- | --- |
| `public.company_profiles` | Deployment company legal/display name, reviewed identifiers and address/reference links; one active deployment profile |
| `public.company_credentials` | Startup/Udyam/other registrations and electrical licences; issuer, number, applicable state, voltage/category, issue/expiry and applicability restrictions |
| `public.company_financial_years` | Exact turnover, net worth and other reviewed annual figures by financial year, currency and accounting basis |
| `public.company_financial_certificates` | Dated bank/CA/solvency/working-capital evidence with amount, basis, issuer and validity; distinct from annual figures |
| `public.company_work_experience` | Employer, scope, location, completion/status, executed amount, currency/tax/date basis, company/JV share and completion evidence |
| `public.company_authorisations` | OEM authorisations and endorsements with product, issuer, scope and tender-specific restrictions where applicable |
| `public.company_evidence_revisions` | Append-only evidence/review/provenance history linking each fact revision to protected originals and locators |
| `tender.tsc_templates` | Versioned question registry and source DOCX hash/version |
| `tender.tsc_answer_revisions` | Append-only question response, typed values, tender requirement, company comparison, status and evidence references |
| `tender.tsc_snapshots` | Immutable input/answer/review manifest, template revision and draft/final export identities |

Every administrator-managed company record has `is_active`, timestamps and nullable
`created_by_id`/`updated_by_id` foreign keys to `auth.users`, plus optimistic version.
Creation and changes use authenticated actors. Deactivate instead of deleting.
Revisions retain reviewer, review time, effective/expiry dates, correction reason,
source document/revision/hash and page/clause/sheet/cell locator. Bootstrap/import
provenance remains distinct from actor IDs. Do not seed actual company facts.

Use exact decimal amounts with explicit currency and tax/time/scope bases. Unknown
is null. Annual turnover is stored per year; a tender-specific average is a derived,
reviewable calculation using exactly the tender's required years and definition.
Never assume a universal escalation rate, experience threshold or exemption.

Original company attachments use the reusable protected document service through
published contracts, with Common-owned context authorisation. Expired, unreviewed,
revoked or inaccessible evidence cannot be presented as current compliance.

## Question routing

All original questions remain in the output. The companion
`GO_TSC_QUESTION_SOURCE_MAP.json` contains the exact extracted labels and stable
section IDs for all 70 questions and eight BOQ placeholders.

| Section | Tender evidence | Company evidence or judgement |
| --- | --- | --- |
| Particulars | Department, work, location, values/tax basis, RA, periods, deadlines, portal, site visit, hard copy, validity, JV permission and tender type | No company substitute for missing tender terms |
| Tender fee | Amount, payment modes, exemption conditions and surety acceptance | Reviewed Startup/Udyam credentials and applicable category; eligibility for exemption needs comparison |
| EMD | Amount, payment modes, exemption/declaration conditions and surety acceptance | Reviewed credentials; an allowed declaration is not an existing signed declaration |
| Similar work | Monetary thresholds, lookback, escalation, definition and required documents/JV/OEM conditions | Completed-work evidence, applicable company/JV share, completion certificates and current OEM authorisations |
| Finance | Required turnover/years, bank/CA/third-party certificates, solvency, net worth and working capital | Reviewed annual accounts and dated certificates; restricted Finance access |
| Electrical licence | Required state, category/voltage and endorsement | Current licence/endorsement with matching state, voltage and validity |
| SD and PBG | Surety acceptance, percentage, DLP, LED warranty and guarantee validity | Capacity/financial approval is a separate decision; not implied by tender terms |
| Payment | Contractual milestones and payer | Payment regularity observations need dated reviewed operational evidence |
| BOQ | Items, specified makes, equivalent/similar permissions and source lines | Do not replace a client BOQ with company catalogue defaults; retain all items on continuation pages |
| Work location | Source location and stated constraints | Dated site/team/vendor or reviewed external evidence; no automatic news lookup or safe-location verdict |
| Risk | Escalation and LD clauses, contractual restrictions | Labour, payment, supplier, location and delivery risks require reviewed evidence |
| Conclusion | Reviewed criteria and source clauses | Eligibility requires comparison to current company evidence; bid/no-bid requires an authorised human decision |
| Other questions | Additional qualification documents, PQ terms, technical forms and price implications | Evidence availability and specific exceptions need review |

## Answer and document contract

Each answer retains question ID/template version; tender requirement; optional
company fact IDs/revisions; answer text and typed value; supporting excerpt and
original-source locators; candidate/manual/model origin; author/reviewer; timestamps;
input manifest and applicability/conflict reasons. Model provenance includes
provider/model/prompt version without storing secrets.

Response states include `not_found`, `needs_review`, `conflicting`, `answered` and
reasoned `not_applicable`. Review state is separate from response state. An
unreviewed answer is not approved, even when its citation is valid. Do not turn
absence of a clause into No, an unavailable amount into zero, or an expired
certificate into current compliance. Clearly distinguish tender permission from
company possession and from an authorised compliance conclusion.

DOCX export preserves the supplied form's question text, ordering, numbering and
columns. Put concise responses in answer cells and full evidence in an appendix
keyed to stable question IDs. Preserve units, dates/timezones, qualifications,
conditional clauses and uncertainty. Add continuation pages for long answers and
all BOQ items beyond the eight original rows. Render and inspect every page in
acceptance tests; text extraction alone does not establish usable layout.

Export permissions must cover every included source and company fact at download
time. A retained export inherits its most restrictive included content. Never leak
restricted Finance/credential evidence through an otherwise readable TSC, citation,
download, search result or model request.

## Permissions and audit

Proposed capabilities, registered only with their backed runtime slices:
`settings.company.view/read/write` in settings;
`settings.company_credentials.read/write` in settings;
`accounts.company_financials.read/write` in accounts;
and `tenders.tsc.read/write/review/export` in tenders. Exact company evidence
review authority must be source-specific and defined before review commands are
enabled. Preserve existing recovery administrator behaviour and live revocation.
Do not expand ordinary role bundles automatically or infer authority from titles.

Tender access requires current owner/assignment/portfolio scope in addition to
capability permissions. Reading a TSC does not grant company financial access.
Log profile/evidence changes, deactivation, review, proposal generation, answer
correction, finalisation and export with authenticated actor/reason/version and
source identifiers. Source state, immutable history and business event commit on
one shared transaction/context. No network/model/render work inside that transaction.

## Delivery sequence and acceptance

- [x] Record the requested ownership, records, question routing and document contract.
- [ ] C1: implement Common company profile/evidence administration, protected source
  links, permissions, migrations/sqlc, audit, native DB tests and frozen grid/drawers.
- [ ] C2: transfer existing state/city service/query ownership with compatibility
  routes and published Common references; prove unchanged data/FKs/permissions.
- [ ] T1: deliver the versioned 70-question/eight-row registry, manually editable
  source-linked answers and template-preserving draft DOCX export.
- [ ] T2: add bounded optional provider proposals, validated source citations,
  persistent work/recovery, cross-document conflicts and revision invalidation.
  Local Ollama versus manual-first is awaiting the user's answer. No cloud
  transmission follows from the presence of an environment variable.
- [ ] T3: deliver accountable Manager review/finalisation, approved snapshots and
  complete browser/real-tender acceptance without bypassing assignment contracts.

Before each runtime slice, define strict DTOs, methods, request/output limits,
rate limits, replay keys and expected versions. Test allow/deny and live revocation,
source ACLs, expired evidence, conditional exemptions, noncomparable amounts,
rollback/concurrent edits, stable exports, new-source invalidation and overflow.
AST tests must prohibit sibling adapters/direct cross-module SQL. Run owned native
PostgreSQL, sqlc vet/diff, formatting/vet/race/build and relevant frontend/browser
checks. Never use operational records as fixtures.

Reporting shows unanswered/conflicting/awaiting-review questions, source-reading
gaps, missing/expired company evidence and stale TSC inputs separately. Expiry
reminders require their own durable source-linked delivery; no real emails or
automatic external monitoring are enabled by this design.

The first populated document needs the user's selected tender draft and actual
reviewed company evidence. No tender/company answers have been fabricated or
exported by this specification.
