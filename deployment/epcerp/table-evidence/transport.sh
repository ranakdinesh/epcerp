#!/usr/bin/env bash
# Server deployment for tested images; no schema changes or cloud job submission.
set -Eeuo pipefail
umask 077
[[ $EUID == 0 ]] || { echo 'Run with sudo.'; exit 1; }
patch=${1:?Supply the transport patch directory}
archive="$patch/api"
expected=${2:?Supply its SHA256 digest}
[[ "$expected" =~ ^[a-f0-9]{64}$ ]] || exit 1
printf '%s  %s\n' "$expected" "$archive" | sha256sum --check
root=$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project.working_dir" }}' epcerp-dev-frontend-1)
[[ "$root" == /home/vtplerp/epcerp-releases/dev-20261102-1-amd64 ]] || { echo 'Release changed; inspect source compatibility first.'; exit 1; }
cd "$root"
exec 9>/run/lock/epcerp-deployment.lock
flock -n 9 || { echo 'Another deployment is active.'; exit 1; }
backup="$root/pre-table-transport-$(date -u +%Y%m%dT%H%M%SZ)-$$"
mkdir -m 700 "$backup"
for file in compose.yaml Caddyfile .env .env.gemini release.env; do
 [[ ! -f "$file" ]] || cp -p "$file" "$backup/"
done
chmod 600 "$backup/"*
docker exec epcerp-dev-db-1 pg_dump -U epcerp -d epcerp -Fc > "$backup/database.dump"
docker exec -i epcerp-dev-db-1 pg_restore --list < "$backup/database.dump" > /dev/null
docker exec epcerp-dev-db-1 psql -U epcerp -d epcerp -Atc 'select version,dirty from public.schema_migrations' > "$backup/schema-before.txt"
grep -Fxq '98|f' "$backup/schema-before.txt" || { echo 'Schema changed; inspect before deploying.'; exit 1; }
active=$(docker exec epcerp-dev-db-1 psql -U epcerp -d epcerp -Atc "select count(*) from tender.tsc_jobs where state in ('pending','processing');")
[[ "$active" == 0 ]] || { echo "TSC work is active; retry deployment after it finishes."; exit 1; }
docker build --network none -t epcerp-api:table-evidence-20261103-2-amd64 "$patch"
files=()
[[ ! -f release.env ]] || files+=(--env-file release.env)
files+=(--env-file .env)
compose=(docker compose -p epcerp-dev "${files[@]}" -f compose.yaml)
docker exec -i epcerp-dev-frontend-1 node -e '
const fs=require("fs"),c=JSON.parse(fs.readFileSync(0,"utf8"));
c.services.api.image="epcerp-api:table-evidence-20261103-2-amd64";
c.services["tsc-worker"].image=c.services.api.image;
c.services["document-worker"].image="epcerp-document-worker:table-evidence-20261103-1-amd64";
c.services["document-package-worker"].image=c.services["document-worker"].image;
c.services.frontend.image="epcerp-frontend:table-evidence-20261103-1-amd64";
c.services["frontend-lan"].image=c.services.frontend.image;
process.stdout.write(JSON.stringify(c,null,2)+"\n");
' < compose.yaml > "$backup/compose.prepared.json"
rollback() {
 result=$?; trap - ERR
 cat "$backup/compose.yaml" > compose.yaml
 "${compose[@]}" up -d --no-deps --wait --wait-timeout 180 api tsc-worker document-worker document-package-worker frontend frontend-lan || true
 echo "Deployment failed; prior image configuration restored. Backup: $backup" >&2
 exit "$result"
}
trap rollback ERR
cat "$backup/compose.prepared.json" > compose.yaml
export COMPOSE_PROGRESS=plain COMPOSE_ANSI=never
"${compose[@]}" config --quiet
"${compose[@]}" up -d --no-deps --wait --wait-timeout 180 api document-worker document-package-worker
"${compose[@]}" up -d --no-deps --wait --wait-timeout 180 tsc-worker
"${compose[@]}" up -d --no-deps --wait --wait-timeout 180 frontend frontend-lan
curl --noproxy '*' --fail --silent --show-error --max-time 20 -o /dev/null 'http://192.168.1.50/login?session=expired'
curl --noproxy '*' --fail --silent --show-error --resolve erp.vishaltechnopower.in:443:127.0.0.1 --max-time 20 -o /dev/null 'https://erp.vishaltechnopower.in/login?session=expired'
docker exec epcerp-dev-db-1 psql -U epcerp -d epcerp -Atc 'select version,dirty from public.schema_migrations' > "$backup/schema-after.txt"
cmp "$backup/schema-before.txt" "$backup/schema-after.txt"
sha256sum compose.yaml Caddyfile > TABLE-EVIDENCE-SHA256SUMS
trap - ERR
printf 'Table transport correction deployed. Private rollback/database backup: %s\n' "$backup"
echo 'Database/schema/documents, provider credentials and LAN/public HTTPS routes preserved.'
