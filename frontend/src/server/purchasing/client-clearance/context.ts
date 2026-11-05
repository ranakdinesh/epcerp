import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { stageAccess } from '../core/stage-access';
import { verifiedFact } from '../quality/service';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_client_clearance.read'], true);
export const access = (db: DB, a: Actor, p: string, id: string) => stageAccess(db, a, p, id, 'projects.purchase_client_clearance', 15);
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { project_id: p, package_id: id, quality_id: '', quality_version: 0, quality_revision: 0, report_id: '', report_version: 0, report_revision: 0, ir_number: '', irn_number: '', issue_date: '', order_id: '', order_version: 0, number: '', supplier_name: '', description: '', cleared_quantity: '', uom: '', inspection_id: '', inspection_version: 0, available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const step of record.steps)
    if (step.number === 15 && step.state === 'blocked')
        out.blockers.push('Client dispatch clearance has an unresolved blocker'); let actual; try {
    actual = await verifiedFact(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 409, 422].includes(e.status)) {
        out.available = false;
        out.blockers.push(e.message);
        return out;
    }
    throw e;
} if (actual.project_id !== p || actual.package_id !== id)
    invalid('source', 'Report belongs to another project/package'); const s = actual.source_snapshot; return { ...out, quality_id: actual.id, quality_version: actual.version, quality_revision: actual.revision, report_id: s.report_id, report_version: s.report_version, report_revision: s.report_revision, ir_number: s.ir_number, irn_number: s.irn_number, issue_date: s.issue_date, order_id: s.order_id, order_version: s.order_version, number: s.number, supplier_name: s.supplier_name, description: s.description, cleared_quantity: s.cleared_quantity, uom: s.uom, inspection_id: s.inspection_id, inspection_version: s.inspection_version }; }
