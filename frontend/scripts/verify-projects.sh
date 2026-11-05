#!/usr/bin/env bash
set -euo pipefail
umask 077
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root/frontend"
node -e 'if(!new URL(process.env.EPCERP_DATABASE_URL||"invalid").pathname.startsWith("/epcerp_auth_qa_projects_"))throw Error("Use an isolated epcerp_auth_qa_projects_ database")'
export TZ=UTC EPCERP_AUTH_OWNER=nextjs EPCERP_PROJECTS_OWNER=nextjs EPCERP_ENVIRONMENT=test EPCERP_EMAIL_DELIVERY_ENABLED=false
export EPCERP_DOCUMENT_READING_KEY=4444444444444444444444444444444444444444444444444444444444444444 EPCERP_SUPPLIER_DOCUMENT_KEY=3333333333333333333333333333333333333333333333333333333333333333 EPCERP_IMAGE_ENCRYPTION_KEY=1111111111111111111111111111111111111111111111111111111111111111
export VTPL_JWT_SECRET_KEY=epcerp-synthetic-auth-compatibility-secret-only VTPL_JWT_ISSUER=epcERP VTPL_JWT_AUDIENCE=epcERP-api
export VTPL_BOOTSTRAP_SUPERADMIN_USERNAME=superadmin VTPL_BOOTSTRAP_SUPERADMIN_EMAIL=admin@fixture.invalid VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD='Migration-Test1!'
export EPCERP_TSC_CLOUD_APPROVED=false
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$root/backend"
go run ./cmd/migrate up
go build -o "$work/projects-oracle" ./tools/projects-compatibility
export EPCERP_PROJECTS_ORACLE="$work/projects-oracle"
cd "$root/frontend"
node --conditions=react-server --import tsx --test --test-concurrency=1 tests/server-projects/*.test.ts
