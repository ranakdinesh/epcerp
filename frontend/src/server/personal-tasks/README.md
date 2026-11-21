> Historical migration notes. Current runtime and setup: [frontend README](../../../README.md). The Go backend is no longer in the working tree.

# Native Personal Tasks

`EPCERP_PERSONAL_TASKS_OWNER=nextjs` selects the native `GET /my-tasks` dashboard.
Go omits that HTTP route under the same flag and rejects invalid ownership config.
Native failures never fall back to a second reader. This is a read-only projection;
there is no new task CRUD, completion endpoint, task table or schema migration.

- `service.ts`: current Auth capabilities, bounded assignment selection and page DTO.
- `validation.ts`: existing query bounds, literal search and India-date deadlines.
- `http.ts`: authenticated response/error boundary.
- `../purchasing/task-source.ts`: live released-indent, project, catalogue, client
  instruction and hold checks; cached once per package within the request.
- `../purchasing/quotation-tasks.ts`: current recipient plus purchase/Finance scope.
- `../master/document-task-reader.ts`: published supplier-review/correction read model.
- `../master/items/task-reader.ts`: published catalogue approval check, reusing native
  catalogue decoding, parent/unit/manufacturer eligibility and locks.

The dashboard requires both `work.tasks.view` and `work.tasks.read`. Even a
superuser sees only their own doer/resolver, quotation and supplier document tasks.
Source permissions and current project responsibility membership are rechecked.
No source workflow permission is inferred from the ability to open the dashboard.

Purchase preparation pagination/filtering covers steps 3–20, with blocked steps
first. Resolver assignments are shown only while blocked. A doer who is also the
blocked resolver is shown once, with doer precedence unless filtering by resolver.
Search treats %, _ and backslash literally. Quotation (latest 100 pending work
records before scope filtering) and supplier sections retain their independent
existing filtering/ordering; they are not silently folded into the preparation total.
Deadlines use the same fixed India date boundary and role-specific due date as Go.

Assigned historical work stays visible while its source is held/withdrawn/inactive,
with live availability and blocker reasons. Missing or malformed source evidence
fails the request rather than asserting that work is safe. Catalogue permission,
client instructions (including alternatives), active holds, source withdrawal and
requirement revision changes are checked on the consuming transaction.

Task actions/history stay with their purchasing and supplier modules. This migration
never approves a document, completes a task, spends capacity, sends messages or
calls an LLM. Existing passwords, permissions and encryption keys are unchanged.

Run `bash frontend/scripts/verify-personal-tasks.sh` against an isolated
`epcerp_auth_qa_tasks_*` database. The script applies owned migrations, uses synthetic
credentials and builds a Go oracle with the original transaction-bound source
readers. Tests compare full payloads and live permission/assignment/source changes.
They also check India midnight, literal searches, revocation, held/withdrawn sources,
scoped quotation responsibility and supplier correction/completion evidence.
