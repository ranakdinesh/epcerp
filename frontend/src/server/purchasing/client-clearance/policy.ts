import { PurchaseError, invalid, isoDate } from '../core/validation';
import { id } from '../sourcing/values';
import { text, sameSource } from '../orders/policy';
import { quantity } from '../inspection-reports/policy';
import { indiaDate } from '../../personal-tasks/validation';
import type { Actor } from '../core/access';
import type { Capabilities } from '../inspections/types';
import type { Record, Clearance, Source, Command } from './types';
export const forbidden = () => new PurchaseError(403, 'Current clearance permission and Step 15 scope are required');
export const missing = () => new PurchaseError(404, 'Clearance or current cleared inspection not found');
export const stale = () => new PurchaseError(409, 'Clearance or inspection evidence changed; retain your draft, reload and review');
export function guard(s: Source) { if (!s.available || s.blockers.length)
    invalid('source', 'Resolve current quality, IRN and project/package blockers first'); for (const v of [s.project_id, s.package_id, s.quality_id, s.report_id, s.order_id, s.inspection_id])
    id(v); if (s.quality_version < 1 || s.quality_revision < 1 || s.report_version < 1 || s.report_revision < 1 || s.inspection_version < 1 || s.order_version < 2 || !isoDate(s.issue_date) || !s.ir_number || !s.irn_number || !s.number || !s.supplier_name || !s.description || !s.uom)
    invalid('source', 'Use the actual current verified quality and issued IRN'); quantity('source', s.cleared_quantity, true); }
export function accessGuard(a: Actor, c: Capabilities, write: boolean) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!c.read || write && (!c.write || !c.work))
    throw forbidden(); }
export function validate(raw: Command) { const q = { ...raw, reason: text('reason', raw.reason, 2000) }; if (q.version < 0)
    invalid('version', 'Use the current nonnegative clearance version'); if (q.action === 'record') {
    if (!q.clearance)
        invalid('clearance', 'Supply the clearance details');
}
else if (q.action === 'revise') {
    if (q.clearance)
        invalid('clearance', 'Revision carries only its version and reason');
}
else
    invalid('action', 'Choose record or revise'); return q; }
export function clearance(q: Clearance, s: Source, today: string) { const out = { ...q }; if (!['pending', 'cleared'].includes(q.status))
    invalid('status', 'Choose pending or cleared'); for (const [field, n, required] of [['mdcc_ref', 200, q.status === 'cleared'], ['cleared_by_authority', 300, false], ['remarks', 4000, false], ['file_name', 500, false]] as const)
    out[field] = text(field, out[field], n, required); if (out.clearance_date && (!isoDate(out.clearance_date) || out.clearance_date < s.issue_date || out.clearance_date > today) || out.status === 'cleared' && !out.clearance_date)
    invalid('clearance_date', 'Use the actual clearance date on or after issued IRN and not in the future'); return out; }
export function can(r: Record | null, a: Actor, c: Capabilities, action: string, s: Source) { accessGuard(a, c, true); guard(s); if (!['record', 'revise'].includes(action))
    invalid('action', 'Choose record or revise'); if (!r) {
    if (action !== 'record')
        invalid('clearance', 'Record the clearance before starting a revision');
    return;
} if (!r.is_active || r.project_id !== s.project_id || r.package_id !== s.package_id)
    throw missing(); if (action === 'record' && (!sameSource(r.source_snapshot, s) || r.clearance.status === 'cleared'))
    throw stale(); if (action === 'revise' && r.clearance.status !== 'cleared' && sameSource(r.source_snapshot, s))
    invalid('revision', 'Edit the current pending clearance instead'); }
const blank = (): Clearance => ({ status: 'pending', mdcc_ref: '', clearance_date: '', cleared_by_authority: '', remarks: '', file_name: '' });
export function apply(base: Record | null, a: Actor, c: Capabilities, raw: Command, s: Source, at: string, newID: string): Record { const q = validate(raw); can(base, a, c, q.action, s); if (q.version !== (base?.version ?? 0))
    throw stale(); const out: Record = base ? structuredClone(base) : { id: newID, project_id: s.project_id, package_id: s.package_id, version: 0, revision: 1, source_snapshot: structuredClone(s), clearance: blank(), is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id }; id(out.id); id(a.id); if (q.action === 'revise') {
    out.revision++;
    out.source_snapshot = structuredClone(s);
    out.clearance = blank();
}
else
    out.clearance = clearance(q.clearance!, s, indiaDate(new Date(at))); out.version = q.version + 1; out.updated_at = at; out.updated_by_id = a.id; return out; }
export function cleared(r: Record | null, s: Source) { guard(s); if (!r?.is_active)
    throw missing(); if (r.project_id !== s.project_id || r.package_id !== s.package_id || !sameSource(r.source_snapshot, s))
    throw stale(); if (r.version < 1 || r.revision < 1 || r.clearance.status !== 'cleared')
    invalid('clearance', 'Current issued Client dispatch clearance is required'); clearance(r.clearance, s, indiaDate(new Date(r.updated_at))); return structuredClone(r); }
