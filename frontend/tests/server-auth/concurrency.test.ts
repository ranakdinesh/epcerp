import assert from 'node:assert/strict';
import test from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { pool } from '../../src/server/db';
import { requireAnyPermission } from '../../src/server/auth/access';
import { login } from '../../src/server/auth/sessions';
import { sha256 } from '../../src/server/auth/security';

const database = process.env.EPCERP_DATABASE_URL;
test('business guards allow concurrency and fence committed session revocation', { skip: !database, timeout: 15000 }, async () => {
    assert.match(new URL(database!).pathname, /^\/epcerp_auth_qa_/, 'Use an isolated synthetic QA database');
    const sql = pool();
    const clients = await Promise.all([sql.connect(), sql.connect(), sql.connect()]);
    const [first, second, admin] = clients;
    let waiting: Promise<unknown> | undefined;
    try {
        const token = (await login('superadmin', process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token;
        for (const db of clients) {
            await db.query('BEGIN');
            await db.query("SET LOCAL statement_timeout='5s'");
        }
        await requireAnyPermission(first, token, ['projects.purchase_workflow.view'], 'business');
        await requireAnyPermission(second, token, ['work.tasks.view'], 'business');
        assert.equal((await admin.query("SELECT pg_try_advisory_xact_lock(hashtextextended('auth.administration',0)) AS acquired")).rows[0].acquired, false);
        assert.equal((await first.query("SELECT current_setting('epcerp.auth_actor') AS actor")).rows[0].actor,
            (await second.query("SELECT current_setting('epcerp.auth_actor') AS actor")).rows[0].actor);
        await assert.rejects(requireAnyPermission(second, 'invalid-token', ['work.tasks.view'], 'business'), { status: 401 });
        await first.query('COMMIT');
        await second.query('COMMIT');
        await requireAnyPermission(admin, token, ['work.tasks.view'], true);

        // A request arriving during administration must resolve credentials only
        // after the exclusive transaction commits, and observe its revocation.
        await first.query('BEGIN');
        await first.query("SET LOCAL statement_timeout='5s'");
        const pid = (await first.query('SELECT pg_backend_pid() AS pid')).rows[0].pid;
        waiting = assert.rejects(requireAnyPermission(first, token, ['work.tasks.view'], 'business'), { status: 401 });
        let blocked = false;
        for (let i = 0; i < 100; i++) {
            blocked = (await second.query("SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted) AS blocked", [pid])).rows[0].blocked;
            if (blocked) break;
            await delay(10);
        }
        assert.equal(blocked, true, 'Business authorization must wait behind account administration');
        await admin.query('UPDATE auth.access_sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1', [sha256(token)]);
        await admin.query('COMMIT');
        await waiting;
    } finally {
        // Release administration before joining a potentially waiting guard.
        await admin.query('ROLLBACK').catch(() => {});
        await waiting?.catch(() => {});
        for (const db of clients) {
            await db.query('ROLLBACK').catch(() => {});
            db.release();
        }
        await sql.end();
    }
});
