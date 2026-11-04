import "server-only";
import type { DB } from "../db";
import { authorize, resolve, loadUser } from "./repository";
import { allows, forbidden, type User } from "./domain";
import { actor, lock, rows } from "../db";
// Published contract for other server modules. Credential data stays inside Auth.
export async function requirePermission(db: DB, token: string, permission: string, write = false) {
    const user = await authorize(db, token, permission, write);
    return { id: user.id, is_superuser: user.is_superuser };
}
function capabilities(user: User) {
    return { id: user.id, active: user.is_active, superuser: user.is_superuser,
        allows: (permission: string) => allows(user, permission) };
}
export async function requireAnyPermission(db: DB, token: string, permissions: string[], write = false) {
    if (write)
        await lock(db, "auth.administration");
    const user = await resolve(db, token);
    if (!permissions.some(permission => allows(user, permission)))
        throw forbidden();
    if (write)
        await actor(db, user.id);
    return capabilities(user);
}
// Live directory contract: credential fields never leave Auth.
export async function userCapabilities(db: DB, id: string) {
    const user = await loadUser(db, id);
    // Match Go's AssignmentDirectory: Ready is separate from active role eligibility.
    // Interactive authorization still goes through resolve/requireAnyPermission above.
    return user ? { id: user.id, active: user.is_active, superuser: user.is_superuser, ready: !user.must_change_password,
        allows: (permission: string) => user.is_active && (user.is_superuser || user.roles.some(role => role.is_active && role.permissions.includes(permission))) } : null;
}
export async function documentReviewCandidates(db: DB) {
    const candidates = [];
    for (const { id } of await rows<{
        id: string;
    }>(db, "SELECT id FROM auth.users WHERE is_active AND NOT is_superuser ORDER BY id")) {
        const user = await userCapabilities(db, id);
        if (user && ["view", "read", "write", "auto_approve"].every(action => user.allows(`accounts.vendor_verification.${action}`)))
            candidates.push(id);
    }
    return candidates;
}
export async function financeReviewers(db: DB) {
    const candidates = [];
    for (const { id, username } of await rows<{
        id: string;
        username: string;
    }>(db, "SELECT id,CASE WHEN trim(first_name||' '||last_name)<>'' THEN trim(first_name||' '||last_name)||' ('||username||')' ELSE username END AS username FROM auth.users WHERE is_active ORDER BY auth.users.username")) {
        const user = await userCapabilities(db, id);
        if (user && ["view", "read", "write"].every(action => user.allows(`accounts.vendor_verification.${action}`)))
            candidates.push({ id, name: username });
    }
    return candidates;
}

export async function assignmentIdentity(db: DB, id: string, approvedBundles: string[] = []) {
    const user = await userCapabilities(db, id);
    if (!user) return null;
    const row = (await db.query(`SELECT CASE WHEN trim(first_name||' '||last_name)<>'' THEN trim(first_name||' '||last_name)||' ('||username||')' ELSE username END AS name,
      EXISTS(SELECT 1 FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id AND r.is_active WHERE ur.user_id=u.id AND r.id=ANY($2::uuid[])) AS designated
      FROM auth.users u WHERE u.id=$1`, [id, approvedBundles])).rows[0];
    return { ...user, name: row.name as string, designated: row.designated as boolean };
}
// One bounded directory query avoids loading each candidate's account/roles separately.
export async function assignmentIdentities(db: DB, approvedBundles: string[]) {
    const all = (await db.query(`SELECT u.id,u.is_active,u.is_superuser,NOT u.must_change_password AS ready,
      CASE WHEN trim(u.first_name||' '||u.last_name)<>'' THEN trim(u.first_name||' '||u.last_name)||' ('||u.username||')' ELSE u.username END AS name,
      EXISTS(SELECT 1 FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id AND r.is_active WHERE ur.user_id=u.id AND r.id=ANY($1::uuid[])) AS designated,
      COALESCE((SELECT jsonb_agg(DISTINCT p.code) FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id AND r.is_active JOIN auth.role_permissions rp ON rp.role_id=r.id JOIN auth.permissions p ON p.id=rp.permission_id WHERE ur.user_id=u.id),'[]'::jsonb) AS permissions
      FROM auth.users u WHERE u.is_active ORDER BY u.username LIMIT 10001`,[approvedBundles])).rows;
    if (all.length > 10000) throw new Error('Assignment directory exceeds reviewed bounds');
    return all.map(u => ({id:u.id as string,active:u.is_active as boolean,superuser:u.is_superuser as boolean,ready:u.ready as boolean,name:u.name as string,designated:u.designated as boolean,
      allows:(permission: string) => u.is_active && (u.is_superuser || u.permissions.includes(permission)) as boolean}));
}
