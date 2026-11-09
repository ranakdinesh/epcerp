import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import { project, type Actor } from '../core/access';
import { inspect, stepAccess } from '../workflow/reader';
import { scoped } from '../sourcing/context';
import { released } from '../orders/reader';
import type { Source, Capabilities } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_acceptance.read'], 'business');
export async function access(db: DB, a: Actor, p: string, id: string): Promise<Capabilities> { await project(db, p, true); if (!a.allows('projects.purchase_acceptance.read'))
    return { read: false, write: false, scoped: false, work: false }; const scope = await scoped(db, a, p, 'projects.purchase_acceptance.read', false), task = await stepAccess(db, a, p, id, 10), read = scope || task.read; return { read, write: read && a.allows('projects.purchase_acceptance.write'), scoped: scope, work: task.work }; }
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { project_id: p, package_id: id, available: current.project_available && current.eligible, blockers: [...current.blockers], order: null }; for (const s of record.steps)
    if (s.number === 10 && s.state === 'blocked')
        out.blockers.push('Vendor acceptance has an unresolved blocker'); let o; try {
    o = await released(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 422].includes(e.status)) {
        out.available = false;
        out.blockers.push('Release the actual purchase order before recording vendor acceptance');
        return out;
    }
    throw e;
} out.order = { id: o.id, project_id: o.project_id, package_id: o.package_id, version: o.version, number: o.number, date: o.details.date, supplier_id: o.source_snapshot.supplier.id, supplier_name: o.source_snapshot.supplier.name, description: o.source_snapshot.description, quantity: o.source_snapshot.quantity, uom: o.source_snapshot.uom, currency: o.source_snapshot.currency, total: o.source_snapshot.total, payment_terms: o.source_snapshot.payment_terms, delivery_days: o.source_snapshot.delivery_days, warranty: o.source_snapshot.warranty, released_at: o.released_at!, released_by_id: o.released_by_id }; return out; }
