import { PurchaseError, invalid, isoDate } from '../core/validation';
import { id } from '../sourcing/values';
import { text, sameSource } from '../orders/policy';
import { quantity as reportQuantity } from '../inspection-reports/policy';
import { indiaDate } from '../../personal-tasks/validation';
import type { Actor } from '../core/access';
import type { Capabilities } from '../inspections/types';
import type { Record, GRN, Source, Command } from './types';
export const forbidden = () => new PurchaseError(403, 'Current grn permission and Step 18 scope are required');
export const missing = () => new PurchaseError(404, 'GRN or current physical site receipt not found');
export const stale = () => new PurchaseError(409, 'GRN or site receipt evidence changed; retain your draft, reload and review');
export function quantity(field: string, raw: string, positive: boolean) { try {
    return reportQuantity(field, raw, positive);
}
catch (e) {
    if (e instanceof PurchaseError && e.message === 'Cleared quantity must be positive')
        invalid(field, 'Quantity must be positive');
    throw e;
} }
export function guard(s: Source) { if (!s.available || s.blockers.length)
    invalid('source', 'Resolve current quality, IRN and project/package blockers first'); for (const v of [s.project_id, s.package_id, s.receipt_id, s.dispatch_id, s.clearance_id, s.quality_id, s.report_id, s.order_id, s.inspection_id])
    id(v); if (s.receipt_version < 1 || s.receipt_revision < 1 || !isoDate(s.arrival_date) || s.arrival_date < s.dispatch_date || !s.receiving_officer || s.packages_received < 0 || s.dispatch_version < 1 || s.dispatch_revision < 1 || !isoDate(s.dispatch_date) || s.dispatch_date < s.clearance_date || !s.lr_number || !s.transporter_name || s.clearance_version < 1 || s.clearance_revision < 1 || !s.mdcc_ref || !isoDate(s.clearance_date) || s.clearance_date < s.issue_date || s.quality_version < 1 || s.quality_revision < 1 || s.report_version < 1 || s.report_revision < 1 || s.inspection_version < 1 || s.order_version < 2 || !isoDate(s.issue_date) || !s.ir_number || !s.irn_number || !s.number || !s.supplier_name || !s.description || !s.uom)
    invalid('source', 'Use the actual current physical site receipt and its clearance evidence'); quantity('source', s.cleared_quantity, true); }
export function accessGuard(a: Actor, c: Capabilities, write: boolean) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!c.read || write && (!c.write || !c.work))
    throw forbidden(); }
export function validate(raw: Command) { const q = { ...raw, reason: text('reason', raw.reason, 2000) }; if (q.version < 0)
    invalid('version', 'Use the current nonnegative grn version'); if (q.action === 'record') {
    if (!q.grn)
        invalid('grn', 'Supply the grn details');
}
else if (q.action === 'revise') {
    if (q.grn)
        invalid('grn', 'Revision carries only its version and reason');
}
else
    invalid('action', 'Choose record or revise'); return q; }
export function grn(q: Omit<GRN, 'uom'>, s: Source, today: string): GRN { const out = { ...q, uom: s.uom }; if (!['pending', 'issued'].includes(out.status))
    invalid('status', 'Choose pending or issued'); for (const [field, n, required] of [['grn_number', 200, out.status === 'issued'], ['store_keeper_name', 300, false], ['remarks', 4000, false], ['warranty_ref', 200, false], ['file_name', 500, false]] as const)
    out[field] = text(field, out[field], n, required); if (out.grn_date && (!isoDate(out.grn_date) || out.grn_date < s.arrival_date || out.grn_date > today) || out.status === 'issued' && !out.grn_date)
    invalid('grn_date', 'Use the actual GRN date on or after site arrival and not in the future'); if (out.warranty_valid_until && !isoDate(out.warranty_valid_until))
    invalid('warranty_valid_until', 'Use a real warranty validity date'); const received = quantity('received_qty', out.received_qty, out.status === 'issued'), accepted = quantity('accepted_qty', out.accepted_qty, false), rejected = quantity('rejected_qty', out.rejected_qty, false); out.received_qty = received.text; out.accepted_qty = accepted.text; out.rejected_qty = rejected.text; if (out.status === 'issued') {
    if (received.value.compare(quantity('source', s.cleared_quantity, true).value) > 0)
        invalid('received_qty', 'Received quantity cannot exceed the current cleared allowance');
    if (accepted.value.add(rejected.value).compare(received.value) > 0)
        invalid('accepted_qty', 'Accepted and rejected quantities together cannot exceed received quantity');
} return out; }
export function can(r: Record | null, a: Actor, c: Capabilities, action: string, s: Source) { accessGuard(a, c, true); guard(s); if (!['record', 'revise'].includes(action))
    invalid('action', 'Choose record or revise'); if (!r) {
    if (action !== 'record')
        invalid('grn', 'Record the grn before starting a revision');
    return;
} if (!r.is_active || r.project_id !== s.project_id || r.package_id !== s.package_id)
    throw missing(); if (action === 'record' && (!sameSource(r.source_snapshot, s) || r.grn.status === 'issued'))
    throw stale(); if (action === 'revise' && r.grn.status !== 'issued' && sameSource(r.source_snapshot, s))
    invalid('revision', 'Edit the current pending GRN instead'); }
const blank = (uom: string): GRN => ({ status: 'pending', grn_number: '', grn_date: '', received_qty: '0', accepted_qty: '0', rejected_qty: '0', store_keeper_name: '', remarks: '', warranty_ref: '', warranty_valid_until: '', file_name: '', uom });
export function apply(base: Record | null, a: Actor, c: Capabilities, raw: Command, s: Source, at: string, newID: string): Record { const q = validate(raw); can(base, a, c, q.action, s); if (q.version !== (base?.version ?? 0))
    throw stale(); const out: Record = base ? structuredClone(base) : { id: newID, project_id: s.project_id, package_id: s.package_id, version: 0, revision: 1, source_snapshot: structuredClone(s), grn: blank(s.uom), is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id }; id(out.id); id(a.id); if (q.action === 'revise') {
    out.revision++;
    out.source_snapshot = structuredClone(s);
    out.grn = blank(s.uom);
}
else
    out.grn = grn(q.grn!, s, indiaDate(new Date(at))); out.version = q.version + 1; out.updated_at = at; out.updated_by_id = a.id; return out; }
export function issued(r: Record | null, s: Source) { guard(s); if (!r?.is_active)
    throw missing(); if (r.project_id !== s.project_id || r.package_id !== s.package_id || !sameSource(r.source_snapshot, s))
    throw stale(); if (r.version < 1 || r.revision < 1 || r.grn.status !== 'issued' || r.grn.uom !== s.uom)
    invalid('grn', 'Current issued GRN is required'); grn(r.grn, s, indiaDate(new Date(r.updated_at))); return structuredClone(r); }
