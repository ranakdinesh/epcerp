# Profile persistence verification — 5 November 2026

Local preview only; production deployment is outstanding.

Integration checkout: `codex/development`, HEAD `8015108706a84ff15c8fad0d68c77a837af0f509`, with existing uncommitted concurrency changes plus this profile fix. Remote refs fetched before testing; all listed tips verified ancestors of HEAD:

| Remote branch | Tip |
| --- | --- |
| main / chor/auth | 7563358 |
| codex/development | 8015108 |
| codex/table-aware-tender-evidence | 7acaa51 |
| codex/tsc-processing-guardrails | 1786850 |
| tenderFMS | 1c7c4f6 |

Profile changes: normalize PostgreSQL bigint versions before optimistic comparison; load a dialog-owned contact snapshot before mounting the form; prevent an older directory response from replacing newer profile data; derive completion reminder from saved contact data while keeping the open dialog mounted.

Feature verification:

- Clean disposable database migrated to owned version 101: platform workflow suite, 9 tests passed, including repeated profile save, fresh GET persistence, stale-write rejection and personal-contact privacy.
- Chrome/Playwright at `http://localhost:59093`, 1440×1000: synthetic account completes profile, saves twice, reloads, starts a fresh browser session and reopens the form with all values retained. Completion reminder disappears; no page exceptions. Browser plugin unavailable; installed Playwright used. Screenshot: `/tmp/epcerp-profile-saved.png`.
- Targeted ESLint, TypeScript check and production Docker build passed.
- Earlier run against a populated disposable copy passed profile assertions but failed unrelated director and TSC fixture assumptions; clean fixture run passed all tests.

Combined preview smoke verification:

- Matching frontend/migrator and document processor built from the same checkout and started on localhost:9092. Login HTTP request succeeds; frontend and document processor healthy.
- Existing database version 101, dirty=false; no schema upgrade or real-user profile mutation. Existing storage volumes and keys retained.
- Frontend image: `sha256:93b720e3863566649a9d62d23fa61befb8e8b147af6081fdda1b3121b5dc3af0`.
- Processor image: `sha256:6b4a9376e540e9532bd6b8206cc71b095ac7381e248a70bc1b881958045ec06a`.

Tested source SHA-256:

- profile.ts: `cd7d36caa5be3c4b0d0a42f9972a21cbb8b516a479a11e1b848d807cbdd6e082`
- personal-profile.tsx: `e7adc3ee5cde73928076b677c0e28a1dd19f06bc82d8d5472704d4eba70aa1cb`
- workflow.test.ts: `f41f213b51f926f5081a9ad3b355724bea1f9d9c1bda8cbe76394a6ab5152405`

No production deployment, provider calls, real messages or cloud uploads performed.
