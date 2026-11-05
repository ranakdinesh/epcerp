#!/usr/bin/env bash
set -euo pipefail
umask 077
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root/frontend"
node -e 'if(!new URL(process.env.EPCERP_DATABASE_URL || "invalid").pathname.startsWith("/epcerp_auth_qa_tsc_"))throw new Error("Provide an isolated epcerp_auth_qa_tsc_ database")'
export EPCERP_AUTH_OWNER=nextjs EPCERP_TSC_OWNER=nextjs EPCERP_ENVIRONMENT=test EPCERP_EMAIL_DELIVERY_ENABLED=false
export EPCERP_DOCUMENT_READING_KEY=4444444444444444444444444444444444444444444444444444444444444444
export EPCERP_IMAGE_ENCRYPTION_KEY=1111111111111111111111111111111111111111111111111111111111111111
export VTPL_JWT_SECRET_KEY=epcerp-synthetic-auth-compatibility-secret-only
export VTPL_JWT_ISSUER=epcERP VTPL_JWT_AUDIENCE=epcERP-api
export VTPL_BOOTSTRAP_SUPERADMIN_USERNAME=superadmin VTPL_BOOTSTRAP_SUPERADMIN_EMAIL=admin@fixture.invalid VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD='Migration-Test1!'
# Fetch is injected in all provider tests; these are deliberately invalid credentials.
export EPCERP_TSC_CLOUD_APPROVED=true EPCERP_TSC_GEMINI_API_KEY=synthetic-test-key EPCERP_TSC_OPENAI_API_KEY=synthetic-test-key
export EPCERP_TSC_GEMINI_TIER=paid EPCERP_TSC_USD_INR=95
unset EPCERP_TSC_TEST_PROVIDER_URL EPCERP_TSC_TENDER_LIMIT_PAISE EPCERP_TSC_DAILY_LIMIT_PAISE
node tools/migrate.mjs up
cd "$root/frontend"
node --conditions=react-server --import tsx --test --test-concurrency=1 tests/server-tsc/*.test.ts
