#!/bin/sh
set -eu
cd "$(dirname "$0")"
# Display only on the user's explicit local invocation; never copy into logs/docs.
printf 'URL: http://localhost:9092\nUsername: superadmin\n'
awk -F= '$1 == "EPCERP_ADMIN_PASSWORD" { printf "Password: %s\n", $2 }' .env
