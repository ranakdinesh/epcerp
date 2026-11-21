import { PurchaseError, invalid, isoDate } from '../core/validation';
import { id } from '../sourcing/values';
import { text, sameSource } from '../orders/policy';
import { quantity } from '../inspection-reports/policy';
import { indiaDate } from '../../personal-tasks/validation';
import type { Actor } from '../core/access';
import type { Capabilities } from '../inspections/types';
import type { Record, Dispatch, Source, Command } from './types';
export const forbidden = () => new PurchaseError(403, 'Current dispatch permission and Step 16 scope are required');
export const missing = () => new PurchaseError(404, 'Dispatch or current client MDCC not found');
export const stale = () => new PurchaseError(409, 'Dispatch or client clearance evidence changed; retain your draft, reload and review');
export function guard(s: Source) { if (!s.available || s.blockers.length)
    invalid('source', 'Resolve current quality, IRN and project/package blockers first'); for (const v of [s.project_id, s.package_id, s.clearance_id, s.quality_id, s.report_id, s.order_id, s.inspection_id])
    id(v); if (s.clearance_version < 1 || s.clearance_revision < 1 || !s.mdcc_ref || !isoDate(s.clearance_date) || s.clearance_date < s.issue_date || s.quality_version < 1 || s.quality_revision < 1 || s.report_version < 1 || s.report_revision < 1 || s.inspection_version < 1 || s.order_version < 2 || !isoDate(s.issue_date) || !s.ir_number || !s.irn_number || !s.number || !s.supplier_name || !s.description || !s.uom)
    invalid('source', 'Use the actual current verified quality and issued IRN'); quantity('source', s.cleared_quantity, true); }
export function accessGuard(a: Actor, c: Capabilities, write: boolean) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!c.read || write && (!c.write || !c.work))
    throw forbidden(); }
export function validate(raw: Command) { const q = { ...raw, reason: text('reason', raw.reason, 2000) }; if (q.version < 0)
    invalid('version', 'Use the current nonnegative dispatch version'); if (q.action === 'record') {
    if (!q.dispatch)
        invalid('dispatch', 'Supply the dispatch details');
}
else if (q.action === 'revise') {
    if (q.dispatch)
        invalid('dispatch', 'Revision carries only its version and reason');
}
else
    invalid('action', 'Choose record or revise'); return q; }
export function dispatch(q: Dispatch, s: Source, today: string) { const out = { ...q }; if (!['pending', 'dispatched'].includes(q.status))
    invalid('status', 'Choose pending or dispatched'); for (const [field, n, required] of [['transporter_name', 300, q.status === 'dispatched'], ['lr_number', 200, q.status === 'dispatched'], ['vehicle_number', 100, false], ['eway_bill_number', 200, false], ['invoice_number', 200, false], ['driver_contact', 100, false], ['file_name', 500, false]] as const)
    out[field] = text(field, out[field], n, required); if (out.dispatch_date && (!isoDate(out.dispatch_date) || out.dispatch_date < s.clearance_date || out.dispatch_date > today) || q.status === 'dispatched' && !out.dispatch_date)
    invalid('dispatch_date', 'Use the actual dispatch date on or after client clearance and not in the future'); for (const field of ['lr_date', 'invoice_date'] as const)
    if (out[field] && (!isoDate(out[field]) || out[field] > today))
        invalid(field, 'Use a real nonfuture document date'); if (out.eway_bill_valid_until && !isoDate(out.eway_bill_valid_until))
    invalid('eway_bill_valid_until', 'Use a real e-way bill validity date'); return out; }
export function can(r: Record | null, a: Actor, c: Capabilities, action: string, s: Source) { accessGuard(a, c, true); guard(s); if (!['record', 'revise'].includes(action))
    invalid('action', 'Choose record or revise'); if (!r) {
    if (action !== 'record')
        invalid('dispatch', 'Record the dispatch before starting a revision');
    return;
} if (!r.is_active || r.project_id !== s.project_id || r.package_id !== s.package_id)
    throw missing(); if (action === 'record' && (!sameSource(r.source_snapshot, s) || r.dispatch.status === 'dispatched'))
    throw stale(); if (action === 'revise' && r.dispatch.status !== 'dispatched' && sameSource(r.source_snapshot, s))
    invalid('revision', 'Edit the current pending dispatch instead'); }
const blank = (): Dispatch => ({ status: 'pending', dispatch_date: '', transporter_name: '', lr_number: '', lr_date: '', vehicle_number: '', eway_bill_number: '', eway_bill_valid_until: '', invoice_number: '', invoice_date: '', driver_contact: '', file_name: '' });
export function apply(base: Record | null, a: Actor, c: Capabilities, raw: Command, s: Source, at: string, newID: string): Record { const q = validate(raw); can(base, a, c, q.action, s); if (q.version !== (base?.version ?? 0))
    throw stale(); const out: Record = base ? structuredClone(base) : { id: newID, project_id: s.project_id, package_id: s.package_id, version: 0, revision: 1, source_snapshot: structuredClone(s), dispatch: blank(), is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id }; id(out.id); id(a.id); if (q.action === 'revise') {
    out.revision++;
    out.source_snapshot = structuredClone(s);
    out.dispatch = blank();
}
else
    out.dispatch = dispatch(q.dispatch!, s, indiaDate(new Date(at))); out.version = q.version + 1; out.updated_at = at; out.updated_by_id = a.id; return out; }
export function dispatched(r: Record | null, s: Source) { guard(s); if (!r?.is_active)
    throw missing(); if (r.project_id !== s.project_id || r.package_id !== s.package_id || !sameSource(r.source_snapshot, s))
    throw stale(); if (r.version < 1 || r.revision < 1 || r.dispatch.status !== 'dispatched')
    invalid('dispatch', 'Current issued Material dispatch is required'); dispatch(r.dispatch, s, indiaDate(new Date(r.updated_at))); return structuredClone(r); }
