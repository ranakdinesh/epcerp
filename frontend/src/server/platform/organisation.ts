import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../db';
import { requireAnyPermission, assignmentIdentity } from '../auth/access';
import { AppError } from '../errors';
import { body, json } from '../purchasing/http-input';
import { uuid, version, trim } from '../purchasing/core/validation';
import { timeSQL, timestamp } from '../inbox/repository';
import { journal } from '../purchasing/core/journal';
import { now } from '../purchasing/core/time';
import { inventory, workView } from './inventory';
export const validReason = (v: unknown) => typeof v === 'string' && [...v.trim()].length >= 10 && [...v.trim()].length <= 1000;
export function contact(b: Record<string, unknown>) {
    const first = typeof b.first_name === 'string' ? trim(b.first_name) : '', last = typeof b.last_name === 'string' ? trim(b.last_name) : '', mobile = typeof b.mobile === 'string' ? trim(b.mobile) : '';
    if (!first || !last || [...first].length > 100 || [...last].length > 100 || /[\p{Cc}]/u.test(first + last) || !/^\+?[\d ()-]{8,24}$/.test(mobile) || mobile.replace(/\D/g, '').length < 8 || mobile.replace(/\D/g, '').length > 15)
        throw new AppError(422, 'First name, last name and valid mobile are required');
    return { first_name: first, last_name: last, mobile };
}
export async function employees(db: DB) {
    return (await db.query("SELECT u.id,u.username,u.email,u.first_name,u.last_name,u.mobile,u.is_active,coalesce(r.department_id::text,'')AS department_id,coalesce(r.manager_id::text,'')AS manager_id,coalesce(r.is_company_root,false)AS is_company_root,coalesce(r.version,0)::int AS version FROM auth.users u LEFT JOIN auth.auth_reporting_lines r ON r.user_id=u.id AND r.is_active ORDER BY u.username,u.id")).rows;
}
export function reports(all: Awaited<ReturnType<typeof employees>>, id: string, manager: string) {
    if (id === manager)
        return false;
    const seen = new Set<string>();
    while (id && !seen.has(id)) {
        seen.add(id);
        const p = all.find(r => r.id === id);
        if (!p?.is_active)
            return false;
        if (p.manager_id === manager)
            return true;
        id = p.manager_id;
    }
    return false;
}
export async function orgEvent(db: DB, id: string, kind: string, actor: string, reason: string, event = randomUUID()) {
    const table = kind === 'department' ? 'auth.auth_departments' : 'auth.auth_reporting_lines', key = kind === 'department' ? 'id' : 'user_id';
    const row = (await db.query(`SELECT to_jsonb(r)AS snapshot,version FROM ${table} r WHERE ${key}=$1`, [id])).rows[0];
    if (!row)
        return;
    let snapshot = row.snapshot;
    if (kind === 'employee') {
        const u = (await db.query('SELECT first_name,last_name,mobile FROM auth.users WHERE id=$1', [id])).rows[0];
        snapshot = { ...snapshot, ...u };
    }
    await db.query('INSERT INTO auth.auth_organisation_changes(id,entity_id,entity_type,version,reason,snapshot,actor_id)VALUES($1,$2,$3,$4,$5,$6,$7)', [event, id, kind, row.version, reason, snapshot, actor]);
    await journal(db, {
        id: event, module: 'auth', type: kind, entity: id, version: Number(row.version), action: 'organisation_updated', actor, project: null, at: now()
    });
}
async function departments(db: DB) {
    return (await db.query(`SELECT id,code,name,coalesce(manager_id::text,'')AS manager_id,is_active,version::int,${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')}AS updated_at FROM auth.auth_departments ORDER BY name,id`)).rows.map(r => ({ ...r, created_at: timestamp(r.created_at), updated_at: timestamp(r.updated_at) }));
}
export async function handleOrganisation(r: Request) {
    try {
        const p = new URL(r.url).pathname.replace(/^\/api\/v1\/auth\/organisation\/?/, '').split('/').filter(Boolean), token = r.headers.get('authorization')?.replace(/^Bearer /i, '') ?? '', write = r.method !== 'GET';
        if (p[0] === 'offboarding')
            return (await import('./offboarding')).handleOffboarding(r);
        return await transaction(async (db) => {
            const team = p[0] === 'team', a = await requireAnyPermission(db, token, [team ? 'account.team.read' : write ? 'admin.organisation.write' : 'admin.organisation.read'], true), all = await employees(db);
            if (r.method === 'GET') {
                if (p[0] === 'departments')
                    return json(await departments(db));
                if (p[0] === 'employees')
                    return json(all);
                if (team && p.length === 1)
                    return json(all.filter(u => reports(all, u.id, a.id)));
                if (team && p.length === 3 && p[2] === 'work') {
                    const id = uuid(p[1]);
                    if (!reports(all, id, a.id))
                        throw new AppError(403, 'Permission denied');
                    return json((await inventory(db, id)).map(workView));
                }
                return json({ detail: 'Not found' }, 404);
            }
            if (p[0] === 'departments' && (r.method === 'POST' || r.method === 'PATCH')) {
                const b = await body(r, ['code', 'name', 'manager_id', 'is_active', 'version', 'reason'], 16384), code = typeof b.code === 'string' ? trim(b.code).toUpperCase() : '', name = typeof b.name === 'string' ? trim(b.name) : '', manager = b.manager_id ? uuid(b.manager_id) : '', v = version(b.version), active = b.is_active;
                if (!/^[A-Z\d_-]{2,32}$/.test(code) || [...name].length < 2 || [...name].length > 150 || typeof active !== 'boolean' || !validReason(b.reason))
                    throw new AppError(422, 'Provide department code, name, version and a reason of 10 to 1000 characters.');
                const list = await departments(db), id = p[1] ? uuid(p[1]) : randomUUID(), before = list.find(d => d.id === id);
                if (p[1] && !before || before && before.version !== v || !p[1] && v !== 0)
                    throw new AppError(409, 'Organisation record changed. Refresh and review before saving.');
                if (list.some(d => d.id !== id && d.code === code))
                    throw new AppError(409, 'Department code is already in use.');
                if (manager && !(await assignmentIdentity(db, manager))?.active)
                    throw new AppError(422, 'Department manager must be an active employee.');
                if (!active && (manager || all.some(u => u.is_active && u.department_id === id)))
                    throw new AppError(409, 'Move active employees and remove the manager before deactivating this department.');
                const row = before ? await db.query('UPDATE auth.auth_departments SET code=$2,name=$3,manager_id=$4,is_active=$5,version=version+1,updated_at=clock_timestamp(),updated_by_id=$6 WHERE id=$1 AND version=$7', [id, code, name, manager || null, active, a.id, v]) : await db.query('INSERT INTO auth.auth_departments(id,code,name,manager_id,is_active,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$6)', [id, code, name, manager || null, active, a.id]);
                if (row.rowCount !== 1)
                    throw new AppError(409, 'Organisation record changed.');
                await orgEvent(db, id, 'department', a.id, String(b.reason));
                return json((await departments(db)).find(d => d.id === id));
            }
            if (p[0] === 'employees' && p.length === 2 && r.method === 'PATCH') {
                const b = await body(r, ['first_name', 'last_name', 'mobile', 'department_id', 'manager_id', 'is_company_root', 'version', 'reason'], 16384), id = uuid(p[1]), c = contact(b), department = uuid(b.department_id), manager = b.manager_id ? uuid(b.manager_id) : '', v = version(b.version), before = all.find(u => u.id === id);
                if (!before)
                    throw new AppError(404, 'User not found');
                if (before.version !== v)
                    throw new AppError(409, 'Organisation record changed. Refresh and review before saving.');
                if (typeof b.is_company_root !== 'boolean' || !validReason(b.reason) || manager === id || b.is_company_root && manager || !b.is_company_root && !manager)
                    throw new AppError(422, 'Select one reporting manager, or explicitly mark the employee as a company root.');
                if (!(await departments(db)).some(d => d.id === department && d.is_active))
                    throw new AppError(422, 'Select an active department.');
                if (manager && (!all.some(u => u.id === manager && u.is_active && u.version > 0) || reports(all, manager, id)))
                    throw new AppError(422, 'Set up an active reporting manager without a circular hierarchy.');
                const changed = await db.query('INSERT INTO auth.auth_reporting_lines(user_id,department_id,manager_id,is_company_root,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$5)ON CONFLICT(user_id)DO UPDATE SET department_id=EXCLUDED.department_id,manager_id=EXCLUDED.manager_id,is_company_root=EXCLUDED.is_company_root,version=auth_reporting_lines.version+1,updated_at=clock_timestamp(),updated_by_id=EXCLUDED.updated_by_id WHERE auth_reporting_lines.version=$6', [id, department, manager || null, b.is_company_root, a.id, v]);
                if (changed.rowCount !== 1)
                    throw new AppError(409, 'Organisation record changed.');
                await db.query('UPDATE auth.users SET first_name=$2,last_name=$3,mobile=$4,updated_by_id=$5,updated_at=clock_timestamp()WHERE id=$1', [id, c.first_name, c.last_name, c.mobile, a.id]);
                await orgEvent(db, id, 'employee', a.id, String(b.reason));
                return json((await employees(db)).find(u => u.id === id));
            }
            return json({ detail: 'Not found' }, 404);
        });
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status);
        console.error('Organisation operation unavailable');
        return json({ detail: 'Organisation service unavailable' }, 500);
    }
}
