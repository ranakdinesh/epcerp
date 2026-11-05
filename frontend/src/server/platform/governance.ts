import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../db';
import { requireAnyPermission, assignmentIdentity, assignmentIdentities } from '../auth/access';
import { AppError } from '../errors';
import { timeSQL, timestamp } from '../inbox/repository';
import { body, json } from '../purchasing/http-input';
import { object, uuid, reason, version, invalid } from '../purchasing/core/validation';
import { journal } from '../purchasing/core/journal';
const exact = `${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at`;
const view = (r: Record<string, unknown>) => ({
    id: r.id as string, user_id: r.user_id as string, version: Number(r.version), is_active: r.is_active === true, created_at: timestamp(r.created_at as string)!, updated_at: timestamp(r.updated_at as string)!, created_by_id: r.created_by_id as string | null, updated_by_id: r.updated_by_id as string | null
});
const user = async (db: DB, id: string) => {
    const u = await assignmentIdentity(db, id);
    return u ? { id: u.id, name: u.name, is_active: u.active } : null;
};
async function read(db: DB, write: boolean) {
    const row = (await db.query(`SELECT *,${exact} FROM organisation_director_designation WHERE slot=true`)).rows[0], record = row ? view(row) : null, u = record ? await user(db, record.user_id) : null;
    return {
        record, user: u, is_effective: !!(record?.is_active && u?.is_active), can_write: write, history: (await db.query(`SELECT id,action,reason,before_snapshot AS before,after_snapshot AS after,created_by_id,${timeSQL('created_at')} AS created_at FROM organisation_director_events ORDER BY created_at DESC,id DESC LIMIT 200`)).rows.map(r => ({ ...r, created_at: timestamp(r.created_at)! }))
    };
}
export async function workspace(token: string) {
    return transaction(async (db) => {
        const a = await requireAnyPermission(db, token, ['admin.governance.read'], true);
        await db.query("SELECT pg_advisory_xact_lock(hashtext('epcerp.organisation.director'))");
        return read(db, a.allows('admin.governance.write'));
    });
}
export async function options(token: string) {
    return transaction(async (db) => {
        const a = await requireAnyPermission(db, token, ['admin.governance.read'], true);
        if (!a.allows('admin.governance.write'))
            throw new AppError(403, 'Governance permission denied');
        await db.query("SELECT pg_advisory_xact_lock(hashtext('epcerp.organisation.director'))");
        const rows = await assignmentIdentities(db, [], 1001);
        if (rows.length > 1000)
            invalid('users', 'Director account choices exceed 1000; use a bounded administrator directory');
        return rows.map(u => ({ id: u.id, name: u.name, is_active: u.active }));
    });
}
export async function save(token: string, raw: Record<string, unknown>) {
    return transaction(async (db) => {
        const a = await requireAnyPermission(db, token, ['admin.governance.read'], true);
        if (!a.allows('admin.governance.write'))
            throw new AppError(403, 'Governance permission denied');
        object(raw, ['user_id', 'version', 'is_active', 'reason']);
        const target = uuid(raw.user_id), v = version(raw.version), why = reason(raw.reason);
        if (typeof raw.is_active !== 'boolean')
            invalid('is_active', 'Use true or false');
        await db.query("SELECT pg_advisory_xact_lock(hashtext('epcerp.organisation.director'))");
        const before = (await read(db, true)).record, candidate = await user(db, target);
        if (!candidate || raw.is_active && !candidate.is_active)
            invalid('user_id', 'Choose an active account for Director designation');
        if (!raw.is_active && (!before || before.user_id !== target))
            invalid('user_id', 'Retain the current Director when revoking the designation');
        if (v !== (before?.version ?? 0))
            throw new AppError(409, 'Director designation changed; reload before saving');
        if (!before && !raw.is_active)
            invalid('is_active', 'Designate an active Director before revoking');
        if (before?.user_id === target && before.is_active === raw.is_active)
            invalid('user_id', 'No designation change to save');
        const action = !before ? 'designate' : !raw.is_active ? 'revoke' : !before.is_active && before.user_id === target ? 'reactivate' : 'change';
        const row = before ? (await db.query(`UPDATE organisation_director_designation SET user_id=$1,is_active=$2,version=version+1,updated_at=clock_timestamp(),updated_by_id=$3 WHERE slot=true AND version=$4 RETURNING *,${exact}`, [target, raw.is_active, a.id, v])).rows[0] : (await db.query(`INSERT INTO organisation_director_designation(id,user_id,version,is_active,created_by_id,updated_by_id)VALUES($1,$2,1,$3,$4,$4)RETURNING *,${exact}`, [randomUUID(), target, raw.is_active, a.id])).rows[0];
        if (!row)
            throw new AppError(409, 'Director designation changed; reload before saving');
        const after = view(row), id = randomUUID();
        await db.query('INSERT INTO organisation_director_events(id,designation_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8)', [id, after.id, action, why, JSON.stringify(before), JSON.stringify(after), after.updated_at, a.id]);
        await journal(db, {
            id, module: 'organisation_governance', type: 'director_designation', entity: after.id, version: after.version, action, actor: a.id, project: null, at: after.updated_at
        });
        return read(db, true);
    });
}
export async function handleGovernance(r: Request) {
    try {
        const token = r.headers.get('authorization')?.replace(/^Bearer /i, '') ?? '', path = new URL(r.url).pathname;
        if (r.method === 'GET')
            return json(path.endsWith('/options') ? await options(token) : await workspace(token));
        if (r.method === 'POST' && !path.endsWith('/options'))
            return json(await save(token, await body(r, ['user_id', 'version', 'is_active', 'reason'], 16384)));
        return json({ detail: 'Not found' }, 404);
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status);
        console.error('Governance operation unavailable');
        return json({ detail: 'Internal server error' }, 500);
    }
}
