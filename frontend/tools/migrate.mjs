// Explicit operator CLI. The application never changes its schema at startup.
import pg from 'pg';
import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = process.env.EPCERP_MIGRATION_ROOT
  ?? fileURLToPath(new URL('../migrations/', import.meta.url));
const files = new Map();
let baseline;
for (const name of await readdir(root)) {
  const match = /^(\d+)_([^/]+)\.up\.sql$/.exec(name);
  if (!match) continue;
  const version = Number(match[1]);
  if (!Number.isSafeInteger(version) || version < 1 || files.has(version)) {
    throw Error('Invalid or duplicate migration version');
  }
  files.set(version, join(root, name));
  if (match[2] === 'baseline') {
    if (baseline !== undefined) throw Error('Multiple migration baselines');
    baseline = version;
  }
}
const versions = [...files.keys()].sort((a, b) => a - b);
const latest = versions.at(-1);
if (!baseline || versions.some((version, index) => version !== baseline + index)) {
  throw Error('Expected one baseline followed by sequential migrations');
}

const command = process.argv[2] ?? 'up';
if (command !== 'up' && command !== 'version') throw Error('Use up or version');
const connectionString = process.env.EPCERP_DATABASE_URL;
if (!connectionString) throw Error('Set EPCERP_DATABASE_URL to the existing application database');
const client = new pg.Client({
  connectionString,
  connectionTimeoutMillis: 10000,
  statement_timeout: 30000,
});

try {
  await client.connect();
  await client.query('SELECT pg_advisory_lock(835719413001)');
  const { rows: [relations] } = await client.query(`
    SELECT to_regclass('public.epcerp_schema_identity') AS identity,
           to_regclass('public.schema_migrations') AS history
  `);
  if (relations.identity) {
    const { rows: [identity] } = await client.query('SELECT project FROM public.epcerp_schema_identity WHERE singleton');
    if (identity?.project !== 'epcERP') throw Error('Unowned database');
  } else {
    const { rows: [{ populated }] } = await client.query(`
      SELECT EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
          AND left(table_schema, 3) <> 'pg_'
          AND NOT (table_schema='public' AND table_name='schema_migrations')
      ) AS populated
    `);
    if (populated) throw Error('Unowned database');
  }

  const history = relations.history
    ? (await client.query('SELECT version,dirty FROM public.schema_migrations')).rows
    : [];
  if (history.length > 1) throw Error('Ambiguous migration history');
  const row = history[0];
  if (row?.dirty) throw Error('Dirty migration history; operator recovery required');
  let current = Number(row?.version ?? 0);
  if (!Number.isSafeInteger(current) || current < 0) throw Error('Invalid migration version');
  if (current > latest) throw Error('Database newer than this checkout');
  if (Boolean(relations.identity) !== (current > 0)) throw Error('Inconsistent EPCERP schema identity');
  if (current > 0 && current < baseline) {
    console.error(`Schema version ${current} predates baseline ${baseline}; use the previous checkout to upgrade it first`);
    throw Error('Database predates this baseline');
  }

  if (command === 'up' && current < latest) {
    if (!relations.history) {
      await client.query('CREATE TABLE public.schema_migrations(version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL)');
    }
    for (const version of versions.filter(version => version > current)) {
      const sql = await readFile(files.get(version), 'utf8');
      await client.query('BEGIN');
      await client.query('DELETE FROM public.schema_migrations');
      await client.query('INSERT INTO public.schema_migrations VALUES($1,true)', [version]);
      await client.query('COMMIT');
      try {
        await client.query(sql);
        await client.query('UPDATE public.schema_migrations SET dirty=false WHERE version=$1', [version]);
        current = version;
      } catch {
        await client.query('ROLLBACK').catch(() => {});
        throw Error('Owned migration failed; history remains dirty');
      }
    }
  }

  if (command === 'version') console.log(current);
  else console.log('Owned schema verified at ' + current);
} catch {
  console.error('Migration refused or failed; inspect owned schema and private configuration');
  process.exitCode = 1;
} finally {
  await client.end().catch(() => {});
}
