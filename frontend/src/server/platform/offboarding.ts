import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../db';
import { requireAnyPermission, assignmentIdentities, assignmentIdentity } from '../auth/access';
import { AppError } from '../errors';
import { body, json } from '../purchasing/http-input';
import { uuid, version } from '../purchasing/core/validation';
import { timeSQL, timestamp } from '../inbox/repository';
import { employees, validReason } from './organisation';
import { inventory, workView, type Work } from './inventory';
type Item = {
    Responsibility: Work;
    ReplacementID: string;
    ApprovedVersion: number;
};
type Case = {
    id: string;
    user_id: string;
    manager_id: string;
    profile_version: number;
    version: number;
    state: string;
    reason: string;
    items: Item[];
    approval_by_id: string;
    approved_at: string | null;
    created_at: string;
    updated_at: string;
};
const stale = () => new AppError(409, 'Handover or reporting information changed. Refresh the case and obtain manager approval again.');
const forbidden = () => new AppError(403, 'Permission denied');
const open = (c: Case) => ['planning', 'approved'].includes(c.state);
const same = (a: Work, b: Work) => a.Source === b.Source && a.Kind === b.Kind && a.ID === b.ID;
const exact = `${timeSQL('created_at')}AS created_at,${timeSQL('updated_at')}AS updated_at,${timeSQL('approved_at')}AS approved_at`;
const internal = (r: Record<string, unknown>) => ({
    ...r, version: Number(r.version), profile_version: Number(r.profile_version), approval_by_id: r.approval_by_id ?? '', created_at: timestamp(r.created_at as string), updated_at: timestamp(r.updated_at as string), approved_at: timestamp(r.approved_at as string)
} as Case);
const dto = (c: Case) => ({
    id: c.id, user_id: c.user_id, manager_id: c.manager_id, state: c.state, reason: c.reason, version: c.version, items: c.items.map(i => ({ work: workView(i.Responsibility), replacement_id: i.ReplacementID, approved_version: i.ApprovedVersion })), approval_by_id: c.approval_by_id, approved_at: c.approved_at, created_at: c.created_at, updated_at: c.updated_at
});
async function get(db: DB, id: string) {
    const r = (await db.query(`SELECT *,${exact} FROM auth.auth_offboarding_cases WHERE id=$1`, [id])).rows[0];
    if (!r)
        throw new AppError(404, 'Offboarding case not found.');
    return internal(r);
}
async function save(db: DB, c: Case, a: string, action: string, reason: string) {
    let r;
    try {
        r = (await db.query(`INSERT INTO auth.auth_offboarding_cases(id,user_id,manager_id,profile_version,state,reason,items,approval_by_id,approved_at,is_active,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)ON CONFLICT(id)DO UPDATE SET manager_id=EXCLUDED.manager_id,profile_version=EXCLUDED.profile_version,state=EXCLUDED.state,items=EXCLUDED.items,approval_by_id=EXCLUDED.approval_by_id,approved_at=EXCLUDED.approved_at,is_active=EXCLUDED.is_active,version=auth_offboarding_cases.version+1,updated_at=clock_timestamp(),updated_by_id=EXCLUDED.updated_by_id WHERE auth_offboarding_cases.version=$12 RETURNING *,${exact}`, [c.id, c.user_id, c.manager_id, c.profile_version, c.state, c.reason, JSON.stringify(c.items), c.approval_by_id || null, c.approved_at, open(c), a, c.version])).rows[0];
    }
    catch (e) {
        if ((e as {
            code?: string;
        }).code === '23505')
            throw new AppError(409, 'An open offboarding case already exists for this employee.');
        throw e;
    }
    if (!r)
        throw stale();
    const saved = internal(r), event = randomUUID();
    await db.query('INSERT INTO auth.auth_offboarding_changes(id,case_id,version,action,reason,snapshot,created_by_id)SELECT $1,c.id,c.version,$3,$4,to_jsonb(c),$5 FROM auth.auth_offboarding_cases c WHERE c.id=$2', [event, c.id, action, reason, a]);
    await db.query("INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)VALUES($1,'auth','offboarding',$2,$3,0,$4,$5,'user',$2,'platform','restricted',$6)", [event, c.id, saved.version, action, a, saved.updated_at]);
    return dto(saved);
}
async function verify(db: DB, c: Case, approved: boolean) {
    if ((await inventory(db, c.user_id)).length)
        throw new AppError(409, 'Complete or transfer all live responsibilities in their owning workspaces before handover approval and completion.');
    for (const i of c.items) {
        const current = await inventory(db, '', i.Responsibility);
        if (!current.length) {
            if (approved && i.ApprovedVersion !== -1)
                throw stale();
            continue;
        }
        if (!i.ReplacementID)
            throw new AppError(409, 'The manager must select a replacement for every live responsibility.');
        const selected = current.filter(w => w.OwnerID === i.ReplacementID);
        if (!selected.length)
            throw new AppError(409, 'A live responsibility has not been transferred to the manager-selected replacement.');
        const u = await assignmentIdentity(db, i.ReplacementID);
        if (!u?.active || !u.ready || u.id === c.user_id || selected.some(w => w.RequiredPermissions.some(p => !u.allows(p))))
            throw new AppError(409, 'A replacement no longer has the required active account and permissions.');
        if (approved && selected.some(w => w.Version !== i.ApprovedVersion))
            throw stale();
    }
}
export async function handleOffboarding(r: Request) {
    try {
        const p = new URL(r.url).pathname.replace(/^\/api\/v1\/auth\/organisation\/offboarding\/?/, '').split('/').filter(Boolean), token = r.headers.get('authorization')?.replace(/^Bearer /i, '') ?? '';
        return await transaction(async (db) => {
            const a = await requireAnyPermission(db, token, ['admin.offboarding.read', 'admin.offboarding.write', 'account.offboarding.read', 'account.offboarding.write'], true), all = await employees(db), admin = a.allows('admin.offboarding.read');
            const scope = (c: Case, managerOnly = false) => {
                const current = all.find(u => u.id === c.user_id);
                if (!current)
                    throw new AppError(404, 'User not found');
                const manager = a.id === current.manager_id && a.id === c.manager_id && a.id !== c.user_id;
                if (managerOnly ? !manager || !a.allows('account.offboarding.write') : !admin && !(manager && a.allows('account.offboarding.read')))
                    throw forbidden();
                return current;
            };
            if (!p.length && r.method === 'GET') {
                if (!admin && !a.allows('account.offboarding.read'))
                    throw forbidden();
                return json((await db.query(`SELECT c.*,${exact.split(',').map(x => x.replace(/\b(created_at|updated_at|approved_at) AT/g, 'c.$1 AT')).join(',')} FROM auth.auth_offboarding_cases c LEFT JOIN auth.auth_reporting_lines r ON r.user_id=c.user_id AND r.is_active WHERE $1 OR(c.manager_id=$2 AND r.manager_id=$2)ORDER BY c.created_at DESC,c.id LIMIT 1000`, [admin, a.id])).rows.map(internal).map(dto));
            }
            if (!p.length && r.method === 'POST') {
                if (!a.allows('admin.offboarding.write'))
                    throw forbidden();
                const b = await body(r, ['user_id', 'reason'], 16384), user = uuid(b.user_id);
                if (!validReason(b.reason))
                    throw new AppError(422, 'Provide a departure reason of 10 to 1000 characters.');
                if (user === a.id)
                    throw new AppError(409, 'You cannot deactivate your own account.');
                const profile = all.find(u => u.id === user);
                if (!profile)
                    throw new AppError(404, 'User not found');
                if (!profile.version || !profile.manager_id)
                    throw new AppError(409, "Set up the employee's reporting manager before starting offboarding. A company root needs an authorised successor reporting arrangement.");
                const m = await assignmentIdentity(db, profile.manager_id);
                if (!m?.active || !m.ready)
                    throw new AppError(409, 'The reporting manager must have an active, ready account.');
                const work = await inventory(db, user);
                return json(await save(db, {
                    id: randomUUID(), user_id: user, manager_id: profile.manager_id, profile_version: profile.version, version: 0, state: 'planning', reason: String(b.reason), items: work.map(w => ({ Responsibility: w, ReplacementID: '', ApprovedVersion: 0 })), approval_by_id: '', approved_at: null, created_at: '', updated_at: ''
                }, a.id, 'opened', String(b.reason)), 201);
            }
            const c = await get(db, uuid(p[0]));
            if (r.method === 'GET') {
                scope(c);
                if (p.length === 1)
                    return json(dto(c));
                if (p[1] === 'history')
                    return json((await db.query(`SELECT h.id,h.action,h.reason,h.created_by_id AS actor_id,coalesce(nullif(trim(u.first_name||' '||u.last_name),''),u.username)AS actor_name,h.version::int,${timeSQL('h.created_at')}AS created_at FROM auth.auth_offboarding_changes h JOIN auth.users u ON u.id=h.created_by_id WHERE h.case_id=$1 ORDER BY h.version DESC LIMIT 1000`, [c.id])).rows.map(h => ({ ...h, created_at: timestamp(h.created_at) })));
                if (p[1] === 'candidates') {
                    const out = [];
                    for (const u of await assignmentIdentities(db, []))
                        if (u.id !== c.user_id && u.ready) {
                            const permissions = (await db.query('SELECT DISTINCT p.code FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id AND r.is_active JOIN auth.role_permissions rp ON rp.role_id=r.id JOIN auth.permissions p ON p.id=rp.permission_id WHERE ur.user_id=$1 ORDER BY p.code', [u.id])).rows.map(x => x.code);
                            out.push({
                                id: u.id, name: u.name, permissions, is_superuser: u.superuser
                            });
                        }
                    return json(out);
                }
                return json({ detail: 'Not found' }, 404);
            }
            if (r.method !== 'POST' || p.length !== 2)
                return json({ detail: 'Not found' }, 404);
            const action = p[1], b = await body(r, ['version', 'reason', 'selections'], 65536);
            if (!validReason(b.reason) || !open(c) || version(b.version) !== c.version)
                throw stale();
            const administrative = ['refresh', 'cancel', 'complete'].includes(action), profile = administrative ? all.find(u => u.id === c.user_id) : scope(c, true);
            if (!profile)
                throw new AppError(404, 'User not found');
            if (['refresh', 'cancel', 'complete'].includes(action) && !a.allows('admin.offboarding.write'))
                throw forbidden();
            if (action === 'cancel')
                c.state = 'cancelled';
            else if (action === 'refresh') {
                const m = await assignmentIdentity(db, profile.manager_id);
                if (!m?.active || !m.ready)
                    throw new AppError(409, 'Assign an active, ready reporting manager before loading handover inventory.');
                for (const w of await inventory(db, c.user_id)) {
                    const i = c.items.find(i => same(i.Responsibility, w));
                    if (i)
                        i.Responsibility = w;
                    else
                        c.items.push({ Responsibility: w, ReplacementID: '', ApprovedVersion: 0 });
                }
                for (const i of c.items) {
                    const current = await inventory(db, '', i.Responsibility), w = current.find(w => w.OwnerID === i.ReplacementID) || (current.length === 1 ? current[0] : null);
                    if (w)
                        i.Responsibility = w;
                    i.ApprovedVersion = 0;
                }
                c.manager_id = profile.manager_id;
                c.profile_version = profile.version;
                c.state = 'planning';
                c.approval_by_id = '';
                c.approved_at = null;
            }
            else if (action === 'plan') {
                if (c.state !== 'planning' || c.profile_version !== profile.version)
                    throw stale();
                if (!Array.isArray(b.selections) || b.selections.length > 200)
                    throw new AppError(422, 'At most 200 updates per request');
                const seen = new Set<Item>();
                for (const s of b.selections) {
                    if (!s || typeof s !== 'object' || Array.isArray(s) || Object.keys(s).some(k=>!['source','kind','id','replacement_id'].includes(k)))
                        throw new AppError(422, 'Invalid handover selection');
                    const i = c.items.find(i => i.Responsibility.Source === s.source && i.Responsibility.Kind === s.kind && i.Responsibility.ID === s.id);
                    if (!i || seen.has(i))
                        throw new AppError(422, 'Choose each responsibility once from the saved inventory.');
                    seen.add(i);
                    const target = s.replacement_id ? uuid(s.replacement_id) : '';
                    if (target) {
                        const u = await assignmentIdentity(db, target);
                        if (!u?.active || !u.ready || target === c.user_id || i.Responsibility.RequiredPermissions.some(p => !u.allows(p)))
                            throw new AppError(422, 'Replacement must be active and already hold all required permissions.');
                    }
                    i.ReplacementID = target;
                }
            }
            else if (action === 'approve') {
                if (c.state !== 'planning' || c.profile_version !== profile.version)
                    throw stale();
                await verify(db, c, false);
                c.state = 'approved';
                c.approval_by_id = a.id;
                c.approved_at = new Date().toISOString();
                for (const i of c.items) {
                    const current = await inventory(db, '', i.Responsibility);
                    i.ApprovedVersion = current.find(w => w.OwnerID === i.ReplacementID)?.Version ?? -1;
                }
            }
            else if (action === 'complete') {
                if (c.state !== 'approved' || c.profile_version !== profile.version || c.manager_id !== profile.manager_id || c.approval_by_id !== profile.manager_id)
                    throw stale();
                const manager = await assignmentIdentity(db, c.manager_id);
                if (!manager?.active || !manager.ready)
                    throw stale();
                await verify(db, c, true);
                c.state = 'completed';
            }
            else
                throw new AppError(422, 'Unknown offboarding action.');
            return json(await save(db, c, a.id, action, String(b.reason)));
        });
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status);
        console.error('Offboarding operation unavailable');
        return json({ detail: 'Offboarding service unavailable' }, 500);
    }
}
