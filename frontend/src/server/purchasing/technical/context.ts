import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import { project, type Actor } from '../core/access';
import { inspect, stepAccess } from '../workflow/reader';
import { scoped } from '../sourcing/context';
import { released } from '../sourcing/reader';
import { projectReady } from '../approvals/service';
import { indiaDate } from '../../personal-tasks/validation';
import type { Source, Capabilities } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_technical.read'], 'business');
export async function access(db: DB, a: Actor, p: string, id: string): Promise<Capabilities> { await project(db, p, true); const caps: Capabilities = { read: false, write: false, approve: false, scoped: false, work: {} }; if (!a.allows('projects.purchase_technical.read'))
    return caps; caps.scoped = await scoped(db, a, p, 'projects.purchase_technical.read', false); caps.read = caps.scoped; for (const n of [6, 7, 8]) {
    const value = await stepAccess(db, a, p, id, n);
    caps.work[n] = value.work;
    caps.read ||= value.read;
} caps.write = caps.read && a.allows('projects.purchase_technical.write'); if (caps.scoped && a.allows('projects.purchase_technical.approve'))
    try {
        caps.approve = await scoped(db, a, p, 'projects.purchase_technical.approve', true);
    }
    catch (e) {
        if (!(e instanceof AppError) || e.status !== 422)
            throw e;
    } return caps; }
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { case_id: '', version: 0, project_id: p, package_id: id, loi_reference: '', supplier_id: '', description: '', quantity: '', currency: '', total: '', financial_reference: '', released_date: '', project_available: false, blockers: [] }; if (!await projectReady(db, p)) {
    out.blockers = ['Finance must clear every current purchase group before technical documents'];
    return out;
} const loi = await released(db, p, id); out.project_available = current.project_available && current.eligible; out.blockers = [...current.blockers]; if (!loi) {
    out.project_available = false;
    out.blockers.push('Release an independently approved LOI before creating the technical kit');
    return out;
} Object.assign(out, { case_id: loi.case_id, version: loi.version, loi_reference: loi.loi_reference, supplier_id: loi.offer.supplier_id, ...(loi.offer.supplier_snapshot.name ? { supplier_name: loi.offer.supplier_snapshot.name } : {}), description: loi.source.description, quantity: loi.source.quantity, currency: loi.source.currency, total: loi.total, financial_reference: loi.financial_reference, released_date: indiaDate(new Date(loi.released_at)) }); if (!current.project_available)
    out.blockers.push('Current project controls prevent technical execution'); for (const s of record.steps)
    if (s.number >= 6 && s.number <= 8 && s.state === 'blocked')
        out.blockers.push('Technical preparation has an unresolved blocker'); return out; }
