#!/usr/bin/env bash
set -euo pipefail
umask 077
root=$(cd "$(dirname "$0")/../../.." && pwd)
cd "$root/frontend"
# Never migrate or seed an application database through a test runner.
node -e 'if(!/\/epcerp_auth_qa_/.test(new URL(process.env.EPCERP_DATABASE_URL || "invalid").pathname))throw new Error("Provide an isolated epcerp_auth_qa_ database")'
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export EPCERP_AUTH_OWNER=nextjs EPCERP_ENVIRONMENT=test EPCERP_EMAIL_DELIVERY_ENABLED=false
export VTPL_JWT_SECRET_KEY=epcerp-synthetic-auth-compatibility-secret-only
export EPCERP_IMAGE_ENCRYPTION_KEY=1111111111111111111111111111111111111111111111111111111111111111
export VTPL_BOOTSTRAP_SUPERADMIN_USERNAME=superadmin VTPL_BOOTSTRAP_SUPERADMIN_EMAIL=admin@fixture.invalid
export VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD='Migration-Test1!'
node "$root/frontend/tools/migrate.mjs" up
# This opt-in oracle is frozen migration evidence, not an application dependency.
reference=a18ae634c00fd194fe00ed7bbe9f303bad9b39b9
git -C "$root" archive "$reference" backend | tar -x -C "$work"
export EPCERP_LEGACY_BACKEND="$work/backend"
cd "$work/backend"
go build -o "$work/compatibility" ./tools/auth-compatibility
"$work/compatibility" > "$work/go-auth.json"
export EPCERP_AUTH_COMPATIBILITY_FILE="$work/go-auth.json" EPCERP_AUTH_COMPATIBILITY_BINARY="$work/compatibility"
cd "$root/frontend"
npm run test:auth
