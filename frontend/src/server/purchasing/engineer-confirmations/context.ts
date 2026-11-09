import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { stageAccess } from '../core/stage-access';
import { issuedFact } from '../grns/service';
import { blankSource as upstreamBlank } from '../grns/context';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_engineer_confirmations.read'], 'business');
export const access = (db: DB, a: Actor, p: string, id: string) => stageAccess(db, a, p, id, 'projects.purchase_engineer_confirmations', 19);
export const blankSource = (p: string, id: string): Source => ({ ...upstreamBlank(p, id), grn_id: '', grn_version: 0, grn_revision: 0, grn_number: '', grn_date: '', received_qty: '', accepted_qty: '', rejected_qty: '', warranty_ref: '', warranty_valid_until: '' });
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { ...blankSource(p, id), available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const step of record.steps)
    if (step.number === 19 && step.state === 'blocked')
        out.blockers.push('Engineer confirmation has an unresolved blocker'); let actual; try {
    actual = await issuedFact(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 409, 422].includes(e.status)) {
        out.available = false;
        out.blockers.push(e.message);
        return out;
    }
    throw e;
} if (actual.project_id !== p || actual.package_id !== id)
    invalid('source', 'GRN belongs to another project/package'); const { available, blockers, ...facts } = actual.source_snapshot; void available; void blockers; return { ...out, ...facts, grn_id: actual.id, grn_version: actual.version, grn_revision: actual.revision, grn_number: actual.grn.grn_number, grn_date: actual.grn.grn_date, received_qty: actual.grn.received_qty, accepted_qty: actual.grn.accepted_qty, rejected_qty: actual.grn.rejected_qty, warranty_ref: actual.grn.warranty_ref, warranty_valid_until: actual.grn.warranty_valid_until }; }
