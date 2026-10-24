#!/bin/sh
set -eu
cd "$(dirname "$0")"
# Preserve existing credentials so a rebuild retains the same database/login.
if [ ! -f .env ]; then
  command -v openssl >/dev/null
  db_password=$(openssl rand -hex 24)
  jwt_secret=$(openssl rand -hex 48)
  admin_password=$(openssl rand -hex 16)
  umask 077
  (
    set -C
    {
      printf 'EPCERP_DB_PASSWORD=%s\n' "$db_password"
      printf 'EPCERP_JWT_SECRET=%s\n' "$jwt_secret"
      printf 'EPCERP_ADMIN_PASSWORD=Aa1!%s\n' "$admin_password"
    } > .env
  )
fi
# Preserve this Go-only key across rebuilds so imported evidence remains readable.
if ! grep -q '^EPCERP_SUPPLIER_DOCUMENT_KEY=' .env; then
  supplier_document_key=$(openssl rand -hex 32)
  umask 077
  printf 'EPCERP_SUPPLIER_DOCUMENT_KEY=%s\n' "$supplier_document_key" >> .env
fi
if ! grep -q '^EPCERP_DOCUMENT_READING_KEY=' .env; then
  document_reading_key=$(openssl rand -hex 32)
  umask 077
  printf 'EPCERP_DOCUMENT_READING_KEY=%s\n' "$document_reading_key" >> .env
fi
if ! grep -q '^EPCERP_STORAGE_SECRET=' .env; then
  storage_secret=$(openssl rand -hex 32)
  umask 077
  printf 'EPCERP_STORAGE_SECRET=%s\n' "$storage_secret" >> .env
fi
if ! grep -q '^EPCERP_IMAGE_ENCRYPTION_KEY=' .env; then
  umask 077
  image_key=$(openssl rand -hex 32)
  printf 'EPCERP_IMAGE_ENCRYPTION_KEY=%s\n' "$image_key" >> .env
fi
docker compose --env-file .env -f compose.yaml up --build -d --wait
