#!/usr/bin/env bash
set -euo pipefail
umask 077
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root/frontend"
node -e 'if(!new URL(process.env.EPCERP_DATABASE_URL||"invalid").pathname.startsWith("/epcerp_auth_qa_purchase_"))throw Error("Use an isolated epcerp_auth_qa_purchase_ database")'
export EPCERP_PROJECT_BUDGETS_OWNER=go EPCERP_PURCHASING_OWNER=go EPCERP_PERSONAL_TASKS_OWNER=go
export TZ=UTC EPCERP_AUTH_OWNER=nextjs EPCERP_PROJECT_TEAM_OWNER=nextjs EPCERP_ENVIRONMENT=test EPCERP_EMAIL_DELIVERY_ENABLED=false
export EPCERP_DOCUMENT_READING_KEY=4444444444444444444444444444444444444444444444444444444444444444 EPCERP_SUPPLIER_DOCUMENT_KEY=3333333333333333333333333333333333333333333333333333333333333333 EPCERP_IMAGE_ENCRYPTION_KEY=1111111111111111111111111111111111111111111111111111111111111111
export VTPL_JWT_SECRET_KEY=epcerp-synthetic-auth-compatibility-secret-only VTPL_JWT_ISSUER=epcERP VTPL_JWT_AUDIENCE=epcERP-api
export VTPL_BOOTSTRAP_SUPERADMIN_USERNAME=superadmin VTPL_BOOTSTRAP_SUPERADMIN_EMAIL=admin@fixture.invalid VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD='Migration-Test1!'
export EPCERP_TSC_CLOUD_APPROVED=false EPCERP_VENDOR_VERIFICATION_OWNER=nextjs
export VTPL_STORAGE_ENDPOINT=127.0.0.1:1 VTPL_STORAGE_ACCESS_KEY=synthetic-purchase-storage VTPL_STORAGE_SECRET_KEY=synthetic-purchase-storage-secret VTPL_STORAGE_BUCKET=synthetic-purchasing
lockdir="${TMPDIR:-/tmp}/epcerp-budget-qa.lock"
if ! mkdir "$lockdir" 2>/dev/null; then echo 'Another budget QA run is active' >&2;exit 1;fi
work=$(mktemp -d)
trap 'rm -rf "$work"; rmdir "$lockdir"' EXIT
cd "$root/backend"
go run ./cmd/migrate up
go build -o "$work/purchasing-oracle" ./tools/purchasing-compatibility
export EPCERP_PURCHASING_ORACLE="$work/purchasing-oracle"
cd "$root/frontend"
node --conditions=react-server --import tsx --test --test-concurrency=1 tests/server-project-budgets/*.test.ts
