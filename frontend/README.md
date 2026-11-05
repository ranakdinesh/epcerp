# epcERP application

Next.js 16, React 19, TypeScript and native server modules. No Go API is required.

## Run

Install Node.js 22, run `npm ci`, configure a private `.env.local` from
[.env.example](.env.example), apply owned migrations with `npm run migrate`, then
run `npm run dev`. The development UI uses port 3001. Docker preview uses port 9092.
Use only the owned epcERP database; keep credentials/encryption keys and existing
volumes unchanged. Do not reapply bootstrap passwords to existing accounts.

The route namespace is internal. All request handlers run in process; unmapped
routes return 404. Module owners default to `nextjs`. The retired Go ownership
configuration is rejected at application startup. Cloud approval, credentials and
spending limits remain explicit; do not activate paid calls for testing.

## Modules

`src/server/auth`, `master`, `tenders`, `projects`, `purchasing`, `personal-tasks`,
`inbox` and `platform` own business logic. The private document processor supplies
malware validation, Office/PDF conversion and text extraction without local LLMs.
Canonical catalogue SQL lives in `src/server/master/catalogue/queries`; the TSC
Word template is `assets/templates/tender-tsc.docx`. Sync commands generate the
runtime copies without any backend checkout.

## Checks

Run `npm test`, `npm run test:auth`, `npm run typecheck`, `npm run build`, and the
SQL/template sync checks. `npm run test:native` uses an explicitly provided fresh
`epcerp_auth_qa_retire_` database and makes no provider calls. Cloud TSC workflow
verification uses a separate `epcerp_auth_qa_tsc_` database via `npm run test:tsc`.
Never point mutation tests at an application database.

[Optional historical parity checks](scripts/compatibility/README.md) use a frozen
reference extracted from Git into a temporary directory; normal development and
builds do not install Go or check out the retired application.
