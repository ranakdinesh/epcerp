#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repository_dir=$(dirname "$script_dir")
shared_environment="$repository_dir/deployment/epcerp/.env"
local_environment="$script_dir/.env.local"

if [ ! -f "$shared_environment" ]; then
  echo "Missing deployment/epcerp/.env; run deployment/epcerp/setup.sh first." >&2
  exit 1
fi
if [ ! -f "$local_environment" ]; then
  echo "Missing backend/.env.local." >&2
  exit 1
fi

set -a
. "$shared_environment"
. "$local_environment"
set +a

cd "$script_dir"
case "${1:-api}" in
  api)
    exec go run ./cmd/api
    ;;
  migrate-up)
    exec go run ./cmd/migrate up
    ;;
  migrate-version)
    exec go run ./cmd/migrate version
    ;;
  supplier-document-storage)
    exec go run ./cmd/supplier-document-storage
    ;;
  document-worker)
    exec go run ./cmd/document-worker
    ;;
  *)
    echo "Usage: ./run-local.sh [api|migrate-up|migrate-version|document-worker|supplier-document-storage]" >&2
    exit 2
    ;;
esac
