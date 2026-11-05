> Historical migration notes. Current runtime and setup: [frontend README](../../../README.md). The Go backend is no longer in the working tree.

# Native Project Master

`EPCERP_PROJECTS_OWNER=nextjs` owns Project Master list/detail, registration,
assignment options, manager transfer/history, draft metadata correction/history,
metadata options and both schedule-preview routes. Native path ownership is exact;
purchasing, budget, material, team and other project subresources keep their owners.
Go omits Project Master HTTP routes under the same flag and rejects invalid owners.
Published Go ContextReader, CalendarUpdater and work-inventory contracts remain for
unmigrated source modules and consume the same PostgreSQL records.

- `validation.ts`: bounded request values and compatible DTO types.
- `schedule.ts`: entered-date validation, working-calendar calculations and rounding.
- `repository.ts`: exact PostgreSQL dates/decimals/microseconds and Go-compatible
  metadata snapshots, manager history and business journal.
- `access.ts`: live Auth checks, portfolio/manager and scoped-assignment visibility.
- `service.ts`: transaction-bound creation, correction, handover and read commands.
- `http.ts`: bounded native route ownership and response contract.
- `../purchasing/project-registration.ts`: published initial procurement writer,
  joining the Project creation transaction without moving purchasing editors.

Creation atomically allocates the existing project number, inserts the master,
records initial PM history/event and initializes the Procurement Executive workspace,
history and event. Invalid procurement or failed audit insertion rolls everything
back; sequence gaps on rollback retain PostgreSQL/Go semantics. Name normalization
uses Go-compatible lowercase and existing unique constraints.

Live existing permissions remain required. Global manager-handover permission keeps
its original scope; ordinary metadata corrections require the current PM or project
portfolio-write scope. Project detail/history visibility includes effective specialist
responsibilities through current Auth role membership and dashboard permission.
Corrections are restricted to active draft masters and optimistic versions. The
technical legacy operations-manager identity is preserved and cannot be submitted
through the form. Existing inactive clients may be retained but not newly selected.
Lifecycle status is displayed; no new lifecycle mutation or bypass was invented.

Schedules keep the current contractual/planned/actual date distinction. Working days
exclude Sundays, second/fourth Saturdays and active holidays, with weekend precedence
for overlapping holidays. Percentage reserve rounds upward and execution days retain
the existing stored-column semantics. Unfinished owner previews remain supported.
Go calendar recalculation is intentionally retained and interoperates with these
native records, including its immutable schedule history.

No database migration, password reset, provider call, external message or new service
is introduced. Native failures never fall back to Go. Browser master writes require
the same origin. Existing purchasing/Finance/project-team actions remain separately
authorized in their owning modules.

`bash frontend/scripts/verify-projects.sh` requires a synthetic
`epcerp_auth_qa_projects_*` database. The Go HTTP oracle uses original Project,
Auth, Master, purchasing ownership, access and calendar readers/writers. Tests cover
bidirectional records/history, atomic rollback, scope/revocation, draft rules,
concurrent handovers, calendar previews/recalculation and retained legacy fields.
