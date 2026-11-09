import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { stageAccess } from '../core/stage-access';
import { deliveredFact } from '../site-receipts/service';
import { blankSource as upstreamBlank } from '../site-receipts/context';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_grns.read'], 'business');
export const access = (db: DB, a: Actor, p: string, id: string) => stageAccess(db, a, p, id, 'projects.purchase_grns', 18);
export const blankSource = (p: string, id: string): Source => ({ ...upstreamBlank(p, id), receipt_id: '', receipt_version: 0, receipt_revision: 0, arrival_date: '', receiving_officer: '', packages_received: 0, packages_condition: '', transit_damage_notes: '', gate_entry_ref: '' });
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { ...blankSource(p, id), available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const step of record.steps)
    if (step.number === 18 && step.state === 'blocked')
        out.blockers.push('GRN inwarding has an unresolved blocker'); let actual; try {
    actual = await deliveredFact(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 409, 422].includes(e.status)) {
        out.available = false;
        out.blockers.push(e.message);
        return out;
    }
    throw e;
} if (actual.project_id !== p || actual.package_id !== id)
    invalid('source', 'Receipt belongs to another project/package'); const { available, blockers, ...facts } = actual.source_snapshot; void available; void blockers; return { ...out, ...facts, receipt_id: actual.id, receipt_version: actual.version, receipt_revision: actual.revision, arrival_date: actual.receipt.arrival_date, receiving_officer: actual.receipt.receiving_officer, packages_received: actual.receipt.packages_received, packages_condition: actual.receipt.packages_condition, transit_damage_notes: actual.receipt.transit_damage_notes, gate_entry_ref: actual.receipt.gate_entry_ref }; }
