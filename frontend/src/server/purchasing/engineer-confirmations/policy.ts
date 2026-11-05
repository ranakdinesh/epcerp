import { PurchaseError, invalid, isoDate } from '../core/validation';
import { id } from '../sourcing/values';
import { text, sameSource } from '../orders/policy';
import { quantity, guard as grnGuard } from '../grns/policy';
import { indiaDate } from '../../personal-tasks/validation';
import type { Actor } from '../core/access';
import type { Capabilities } from '../inspections/types';
import type { Record, Confirmation, Source, Command } from './types';
export const forbidden = () => new PurchaseError(403, 'Current confirmation permission and Step 19 scope are required');
export const missing = () => new PurchaseError(404, 'Engineer confirmation or current issued GRN not found');
export const stale = () => new PurchaseError(409, 'Confirmation or GRN evidence changed; retain your draft, reload and review');
export function guard(s: Source) { grnGuard(s); id(s.grn_id); if (s.grn_version < 1 || s.grn_revision < 1 || !s.grn_number || !isoDate(s.grn_date) || s.grn_date < s.arrival_date)
    invalid('source', 'Use the actual current issued GRN and its clearance evidence'); const allowance = quantity('source', s.cleared_quantity, true), received = quantity('source', s.received_qty, true), accepted = quantity('source', s.accepted_qty, false), rejected = quantity('source', s.rejected_qty, false); if (received.value.compare(allowance.value) > 0 || accepted.value.add(rejected.value).compare(received.value) > 0)
    invalid('source', 'Use the actual current issued GRN breakdown'); }
export function accessGuard(a: Actor, c: Capabilities, write: boolean) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!c.read || write && (!c.write || !c.work))
    throw forbidden(); }
export function validate(raw: Command) { const q = { ...raw, reason: text('reason', raw.reason, 2000) }; if (q.version < 0)
    invalid('version', 'Use the current nonnegative confirmation version'); if (q.action === 'record') {
    if (!q.confirmation)
        invalid('confirmation', 'Supply the confirmation details');
}
else if (q.action === 'revise') {
    if (q.confirmation)
        invalid('confirmation', 'Revision carries only its version and reason');
}
else
    invalid('action', 'Choose record or revise'); return q; }
export function confirmation(q: Confirmation, s: Source, today: string) { const out = { ...q }; if (!['pending', 'confirmed'].includes(out.status))
    invalid('status', 'Choose pending or confirmed'); for (const [field, n, required] of [['engineer_name', 300, out.status === 'confirmed'], ['designation', 200, false], ['mbook_ref', 200, false], ['work_package_ref', 200, false], ['remarks', 4000, false]] as const)
    out[field] = text(field, out[field], n, required); if (out.confirmation_date && (!isoDate(out.confirmation_date) || out.confirmation_date < s.grn_date || out.confirmation_date > today) || out.status === 'confirmed' && !out.confirmation_date)
    invalid('confirmation_date', 'Use the actual confirmation date on or after GRN issuance and not in the future'); return out; }
export function can(r: Record | null, a: Actor, c: Capabilities, action: string, s: Source) { accessGuard(a, c, true); guard(s); if (!['record', 'revise'].includes(action))
    invalid('action', 'Choose record or revise'); if (!r) {
    if (action !== 'record')
        invalid('confirmation', 'Record the confirmation before starting a revision');
    return;
} if (!r.is_active || r.project_id !== s.project_id || r.package_id !== s.package_id)
    throw missing(); if (action === 'record' && (!sameSource(r.source_snapshot, s) || r.confirmation.status === 'confirmed'))
    throw stale(); if (action === 'revise' && r.confirmation.status !== 'confirmed' && sameSource(r.source_snapshot, s))
    invalid('revision', 'Edit the current pending Confirmation instead'); }
const blank = (): Confirmation => ({ status: 'pending', confirmation_date: '', engineer_name: '', designation: '', mbook_ref: '', work_package_ref: '', remarks: '' });
export function apply(base: Record | null, a: Actor, c: Capabilities, raw: Command, s: Source, at: string, newID: string): Record { const q = validate(raw); can(base, a, c, q.action, s); if (q.version !== (base?.version ?? 0))
    throw stale(); const out: Record = base ? structuredClone(base) : { id: newID, project_id: s.project_id, package_id: s.package_id, version: 0, revision: 1, source_snapshot: structuredClone(s), confirmation: blank(), is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id }; id(out.id); id(a.id); if (q.action === 'revise') {
    out.revision++;
    out.source_snapshot = structuredClone(s);
    out.confirmation = blank();
}
else
    out.confirmation = confirmation(q.confirmation!, s, indiaDate(new Date(at))); out.version = q.version + 1; out.updated_at = at; out.updated_by_id = a.id; return out; }
export function confirmed(r: Record | null, s: Source) { guard(s); if (!r?.is_active)
    throw missing(); if (r.project_id !== s.project_id || r.package_id !== s.package_id || !sameSource(r.source_snapshot, s))
    throw stale(); if (r.version < 1 || r.revision < 1 || r.confirmation.status !== 'confirmed')
    invalid('confirmation', 'Current issued Confirmation is required'); confirmation(r.confirmation, s, indiaDate(new Date(r.updated_at))); return structuredClone(r); }
