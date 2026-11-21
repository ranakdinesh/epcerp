import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readdir, readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import pg from 'pg';

const database = process.env.EPCERP_DATABASE_URL;
const fixtureName = /^epcerp_auth_qa_retire_[a-z0-9_]+$/;
const migrations = new URL('../../migrations/', import.meta.url);

test('owned SQL bytes match recorded provenance', async () => {
  const history = JSON.parse(await readFile(new URL('history.json', migrations), 'utf8'));
  assert.ok(Object.hasOwn(history, '000119_baseline.up.sql'));
  const files = (await readdir(migrations)).filter(name => name.endsWith('.up.sql'));
  assert.deepEqual(files.sort(), Object.keys(history).sort());
  for (const [name, entry] of Object.entries(history)) {
    const data = await readFile(new URL(name, migrations));
    assert.equal(createHash('sha256').update(data).digest('hex'), entry.sha256, name);
  }
});

test('Node migrator refuses unsafe schema states and applies owned SQL', { skip: !database }, async (t) => {
  const name = decodeURIComponent(new URL(database).pathname.slice(1));
  assert.match(name, fixtureName, 'Use a dedicated, empty native verification database');

  const db = new pg.Client({ connectionString: database });
  await db.connect();
  try {
    const { rows: [target] } = await db.query(`
      SELECT current_database() AS name,
        EXISTS (
          SELECT FROM pg_namespace
          WHERE nspname NOT IN ('public', 'information_schema') AND left(nspname, 3) <> 'pg_'
        ) OR EXISTS (
          SELECT FROM pg_class WHERE relnamespace = 'public'::regnamespace
        ) AS populated
    `);
    assert.equal(target.name, name);
    assert.equal(target.populated, false, 'The native verification database must be empty');

    const run = () => {
      const result = spawnSync(process.execPath, [
        fileURLToPath(new URL('../../tools/migrate.mjs', import.meta.url)), 'up',
      ], {
        env: { ...process.env, EPCERP_MIGRATION_ROOT: fileURLToPath(migrations) },
        encoding: 'utf8',
      });
      assert.ifError(result.error);
      assert.equal(result.signal, null);
      return result;
    };

    await t.test('unowned tables are refused before creating migration metadata', async () => {
      await db.query('CREATE TABLE private_unknown(value text)');
      try {
        assert.equal(run().status, 1);
        assert.equal((await db.query("SELECT to_regclass('schema_migrations') AS name")).rows[0].name, null);
      } finally {
        await db.query('DROP TABLE private_unknown');
      }
    });

    await t.test('fresh owned history reaches 121 and repeats without DDL changes', async () => {
      const first = run();
      assert.equal(first.status, 0, first.stderr);
      assert.deepEqual((await db.query('SELECT version,dirty FROM schema_migrations')).rows, [
        { version: '121', dirty: false },
      ]);
      const second = run();
      assert.equal(second.status, 0, second.stderr);
      const { rows: [{ count }] } = await db.query(
        "SELECT count(*) FROM information_schema.tables WHERE table_schema IN ('public','auth','tender')",
      );
      assert.equal(Number(count), 210);
      const { rows: [relations] } = await db.query(`
        SELECT to_regclass('public.business_events')::text AS journal,
               to_regclass('public.event_deliveries') AS deliveries,
               to_regclass('public.event_delivery_attempts') AS attempts
      `);
      assert.deepEqual(relations, { journal: 'business_events', deliveries: null, attempts: null });
    });

    await t.test('dirty, newer, and pre-baseline fixture states fail closed', async () => {
      const { rows: [original] } = await db.query('SELECT version,dirty FROM schema_migrations');
      assert.deepEqual(original, { version: '121', dirty: false });
      try {
        await db.query('UPDATE schema_migrations SET dirty=true');
        assert.equal(run().status, 1);
        assert.equal((await db.query('SELECT dirty FROM schema_migrations')).rows[0].dirty, true);
        await db.query('UPDATE schema_migrations SET dirty=false,version=122');
        assert.equal(run().status, 1);
        assert.equal((await db.query('SELECT version FROM schema_migrations')).rows[0].version, '122');
        await db.query('UPDATE schema_migrations SET version=118');
        assert.match(run().stderr, /Schema version 118 predates baseline 119/);
        assert.equal((await db.query('SELECT version FROM schema_migrations')).rows[0].version, '118');
      } finally {
        await db.query('UPDATE schema_migrations SET version=$1, dirty=$2', [original.version, original.dirty]);
      }
    });
  } finally {
    await db.end();
  }
});
