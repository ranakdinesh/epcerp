import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { requireAnyPermission, assignmentIdentity, assignmentIdentities, assignmentBundles, assignedBundleIDs } from '../../auth/access';
import { timeSQL, timestamp } from '../../inbox/repository';
import { lower } from '../../tenders/intake/receipts';
import { denied, missing, stale, invalid, id, input, scoped, TeamError } from './validation';
import * as repo from './repository';
type Actor = Awaited<ReturnType<typeof actor>>;
const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.team.read'], 'business');
async function project(db: DB, target: string, lock = false) {
    const row = (await db.query(`SELECT id,name,project_manager_id,is_active,lifecycle_status AS lifecycle FROM projects WHERE id=$1${lock ? ' FOR UPDATE' : ''}`, [target])).rows[0];
    if (!row) throw missing();
    const owner = (await db.query('SELECT purchase_manager_id FROM purchase_fms_workspaces WHERE project_id=$1 AND is_active', [target])).rows[0];
    return { id: String(row.id), name: String(row.name), project_manager_id: String(row.project_manager_id), procurement_executive_id: String(owner?.purchase_manager_id ?? ''), is_active: row.is_active === true, lifecycle: String(row.lifecycle) };
}
const manage = (a: Actor, p: Awaited<ReturnType<typeof project>>) => a.allows('projects.team.write') && (a.superuser || a.allows('projects.portfolio.write') || a.id === p.project_manager_id);
async function readable(db: DB, a: Actor, p: Awaited<ReturnType<typeof project>>) {
    if (a.superuser || a.allows('projects.portfolio.read') || a.allows('projects.portfolio.write') || a.id === p.project_manager_id) return;
    const profiles = await repo.list(db, p.id);
    const memberships = await assignedBundleIDs(db, a.id);
    for (const profile of profiles) {
        if (!profile.is_active || profile.user_id !== a.id || !profile.permission_codes.includes('projects.team.read') || !memberships.includes(profile.bundle_id)) continue;
        const [bundle] = await assignmentBundles(db, profile.bundle_id);
        if (bundle?.is_active && bundle.permissions.includes('projects.team.read')) return;
    }
    throw denied();
}
export async function workspace(token: string, target: string) {
    return transaction(async db => {
        const a = await actor(db, token), p = await project(db, id(target));
        await readable(db, a, p);
        const profiles = await repo.list(db, p.id);
        for (const profile of profiles) {
            profile.user_name = (await assignmentIdentity(db, profile.user_id))?.name ?? '';
            profile.bundle_name = (await assignmentBundles(db, profile.bundle_id))[0]?.name ?? '';
        }
        return { project: p, profiles, can_write: manage(a, p) };
    });
}
export async function options(token: string, target: string) {
    return transaction(async db => {
        const a = await actor(db, token), p = await project(db, id(target));
        if (!manage(a, p)) throw denied();
        const users = await assignmentIdentities(db, [], 1001), roles = await assignmentBundles(db);
        if (users.length > 1000) invalid('users', 'Assignment directory exceeds the supported bound');
        if (roles.length > 1000) invalid('bundles', 'Role directory exceeds the supported bound');
        const out = [];
        for (const u of users) out.push({ id: u.id, name: u.name, is_active: u.active, bundle_ids: await assignedBundleIDs(db, u.id) });
        const bundles = roles.filter(r => r.is_active).map(r => ({ ...r, permissions: r.permissions.filter(scoped) })).filter(r => r.permissions.length);
        return { users: out, bundles };
    });
}
export async function save(token: string, target: string, profileID: string | undefined, raw: Record<string, unknown>) {
    return transaction(async db => {
        const a = await actor(db, token), targetID = id(target), recordID = profileID === undefined ? undefined : id(profileID), b = input(raw), p = await project(db, targetID, true);
        if (!manage(a, p)) throw denied();
        const before = recordID ? await repo.get(db, targetID, recordID) : null;
        if (!before && (await repo.list(db, targetID)).length >= 1000) invalid('profiles', 'Project responsibility limit reached');
        if (!before && b.version !== 0 || before && (before.version !== b.version || before.user_id !== b.user_id || before.bundle_id !== b.bundle_id)) throw stale();
        if (b.is_active) {
            if (!p.is_active || ['closed', 'on_hold'].includes(p.lifecycle)) invalid('project', 'Assign responsibilities only in an active open project');
            const user = await assignmentIdentity(db, b.user_id);
            if (!user?.active) invalid('user_id', 'Choose an active account');
            const [bundle] = await assignmentBundles(db, b.bundle_id);
            if (!bundle?.is_active || !(await assignedBundleIDs(db, b.user_id)).includes(b.bundle_id)) invalid('bundle_id', 'Choose an active role currently assigned to this account');
            if (b.permission_codes.some(code => !user.allows(code) || !bundle.permissions.includes(code))) invalid('permission_codes', "Profile permissions must remain within this account's assigned role");
        } else if (!before) invalid('is_active', 'Create an active responsibility before revoking it');
        let row;
        try {
            row = before ? (await db.query(`UPDATE project_responsibility_profiles SET responsibility=$3,responsibility_key=$4,permission_codes=$5,is_active=$6,version=version+1,updated_at=clock_timestamp(),updated_by_id=$7 WHERE project_id=$1 AND id=$2 AND version=$8 RETURNING *,${repo.exact}`, [targetID, recordID, b.responsibility, lower(b.responsibility), JSON.stringify(b.permission_codes), b.is_active, a.id, b.version])).rows[0]
                : (await db.query(`INSERT INTO project_responsibility_profiles(id,project_id,user_id,bundle_id,responsibility,responsibility_key,permission_codes,is_active,version,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,true,1,$8,$8) RETURNING *,${repo.exact}`, [randomUUID(), targetID, b.user_id, b.bundle_id, b.responsibility, lower(b.responsibility), JSON.stringify(b.permission_codes), a.id])).rows[0];
        } catch (error) {
            if ((error as { code?: string }).code === '23505') throw new TeamError(409, 'An active profile for this user, role and responsibility already exists');
            throw error;
        }
        if (!row) throw stale();
        const after = repo.profile(row), event = randomUUID(), action = !b.is_active ? 'revoke' : before ? 'revise' : 'assign';
        await db.query('INSERT INTO project_responsibility_events(id,profile_id,project_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8)', [event, after.id, p.id, action, b.reason, JSON.stringify(before), JSON.stringify(after), a.id]);
        await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,scope_id,visibility,occurred_at)VALUES($1,'project_access','project_responsibility',$2,$3,0,$4,$5,'user',$1,'project',$6,'restricted',$7)`, [event, after.id, after.version, action, a.id, p.id, after.updated_at]);
        return after;
    });
}
export async function history(token: string, target: string, profileID: string) {
    return transaction(async db => {
        const a = await actor(db, token), p = await project(db, id(target)), recordID = id(profileID);
        await readable(db, a, p); await repo.get(db, p.id, recordID);
        return (await db.query(`SELECT *,${timeSQL('created_at')} AS created_at FROM project_responsibility_events WHERE project_id=$1 AND profile_id=$2 ORDER BY project_responsibility_events.created_at DESC,id DESC LIMIT 200`, [p.id, recordID])).rows.map(r => ({ id: r.id, profile_id: r.profile_id, project_id: r.project_id, action: r.action, reason: r.reason, before: r.before_snapshot, after: r.after_snapshot, created_by_id: r.created_by_id ?? '00000000-0000-0000-0000-000000000000', created_at: timestamp(r.created_at) }));
    });
}
