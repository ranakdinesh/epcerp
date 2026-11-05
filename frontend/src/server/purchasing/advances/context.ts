import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import { project, type Actor } from '../core/access';
import { projectAccess } from '../core/project-access';
import { inspect, stepAccess } from '../workflow/reader';
import { scoped } from '../sourcing/context';
import { released } from '../orders/reader';
import { acceptedFact } from '../acceptance/service';
import { VerificationRepository } from '../../master/verification/repository';
import { evaluate } from '../../master/verification/domain';
import type { Source, Capabilities } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_advances.read', 'accounts.purchase_advances.view'], true);
export async function access(db: DB, a: Actor, p: string, id: string): Promise<Capabilities> { const parent = await project(db, p, true), out: Capabilities = { read: false, write: false, work: false, review: false, disburse: false, scoped: false }; if (a.allows('projects.purchase_advances.read')) {
    out.scoped = await scoped(db, a, p, 'projects.purchase_advances.read', false);
    const task = await stepAccess(db, a, p, id, 11);
    out.work = task.work;
    out.read = out.scoped || task.read;
    out.write = out.read && a.allows('projects.purchase_advances.write');
} if (a.allows('accounts.purchase_advances.view') && a.allows('accounts.purchase_advances.read')) {
    try {
        await projectAccess(db, a, parent, 'accounts.purchase_advances.read', false);
    }
    catch (e) {
        if (e instanceof AppError && e.status === 403)
            return out;
        throw e;
    }
    out.read = true;
    out.scoped = true;
    for (const code of ['accounts.purchase_advances.write', 'accounts.purchase_advances.disburse'])
        if (a.allows(code)) {
            try {
                const grant = await projectAccess(db, a, parent, code, true);
                if (grant.can_write) {
                    if (code.endsWith('.write'))
                        out.review = true;
                    else
                        out.disburse = true;
                }
            }
            catch (e) {
                if (!(e instanceof AppError) || e.status !== 403)
                    throw e;
            }
        }
} return out; }
const blank = (p: string, id: string): Source => ({ project_id: p, package_id: id, order_id: '', order_version: 0, number: '', date: '', supplier_id: '', supplier_name: '', currency: '', total: '', payment_terms: '', acceptance_id: '', acceptance_version: 0, supplier_verification: '', calculation_basis: 'po_total', available: false, blockers: [] });
const block = (s: Source, message: string) => { s.available = false; s.blockers.push(message); return s; };
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { ...blank(p, id), available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const s of record.steps)
    if (s.number === 11 && s.state === 'blocked')
        out.blockers.push('Advance request has an unresolved blocker'); let po; try {
    po = await released(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 422].includes(e.status))
        return block(out, 'Release the actual purchase order first');
    throw e;
} Object.assign(out, { order_id: po.id, order_version: po.version, number: po.number, date: po.details.date, supplier_id: po.source_snapshot.supplier.id, supplier_name: po.source_snapshot.supplier.name, currency: po.source_snapshot.currency, total: po.source_snapshot.total, payment_terms: po.source_snapshot.payment_terms }); let accepted; try {
    accepted = await acceptedFact(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 422, 409].includes(e.status))
        return block(out, 'Current unconditional vendor acceptance is required');
    throw e;
} if (accepted.order_snapshot.id !== po.id || accepted.order_snapshot.version !== po.version)
    return block(out, 'Current unconditional vendor acceptance is required'); out.acceptance_id = accepted.id; out.acceptance_version = accepted.version; const repo = new VerificationRepository(db), verified = evaluate(await repo.snapshot(out.supplier_id), await repo.review(out.supplier_id)); if (!verified.can_generate_purchase_order)
    return block(out, verified.message); out.supplier_verification = verified.fingerprint; return out; }
export async function financeRead(db: DB, a: Actor, p: string) { const parent = await project(db, p, true); for (const code of ['accounts.purchase_advances.view', 'accounts.purchase_advances.read']) {
    if (!a.allows(code))
        return false;
    await projectAccess(db, a, parent, code, false);
} return true; }
