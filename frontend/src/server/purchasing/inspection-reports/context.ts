import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { stageAccess } from '../core/stage-access';
import { clearedFact } from '../inspections/service';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_inspection_reports.read'], 'business');
export const access = (db: DB, a: Actor, p: string, id: string) => stageAccess(db, a, p, id, 'projects.purchase_inspection_reports', 13);
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { project_id: p, package_id: id, inspection_id: '', inspection_version: 0, inspection_revision: 0, actual_date: '', order_id: '', order_version: 0, number: '', supplier_name: '', description: '', quantity: '', uom: '', qap_reference: '', available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const step of record.steps)
    if (step.number === 13 && step.state === 'blocked')
        out.blockers.push('IR / IRN has an unresolved blocker'); let actual; try {
    actual = await clearedFact(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 409, 422].includes(e.status)) {
        out.available = false;
        out.blockers.push(e.message);
        return out;
    }
    throw e;
} if (actual.project_id !== p || actual.package_id !== id)
    invalid('source', 'Inspection belongs to another project/package'); const s = actual.source_snapshot; return { ...out, inspection_id: actual.id, inspection_version: actual.version, inspection_revision: actual.revision, actual_date: actual.inspection.actual_date, order_id: s.order_id, order_version: s.order_version, number: s.number, supplier_name: s.supplier_name, description: s.description, quantity: s.quantity, uom: s.uom, qap_reference: s.qap_reference }; }
