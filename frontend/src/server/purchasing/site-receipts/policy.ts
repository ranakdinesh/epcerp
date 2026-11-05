import { PurchaseError, invalid, isoDate } from '../core/validation';
import { id } from '../sourcing/values';
import { text, sameSource } from '../orders/policy';
import { quantity } from '../inspection-reports/policy';
import { indiaDate } from '../../personal-tasks/validation';
import type { Actor } from '../core/access';
import type { Capabilities } from '../inspections/types';
import type { Record, Receipt, Source, Command } from './types';
export const forbidden = () => new PurchaseError(403, 'Current receipt permission and Step 17 scope are required');
export const missing = () => new PurchaseError(404, 'Receipt or current material dispatch not found');
export const stale = () => new PurchaseError(409, 'Receipt or dispatch evidence changed; retain your draft, reload and review');
export function guard(s: Source) { if (!s.available || s.blockers.length)
    invalid('source', 'Resolve current quality, IRN and project/package blockers first'); for (const v of [s.project_id, s.package_id, s.dispatch_id, s.clearance_id, s.quality_id, s.report_id, s.order_id, s.inspection_id])
    id(v); if (s.dispatch_version < 1 || s.dispatch_revision < 1 || !isoDate(s.dispatch_date) || s.dispatch_date < s.clearance_date || !s.lr_number || !s.transporter_name || s.clearance_version < 1 || s.clearance_revision < 1 || !s.mdcc_ref || !isoDate(s.clearance_date) || s.clearance_date < s.issue_date || s.quality_version < 1 || s.quality_revision < 1 || s.report_version < 1 || s.report_revision < 1 || s.inspection_version < 1 || s.order_version < 2 || !isoDate(s.issue_date) || !s.ir_number || !s.irn_number || !s.number || !s.supplier_name || !s.description || !s.uom)
    invalid('source', 'Use the actual current material dispatch and its clearance evidence'); quantity('source', s.cleared_quantity, true); }
export function accessGuard(a: Actor, c: Capabilities, write: boolean) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!c.read || write && (!c.write || !c.work))
    throw forbidden(); }
export function validate(raw: Command) { const q = { ...raw, reason: text('reason', raw.reason, 2000) }; if (q.version < 0)
    invalid('version', 'Use the current nonnegative receipt version'); if (q.action === 'record') {
    if (!q.receipt)
        invalid('receipt', 'Supply the receipt details');
}
else if (q.action === 'revise') {
    if (q.receipt)
        invalid('receipt', 'Revision carries only its version and reason');
}
else
    invalid('action', 'Choose record or revise'); return q; }
export function receipt(q: Receipt, s: Source, today: string) { const out = { ...q }; if (!['pending', 'in_transit', 'delivered_at_site'].includes(out.status))
    invalid('status', 'Choose pending, in transit or delivered at site'); out.packages_condition ||= 'not_checked'; if (!['not_checked', 'intact', 'minor_damage', 'quarantine'].includes(out.packages_condition))
    invalid('packages_condition', 'Choose the actual package condition'); if (out.packages_received < 0)
    invalid('packages_received', 'Package count cannot be negative'); for (const [field, n, required] of [['receiving_officer', 300, out.status === 'delivered_at_site'], ['transit_damage_notes', 4000, false], ['gate_entry_ref', 200, false]] as const)
    out[field] = text(field, out[field], n, required); if (out.arrival_date && (!isoDate(out.arrival_date) || out.arrival_date < s.dispatch_date || out.arrival_date > today) || out.status === 'delivered_at_site' && !out.arrival_date)
    invalid('arrival_date', 'Use the actual site arrival date on or after dispatch and not in the future'); return out; }
export function can(r: Record | null, a: Actor, c: Capabilities, action: string, s: Source) { accessGuard(a, c, true); guard(s); if (!['record', 'revise'].includes(action))
    invalid('action', 'Choose record or revise'); if (!r) {
    if (action !== 'record')
        invalid('receipt', 'Record the receipt before starting a revision');
    return;
} if (!r.is_active || r.project_id !== s.project_id || r.package_id !== s.package_id)
    throw missing(); if (action === 'record' && (!sameSource(r.source_snapshot, s) || r.receipt.status === 'delivered_at_site'))
    throw stale(); if (action === 'revise' && r.receipt.status !== 'delivered_at_site' && sameSource(r.source_snapshot, s))
    invalid('revision', 'Edit the current pending or in-transit receipt instead'); }
const blank = (): Receipt => ({ status: 'pending', arrival_date: '', receiving_officer: '', packages_received: 0, packages_condition: 'not_checked', transit_damage_notes: '', gate_entry_ref: '' });
export function apply(base: Record | null, a: Actor, c: Capabilities, raw: Command, s: Source, at: string, newID: string): Record { const q = validate(raw); can(base, a, c, q.action, s); if (q.version !== (base?.version ?? 0))
    throw stale(); const out: Record = base ? structuredClone(base) : { id: newID, project_id: s.project_id, package_id: s.package_id, version: 0, revision: 1, source_snapshot: structuredClone(s), receipt: blank(), is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id }; id(out.id); id(a.id); if (q.action === 'revise') {
    out.revision++;
    out.source_snapshot = structuredClone(s);
    out.receipt = blank();
}
else
    out.receipt = receipt(q.receipt!, s, indiaDate(new Date(at))); out.version = q.version + 1; out.updated_at = at; out.updated_by_id = a.id; return out; }
export function delivered(r: Record | null, s: Source) { guard(s); if (!r?.is_active)
    throw missing(); if (r.project_id !== s.project_id || r.package_id !== s.package_id || !sameSource(r.source_snapshot, s))
    throw stale(); if (r.version < 1 || r.revision < 1 || r.receipt.status !== 'delivered_at_site')
    invalid('receipt', 'Current physical site receipt is required'); receipt(r.receipt, s, indiaDate(new Date(r.updated_at))); return structuredClone(r); }
