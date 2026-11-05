import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { project } from '../core/access';
import { projectAccess } from '../core/project-access';
import { stepAccess } from '../workflow/reader';
import { released } from '../orders/reader';
import { confirmedFact } from '../engineer-confirmations/service';
import { blankSource as upstreamBlank } from '../engineer-confirmations/context';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['accounts.purchase_acceptance.read'], true);
export async function access(db: DB, a: Actor, p: string, id: string) { const parent = await project(db, p, true), out = { read: false, write: false, work: false, scoped: false }; if (!a.allows('accounts.purchase_acceptance.read') || !a.allows('accounts.purchase_acceptance.view'))
    return out; const scope = async (code: string, write: boolean) => { try {
    const grant = await projectAccess(db, a, parent, code, write);
    return !write || grant.can_write;
}
catch (e) {
    if (e instanceof AppError && e.status === 403)
        return false;
    throw e;
} }; out.scoped = await scope('accounts.purchase_acceptance.read', false); out.read = out.scoped || (await stepAccess(db, a, p, id, 20)).read; if (a.allows('accounts.purchase_acceptance.write')) {
    out.work = await scope('accounts.purchase_acceptance.write', true);
    out.write = out.read && out.work;
} return out; }
export const blankSource = (p: string, id: string): Source => ({ ...upstreamBlank(p, id), engineer_id: '', engineer_version: 0, engineer_revision: 0, confirmation_date: '', engineer_name: '', designation: '', mbook_ref: '', work_package_ref: '', engineer_remarks: '', currency: '' });
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { ...blankSource(p, id), available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const step of record.steps)
    if (step.number === 20 && step.state === 'blocked')
        out.blockers.push('Accounts acceptance has an unresolved blocker'); let actual; try {
    actual = await confirmedFact(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 409, 422].includes(e.status)) {
        out.available = false;
        out.blockers.push(e.message);
        return out;
    }
    throw e;
} if (actual.project_id !== p || actual.package_id !== id)
    invalid('source', 'GRN belongs to another project/package'); const po = await released(db, p, id); if (po.id !== actual.source_snapshot.order_id || po.version !== actual.source_snapshot.order_version) {
    out.available = false;
    out.blockers.push('Current released PO is required');
    return out;
} const { available, blockers, ...facts } = actual.source_snapshot; void available; void blockers; return { ...out, ...facts, engineer_id: actual.id, engineer_version: actual.version, engineer_revision: actual.revision, confirmation_date: actual.confirmation.confirmation_date, engineer_name: actual.confirmation.engineer_name, designation: actual.confirmation.designation, mbook_ref: actual.confirmation.mbook_ref, work_package_ref: actual.confirmation.work_package_ref, engineer_remarks: actual.confirmation.remarks, currency: po.source_snapshot.currency }; }
