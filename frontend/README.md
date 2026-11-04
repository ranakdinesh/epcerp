# epcERP frontend

Go application frontend using Next.js 16.3.4 and React 19.2.8.

## Run

```sh
npm ci
npm run dev
```

Local frontend port: 3001. Upstream is server-only EPCERP_API_URL; production must
configure it explicitly. Native development loads the ignored `.env.local`, which
targets the Go API at `http://127.0.0.1:8001/api/v1`. There is no `BACKEND_API_URL`
fallback. Go API/database/bootstrap must be configured separately using
../backend/README.md.
Enable Go Auth, Clients and references for the visible workspaces.

Cookie names are epcerp_access_token and epcerp_refresh_token. They remain HttpOnly,
SameSite=Lax, Secure in production. For explicitly local HTTP production testing only,
EPCERP_ENVIRONMENT=development and EPCERP_DEVELOPMENT_HTTP=true enable the retained
development exception. Cookies are isolated by name; separate production hostnames
and deployment secrets are still required. Never put backend secrets in NEXT_PUBLIC_*.

## Current integration boundary

Login, logout, refresh, protected layout, Users, Roles, Clients, client types,
industries, states and cities use existing Go contracts. They still require a running
Go API and appropriate permissions. Manufacturers proxy is retained; its copied
editor needs Catalogue APIs, so its navigation remains unpublished.

Projects, Tender/Purchase FMS, Finance, notifications, messaging and timeline are
planned. Their old routes were excluded; copied source components/tests remain
available for staged adaptation. No project data or approval counts are simulated.
See ../docs/GO_COMMUNICATION_GOVERNANCE_SPINE.md for workflow requirements.

```
npm test
npx tsc --noEmit
npm run lint
EPCERP_API_URL=http://127.0.0.1:8001/api/v1 npm run build
```

Build uses webpack explicitly to keep source/dependency resolution inside this
independent project. Production is not deployed by creating this clone. Existing
copied workflow tests cover source helpers and do not establish live Go integration.

## Native authentication and authorization

The local Docker preview runs Auth in Next.js with the existing PostgreSQL schema,
password hashes, signing keys and sessions. Set `EPCERP_AUTH_OWNER=nextjs` on both
Next.js and Go. Go retains live permission enforcement for its remaining business
features. See [the Auth module guide](src/server/auth/README.md) for configuration,
the exact migration boundary, isolated tests and rollback.

The complete Auth verification runner is `bash frontend/scripts/verify-native-auth.sh`
from the repository root with an isolated `EPCERP_DATABASE_URL` whose database name
starts with `epcerp_auth_qa_`. No real email is sent by its tests.

## Shared Master Data

Clients is the first migrated area inside `src/server/master`. States/cities,
client types, industries, items and vendors will share this module as they migrate;
their current Go operations continue working. Clients reuses the existing database
and permission codes. See [the module guide](src/server/master/README.md).

Local Compose sets `EPCERP_CLIENTS_OWNER=nextjs` on both applications. Set it to
`go` on both to roll back only Clients while keeping native Auth. Use
`bash frontend/scripts/verify-master.sh` from the repo root with a dedicated QA
database for compatibility/SQL tests. The browser mutation verifier is restricted
to the isolated QA port 59092.
