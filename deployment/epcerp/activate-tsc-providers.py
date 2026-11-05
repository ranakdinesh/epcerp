#!/usr/bin/env python3
"""Activate operator-supplied TSC provider keys without changing the verified image."""
import json
import os
from pathlib import Path
import subprocess

os.umask(0o077)
root = Path(__file__).resolve().parents[2]
os.chdir(root)
credentials = root / 'deployment/epcerp/.env.providers'
if not credentials.is_file() or credentials.stat().st_mode & 0o077:
    raise SystemExit('Create owner-only deployment/epcerp/.env.providers (chmod 600).')
allowed = {'EPCERP_TSC_GROQ_API_KEY', 'EPCERP_TSC_DEEPSEEK_API_KEY'}
values = {}
for line in credentials.read_text().splitlines():
    line = line.strip()
    if not line or line.startswith('#'):
        continue
    name, separator, value = line.partition('=')
    if not separator or name not in allowed:
        raise SystemExit('Provider credentials file contains an unsupported setting.')
    values[name] = value.strip().strip('\"\'')
if not any(values.values()):
    raise SystemExit('No provider key is configured. Add your own Groq or DeepSeek key locally.')

def output(args):
    return subprocess.check_output(args, text=True).strip()

compose = ['docker', 'compose', '--env-file', 'deployment/epcerp/.env', '--env-file', 'deployment/epcerp/.env.gemini', '--env-file', str(credentials), '-f', 'deployment/epcerp/compose.yaml']
config = json.loads(output(compose + ['config', '--format', 'json']))
container = json.loads(output(['docker', 'inspect', 'epcerp-local-frontend-1']))[0]
environment = dict(item.split('=', 1) for item in container['Config']['Env'])
expected = config['services']['frontend']['environment']
protected = ['EPCERP_DATABASE_URL', 'VTPL_JWT_SECRET_KEY', 'EPCERP_DOCUMENT_READING_KEY', 'EPCERP_IMAGE_ENCRYPTION_KEY', 'EPCERP_SUPPLIER_DOCUMENT_KEY', 'EPCERP_TSC_CLOUD_APPROVED', 'EPCERP_TSC_GEMINI_API_KEY', 'EPCERP_TSC_OPENAI_API_KEY']
if any(environment.get(key, '') != expected.get(key, '') for key in protected):
    raise SystemExit('Existing runtime keys or controls differ. Deploy the verified integration first.')
if expected.get('EPCERP_TSC_CLOUD_APPROVED') != 'true':
    raise SystemExit('Cloud processing is disabled by the existing approval control.')
if output(['git', 'branch', '--show-current']) != 'codex/development':
    raise SystemExit('Activate only the combined codex/development preview.')
if output(['docker', 'image', 'inspect', 'epcerp-frontend:local', '--format', '{{.Id}}']) != container['Image']:
    raise SystemExit('A different frontend image is pending. Deploy and verify it before activating keys.')
psql = ['docker', 'exec', 'epcerp-local-db-1', 'psql', '-U', 'epcerp', '-d', 'epcerp', '-Atc']
if output(psql + ['SELECT version,dirty FROM schema_migrations']) != '101|f':
    raise SystemExit('The verified schema 101 integration must be running first.')
if output(psql + ["SELECT count(*)FROM tender.tsc_workspace_tasks WHERE state IN('queued','processing')"]) != '0':
    raise SystemExit('A TSC request is active; wait for it to finish before activation.')
subprocess.run(compose + ['up', '-d', '--no-build', '--no-deps', '--wait', 'frontend'], check=True)
print('Configured providers: ' + ', '.join('Groq' if 'GROQ' in key else 'DeepSeek' for key, value in values.items() if value))
print('No model request was sent. Choose the provider and confirm documents in Step 2.')
