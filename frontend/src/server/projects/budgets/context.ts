import type { DB } from '../../db';
import { requireAnyPermission, assignmentIdentities, assignmentBundles, assignedBundleIDs } from '../../auth/access';
import { projectAccess } from '../../purchasing/core/project-access';
import { project } from '../../purchasing/core/access';
import * as bo from '../../purchasing/bo/repository';
import { invalid, missing } from './validation';
import { scoped } from '../team/validation';
import type { Owner, Item } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.budgets.read'], true);
export type Actor = Awaited<ReturnType<typeof actor>>;
export async function context(db: DB, target: string, lock = false) {
    const p = await project(db, target, lock), row = (await db.query('SELECT currency,contract_value::text FROM projects WHERE id=$1', [p.id])).rows[0], client = (await db.query('SELECT is_active FROM clients WHERE id=$1', [p.client_id])).rows[0];
    if (!row)
        throw missing();
    return { p, project: {
            id: p.id, name: p.name, currency: row.currency as string, contract_value: row.contract_value as string | null, available: p.is_active && !!client?.is_active && !['closed', 'on_hold'].includes(p.lifecycle_status)
        } };
}
export async function capabilities(db: DB, a: Actor, p: Awaited<ReturnType<typeof project>>) {
    const check = async (code: string) => {
        try {
            await projectAccess(db, a, p, code, true);
            return true;
        }
        catch (e) {
            const error = e as {
                status?: number;
                field?: string;
            };
            if (error.status === 403 || error.status === 422 && error.field === 'project')
                return false;
            throw e;
        }
    };
    return { write: await check('projects.budgets.write'), approve: await check('projects.budgets.approve') };
}
export async function sources(db: DB, p: Awaited<ReturnType<typeof project>>) {
    const users = await assignmentIdentities(db, []), profiles = (await db.query('SELECT user_id,bundle_id,permission_codes,is_active FROM project_responsibility_profiles WHERE project_id=$1 ORDER BY bundle_id,id LIMIT 1001', [p.id])).rows;
    if (profiles.length > 1000)
        invalid('profiles', 'Project responsibility limit reached');
    const executive = (await db.query('SELECT purchase_manager_id FROM purchase_fms_workspaces WHERE project_id=$1 AND is_active', [p.id])).rows[0]?.purchase_manager_id;
    const owners: Owner[] = [];
    for (const u of users) {
        let eligible = u.superuser || u.id === p.project_manager_id || u.id === executive;
        if (!eligible) {
            const relevant = profiles.filter(profile => profile.is_active && profile.user_id === u.id);
            if (!relevant.length)
                continue;
            const memberships = await assignedBundleIDs(db, u.id);
            for (const profile of relevant) {
                if (!profile.is_active || profile.user_id !== u.id || !memberships.includes(profile.bundle_id))
                    continue;
                const [bundle] = await assignmentBundles(db, profile.bundle_id);
                if (bundle?.is_active && profile.permission_codes.some((code: string) => scoped(code) && u.allows(code) && bundle.permissions.includes(code))) {
                    eligible = true;
                    break;
                }
            }
        }
        if (eligible)
            owners.push({ id: u.id, name: u.name });
    }
    const items: Item[] = [];
    for (const b of await bo.list(db, p.id)) {
        if (!b.is_active)
            continue;
        for (const l of b.lines) {
            const s = l.item_snapshot;
            items.push({
                bo_id: b.id, line_id: l.id, bo_version: b.version, bo_name: b.name, quantity: l.quantity ?? '', sku: s.sku, description: s.description, uom: s.uom, manufacturer: s.manufacturer, item_id: s.id, item_snapshot: {
                    sku: s.sku, description: s.description, uom: s.uom, manufacturer: s.manufacturer
                }
            });
            if (items.length > 1000)
                invalid('items', 'At most 1000 purchase items per project budget');
        }
    }
    return { owners, items };
}
