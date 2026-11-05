#!/usr/bin/env bash
set -euo pipefail
umask 077
cd "$(dirname "$0")/.."
node -e 'if(!new URL(process.env.EPCERP_DATABASE_URL||"invalid").pathname.startsWith("/epcerp_auth_qa_retire_"))throw Error("Use a fresh isolated epcerp_auth_qa_retire_ database")'
export EPCERP_ENVIRONMENT=test EPCERP_EMAIL_DELIVERY_ENABLED=false EPCERP_TSC_CLOUD_APPROVED=false
export EPCERP_DOCUMENT_READING_KEY=4444444444444444444444444444444444444444444444444444444444444444
export EPCERP_IMAGE_ENCRYPTION_KEY=1111111111111111111111111111111111111111111111111111111111111111
export VTPL_JWT_SECRET_KEY=epcerp-synthetic-auth-compatibility-secret-only
export VTPL_JWT_ISSUER=epcERP VTPL_JWT_AUDIENCE=epcERP-api
export VTPL_BOOTSTRAP_SUPERADMIN_USERNAME=superadmin VTPL_BOOTSTRAP_SUPERADMIN_EMAIL=admin@fixture.invalid VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD='Migration-Test1!'
node tools/migrate.mjs up
node --conditions=react-server --import tsx --test --test-concurrency=1 tests/server-platform/workflow.test.ts tests/server-platform/migrations.test.mjs
