import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { stageAccess } from '../core/stage-access';
import { clearedFact } from '../client-clearance/service';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_dispatch.read'], 'business');
export const access = (db: DB, a: Actor, p: string, id: string) => stageAccess(db, a, p, id, 'projects.purchase_dispatch', 16);
export const blankSource = (p: string, id: string): Source => ({ clearance_id: '', clearance_version: 0, clearance_revision: 0, mdcc_ref: '', clearance_date: '', project_id: p, package_id: id, quality_id: '', quality_version: 0, quality_revision: 0, report_id: '', report_version: 0, report_revision: 0, ir_number: '', irn_number: '', issue_date: '', order_id: '', order_version: 0, number: '', supplier_name: '', description: '', cleared_quantity: '', uom: '', inspection_id: '', inspection_version: 0, available: false, blockers: [] });
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { ...blankSource(p, id), available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const step of record.steps)
    if (step.number === 16 && step.state === 'blocked')
        out.blockers.push('Material dispatch has an unresolved blocker'); let actual; try {
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
    invalid('source', 'Report belongs to another project/package'); const { available, blockers, ...facts } = actual.source_snapshot; void available; void blockers; return { ...out, ...facts, clearance_id: actual.id, clearance_version: actual.version, clearance_revision: actual.revision, mdcc_ref: actual.clearance.mdcc_ref, clearance_date: actual.clearance.clearance_date }; }
