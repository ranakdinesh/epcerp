# Native-only source cleanup — 5 November 2026

Removed the retired ERP Go backend (1,624 files), API/worker build definitions,
local-model/reranker and old evaluator/deployment scripts. Current Compose has
only the native frontend, Node schema CLI, private Node document processor and
required Postgres/MinIO/ClamAV infrastructure. No schema, credentials, database or
storage volumes are deleted by this source cleanup.

Required Word/SQL/PDF test assets now live under `frontend/`. All 101 owned SQL
migration files retain their original bytes and provenance. SQL/template sync
checks no longer read a backend checkout. New installations use native ownership
by default; server startup rejects retired ownership, and unmapped requests never
fall back to a Go/network writer.

The already-deployed Tender/provider/Word work was committed first at
`a18ae634c00fd194fe00ed7bbe9f303bad9b39b9` so no ongoing changes were lost.
Optional historical parity tests extract that exact reference into a temporary
folder. No ERP Go source is present in the current tree, and normal builds and
native checks do not require Go. Frozen synthetic Argon2id/i credential fixtures
also verify old password compatibility directly in Node.

Past migration/deployment reports are labelled under `history/`; current setup
instructions describe only the native application. The MinIO vendor Dockerfile
still builds pinned upstream storage source with its upstream toolchain. It is
required infrastructure, not an ERP Go backend.

Validation: 242 frontend tests, 14 Node migration/platform integration checks,
48 cloud/manual TSC and Word-import checks with synthetic provider responses,
42 historical purchasing parity checks and 23 historical Auth checks passed.
Native-only Auth security checks passed (historical/database cases are opt-in).
TypeScript, scoped ESLint, production build, canonical SQL/template consistency,
Compose validation and offline PDF extraction tests passed. All database tests
used separate QA databases; no model requests or real messages were sent.

Changes are prepared on `codex/development`. Main remains the reviewed release
branch. Git history is retained for recovery; no history rewrite or force push
is part of this cleanup. Existing development-server deployment remains intact.

A clean committed checkout at `720a639e95ace563ad432634835a291a59408050`
contains no backend directory or private env files and passed a fresh production
build. Frontend/Node migrator and processor images were built from that checkout.
The idle local Docker preview was aligned to those images with its entire
existing runtime environment preserved. Readiness passed; schema stayed 101,
password hashes and model-call count were unchanged, and no migration or provider
request ran during that preview update. The remote development server was not
changed again as part of this Git cleanup.
