#!/bin/sh
# Load the same private configuration for setup, updates and operational commands.
set -eu
cd "$(dirname "$0")"
set -- -f compose.yaml "$@"
if [ -f .env.providers ]; then
  set -- --env-file .env.providers "$@"
fi
if [ -f .env.gemini ]; then
  set -- --env-file .env.gemini "$@"
fi
exec docker compose --project-directory "$PWD" --env-file .env "$@"
