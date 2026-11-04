#!/usr/bin/env bash
set -euo pipefail
umask 077
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root/frontend"
node -e 'if(!/\/epcerp_(auth|master)_qa_/.test(new URL(process.env.EPCERP_DATABASE_URL || "invalid").pathname))throw new Error("Provide an isolated epcerp_auth_qa_ or epcerp_master_qa_ database")'
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export EPCERP_AUTH_OWNER=nextjs EPCERP_CLIENTS_OWNER=nextjs EPCERP_REFERENCES_OWNER=nextjs EPCERP_ENVIRONMENT=test EPCERP_EMAIL_DELIVERY_ENABLED=false
export EPCERP_CATALOGUE_OWNER=nextjs
export EPCERP_VENDOR_PROFILES_OWNER=nextjs
export VTPL_JWT_SECRET_KEY=epcerp-synthetic-auth-compatibility-secret-only
export EPCERP_IMAGE_ENCRYPTION_KEY=1111111111111111111111111111111111111111111111111111111111111111
export VTPL_BOOTSTRAP_SUPERADMIN_USERNAME=superadmin VTPL_BOOTSTRAP_SUPERADMIN_EMAIL=admin@fixture.invalid
export VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD='Migration-Test1!'
cd "$root/backend"
go build -o "$work/migrate" ./cmd/migrate
go build -o "$work/auth-compatibility" ./tools/auth-compatibility
go build -o "$work/master-compatibility" ./tools/master-compatibility
go build -o "$work/catalogue-compatibility" ./tools/catalogue-compatibility
"$work/migrate" up
"$work/auth-compatibility" > "$work/go-auth.json"
export EPCERP_AUTH_COMPATIBILITY_FILE="$work/go-auth.json" EPCERP_AUTH_COMPATIBILITY_BINARY="$work/auth-compatibility"
export EPCERP_MASTER_COMPATIBILITY_BINARY="$work/master-compatibility"
export EPCERP_CATALOGUE_COMPATIBILITY_BINARY="$work/catalogue-compatibility"
cd "$root/frontend"
node scripts/sync-master-catalogue-sql.mjs --check
node --conditions=react-server --import tsx --test tests/server-master/*.test.ts tests/server-auth/*.test.ts
