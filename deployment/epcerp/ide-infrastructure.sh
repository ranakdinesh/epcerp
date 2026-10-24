#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ ! -f "$script_dir/.env" ]; then
  echo "Missing deployment/epcerp/.env; run setup.sh first." >&2
  exit 1
fi

cd "$script_dir"
case "${1:-start}" in
  start)
    docker compose --env-file .env -f compose.yaml stop api frontend document-worker
    docker compose --env-file .env -f compose.yaml up -d --wait db storage
    ;;
  status)
    docker compose --env-file .env -f compose.yaml ps db storage
    ;;
  stop)
    docker compose --env-file .env -f compose.yaml stop db storage
    ;;
  *)
    echo "Usage: ./ide-infrastructure.sh [start|status|stop]" >&2
    exit 2
    ;;
esac
