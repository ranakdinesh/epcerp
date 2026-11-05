# Project Team and Responsibilities

Team management stays inside the Projects server module. `http.ts` owns only
`/projects/:project/responsibilities`, `/options`, `/:profile` and `/:profile/history`
when `EPCERP_PROJECT_TEAM_OWNER=nextjs`. `service.ts` handles transaction-bound
commands and reads; `repository.ts` owns profile records and exact timestamps;
`validation.ts` preserves the Go input and project-scoped permission rules.

Auth supplies live account capabilities, active role memberships and the published
role directory. A profile can narrow an existing role's permissions; it cannot grant
new global permissions, team-management authority, PM/Procurement ownership or a
workflow bypass. Ordinary team readers need an effective scoped profile, current PM
ownership or portfolio read authority. Management additionally requires team.write
and PM ownership, portfolio.write or superuser status.

All saves lock the project before inspecting the profile/version, then write the
profile, immutable event snapshots and correlated business journal in one transaction.
User and role identities cannot change during revision. Active assignments require
an active open project and current account/role eligibility. Revocation remains
available on closed/held/inactive projects and after the assignee loses eligibility.
Historical wording and permissions remain intact. Bounded workspace/options/history
limits match Go. Existing schema, project versions, passwords and owning workflow
state are unchanged.

Go disables only these HTTP routes under the matching flag. Its published access
gate, readable-project directory, owner directory and work inventory remain available
to unmigrated purchasing/Finance consumers and read the same native profile records.
The Go gate is exercised by the compatibility suite, including financial decisions.
No new service or schema migration is added. Native failures never fall back to Go.
Browser writes require the same origin.

Run `bash frontend/scripts/verify-project-team.sh` with an isolated
`epcerp_auth_qa_team_*` database. The suite compares original Go HTTP responses and
persisted history in both directions, verifies live scope and eligibility revocation,
concurrent writes, rollback, bounded input, workspace/history limits and ownership
handover. Cloud calls and email are disabled for these checks.
