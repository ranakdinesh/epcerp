import 'server-only';
import type { DB } from '../../db';
import { transaction } from '../../db';
import { requireAnyPermission, assignmentIdentity, assignmentIdentities } from '../../auth/access';
import { limitPublic } from '../../auth/rate-limit';
import { AppError } from '../../errors';
export const capturePermissions = ['tenders.register.view','tenders.register.read','tenders.register.write','tenders.processing.write','tenders.register.assign'];
export const ownerPermissions = ['tenders.register.view','tenders.register.read','tenders.register.write','tenders.processing.read','tenders.processing.write'];
export const managerPermissions = [...ownerPermissions,'tenders.register.assign','tenders.evaluation.read','tenders.evaluation.write','communication.inbox.read','communication.inbox.write'];
export const forbidden = (): never => { throw new AppError(403, 'Permission denied'); };
export function approvedBundles() {
    const raw = process.env.EPCERP_TENDER_MANAGER_BUNDLE_IDS?.trim();
    const ids = raw ? raw.split(',').map(s => s.trim().toLowerCase()) : [];
    if (ids.length > 20 || new Set(ids).size !== ids.length || ids.some(s => !/^[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}$/.test(s))) throw new Error('Invalid EPCERP_TENDER_MANAGER_BUNDLE_IDS');
    return ids;
}
export const directory = (db: DB, id: string) => assignmentIdentity(db, id, approvedBundles());
export type Identity = NonNullable<Awaited<ReturnType<typeof directory>>>;
export type Actor = Awaited<ReturnType<typeof access>>;
export function canManage(u: Identity | null) { return !!u?.active && u.ready && u.designated && managerPermissions.every(p => u.allows(p)); }
export async function access(db: DB, token: string, permissions = ['tenders.register.view','tenders.register.read']) {
    // Same lock as Go's Auth administration guard. Check live capabilities on every operation/replay.
    const actor = await requireAnyPermission(db, token, [permissions[0]], true);
    if (!permissions.every(p => actor.allows(p))) forbidden();
    return actor;
}
export async function admit(token: string, operation: 'capture' | 'read' | 'assignment', permissions?: string[]) {
    const actor = await transaction(db => access(db, token, permissions));
    await limitPublic(`tender:register:${operation}:${actor.id}`, operation === 'capture' ? 10 : operation === 'assignment' ? 20 : 120, 60);
}
export async function candidates(db: DB, legacy = false) {
    if (!legacy && !approvedBundles().length) return [];
    const out: { id: string; display_name: string }[] = [];
    for (const u of await assignmentIdentities(db, approvedBundles())) {
        const id = u.id;
        if (u && (legacy ? ownerPermissions.every(p => u.allows(p)) : canManage(u))) out.push({ id, display_name: legacy ? u.name : [...u.name].slice(0,255).join('') });
    }
    return out;
}
