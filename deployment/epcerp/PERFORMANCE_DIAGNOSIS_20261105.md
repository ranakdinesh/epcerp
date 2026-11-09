# Data-loading performance diagnosis — 5 November 2026

## Tested source

`codex/development`, base `8015108706a84ff15c8fad0d68c77a837af0f509`, plus the
uncommitted authorization-concurrency change accompanying this report.
Fetched origin and verified every remote tip below is an ancestor of HEAD before
testing; no branch integration or main merge was necessary:

| Branch | Tip |
| --- | --- |
| codex/development | 8015108706a84ff15c8fad0d68c77a837af0f509 |
| codex/table-aware-tender-evidence | 7acaa510a728cca18bb8d7e52e1bee2b9ecf3fc2 |
| codex/tsc-processing-guardrails | 17868507ffaa76221dbb8574856a819149973cac |
| tenderFMS | 1c7c4f613e42e4af093fdcb2ee709ec7b9cc92f9 |
| main, chor/auth | 756335819939beb72596e1200a7c9c510660f265 |

## Findings and change

Business transactions were taking an exclusive transaction-level advisory lock on
`auth.administration`. This queued unrelated requests behind purchasing reads.
Purchasing, projects (including budgets/team), personal tasks, inbox and profile
GET now use an explicit shared business guard. The guard is acquired before
resolving current credentials/permissions and retained until transaction end.
Business writes retain their actor context and existing module/row locks.
Auth administration and profile mutations retain their exclusive guards.
The business guard must never be used for mutations to Auth state.

Local synthetic PostgreSQL reproduction, same purchasing fixture before/after:

| Operation | Before | After |
| --- | --- | --- |
| Profile GET without contention | 8 ms | 8 ms |
| Profile GET while a purchasing actor transaction holds for 1 second | 1,025 ms | 18 ms |
| Purchasing dashboard, 20 projects | 285 ms / 1,671 queries | 304 ms / 1,671 queries |
| Purchasing dashboard requested for count, one project | 11 ms / 110 queries | 13 ms / 110 queries |

The one-second hold is an artificial `pg_sleep(1)` after purchasing authorization;
these are local measurements, not development-server timings. Small timing
differences are noise; the guard fix removes the reproduced cross-request wait
but does not reduce purchasing query count.

## Feature verification

Fresh isolated synthetic databases were migrated to owned schema version 101;
the purchasing QA database reports `101`, `dirty=false`. No application database
was upgraded or reset. Offline historical Go oracles were extracted from the
frozen reference by the existing compatibility scripts; no Go service was run.

- Purchasing compatibility: 42 passed.
- Projects compatibility: 14 passed.
- Project budgets compatibility: 16 passed.
- Project team compatibility: 17 passed.
- Personal tasks compatibility: 11 passed.
- Inbox compatibility: 13 passed.
- New PostgreSQL authorization concurrency/revocation regression: 1 passed.
  Concurrent business guards coexist; exclusive administration waits for them;
  an arriving business request waits for administration and observes a committed
  access-session revocation. Invalid tokens remain denied.
- TypeScript, lint of affected server code/test and `git diff --check`: passed.

No combined Docker-stack smoke test, browser timing or development-server
deployment was performed for this patch. No paid provider calls or messages
were sent. Existing application data, volumes, passwords and keys were untouched.

## Remaining work

Purchasing recursively computes full progress for each dashboard project, with
repeated queries and an additional full-project request used for counts. This
needs separate batching/summary work and output-equivalence verification.
Tender document preparation can also retain an exclusive Auth guard during
Office conversion; that transaction boundary remains to be corrected separately.
Other exclusive guard callers have not been changed indiscriminately.

The public development health endpoint responded in about 101 ms during the
initial investigation; this does not measure authenticated business requests.
The user's development-server data-loading delay still needs authenticated
before/after measurement when this patch is deployed.
