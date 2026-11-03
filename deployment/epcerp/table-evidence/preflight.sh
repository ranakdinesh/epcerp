#!/usr/bin/env bash
# Synthetic PDFs only, with networking disabled. No database or provider access.
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../../.." && pwd)
qa=${1:-"$root/backend/tools/pdf-tables"}
for image in epcerp-api:table-evidence-20261103-1-amd64 epcerp-document-worker:table-evidence-20261103-1-amd64; do
 expected=$(sha256sum "$qa/extract.py" | cut -d ' ' -f 1)
 actual=$(docker run --rm --network none --entrypoint sha256sum "$image" /app/pdf_tables.py | cut -d ' ' -f 1)
 [[ "$actual" == "$expected" ]] || { echo 'Table helper revision mismatch'; exit 1; }
 docker run --rm --network none --read-only --tmpfs /tmp:rw,nosuid,noexec,size=128m --memory 1500m --cpus 2 -v "$qa:/qa:ro" --entrypoint /opt/pdf-tables/bin/python "$image" -m unittest discover -s /qa -v
 done
# Prove the Go reader can launch its bounded helper with a sanitized environment.
docker run --rm -i --network none --read-only --tmpfs /tmp:rw,nosuid,noexec,size=128m --memory 1500m --cpus 2 -v "$qa:/qa:ro" --entrypoint /opt/pdf-tables/bin/python epcerp-document-worker:table-evidence-20261103-1-amd64 - <<'PY'
import sys,tempfile,subprocess,json,hashlib
sys.path.insert(0,'/qa')
from test_extract import pdf,text
raw=pdf('40 600 520 120 re S\n200 600 m 200 720 l S\n'+text(45,660,'Validity of Offer (Days)')+text(205,660,'60'))
with tempfile.NamedTemporaryFile(suffix='.pdf') as f:
 f.write(raw);f.flush()
 result=subprocess.run(['/app/document-reader','--input',f.name,'--format','pdf','--sha256',hashlib.sha256(raw).hexdigest()],check=True,capture_output=True,timeout=45)
 reading=json.loads(result.stdout)
 assert not reading.get('warnings'), 'Go reader table helper unavailable'
 cells=reading['segments'][0]['tables'][0]['cells']
 assert cells[0]['text']=='Validity of Offer (Days)' and cells[1]['text']=='60'
print('PASS: native Go reader and bounded PDF helper, offline synthetic document')
PY
