import { PurchaseError, invalid, isoDate } from '../core/validation';
import { id } from '../sourcing/values';
import { text, sameSource } from '../orders/policy';
import { guard as engineerGuard } from '../engineer-confirmations/policy';
import { trim } from '../core/validation';
import { indiaDate } from '../../personal-tasks/validation';
import type { Actor } from '../core/access';
import type { Capabilities } from '../inspections/types';
import type { Record, Acceptance, Source, Command } from './types';
export const forbidden = () => new PurchaseError(403, 'Current acceptance permission and Step 20 scope are required');
export const missing = () => new PurchaseError(404, 'Accounts acceptance or current issued GRN not found');
export const stale = () => new PurchaseError(409, 'Acceptance or GRN evidence changed; retain your draft, reload and review');
export function guard(s: Source) { engineerGuard(s); id(s.engineer_id); if (s.engineer_version < 1 || s.engineer_revision < 1 || !isoDate(s.confirmation_date) || s.confirmation_date < s.grn_date || !s.engineer_name || !/^[A-Z]{3}$/.test(s.currency))
    invalid('source', 'Use the actual current issued GRN and its clearance evidence'); }
export function accessGuard(a: Actor, c: Capabilities, write: boolean) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!c.read || write && (!c.write || !c.work))
    throw forbidden(); }
export function validate(raw: Command) { const q = { ...raw, reason: text('reason', raw.reason, 2000) }; if (q.version < 0)
    invalid('version', 'Use the current nonnegative acceptance version'); if (q.action === 'record') {
    if (!q.acceptance)
        invalid('acceptance', 'Supply the acceptance details');
}
else if (q.action === 'revise') {
    if (q.acceptance)
        invalid('acceptance', 'Revision carries only its version and reason');
}
else
    invalid('action', 'Choose record or revise'); return q; }
export function acceptance(q: Acceptance, s: Source, today: string) { const out = { ...q }; if (!['pending', 'accepted'].includes(out.status))
    invalid('status', 'Choose pending or accepted'); for (const [field, n, required] of [['officer_name', 300, out.status === 'accepted'], ['voucher_ref', 200, false], ['notes', 4000, false]] as const)
    out[field] = text(field, out[field], n, required); out.booked_amount = trim(out.booked_amount) || '0'; if (!/^(0|[1-9][0-9]{0,11})(\.[0-9]{1,2})?$/.test(out.booked_amount))
    invalid('booked_amount', 'Use an exact nonnegative amount with at most two decimal places'); if (out.acceptance_date && (!isoDate(out.acceptance_date) || out.acceptance_date < s.confirmation_date || out.acceptance_date > today) || out.status === 'accepted' && !out.acceptance_date)
    invalid('acceptance_date', 'Use actual acceptance date on or after engineer confirmation and not in the future'); return out; }
export function can(r: Record | null, a: Actor, c: Capabilities, action: string, s: Source) { accessGuard(a, c, true); guard(s); if (!['record', 'revise'].includes(action))
    invalid('action', 'Choose record or revise'); if (!r) {
    if (action !== 'record')
        invalid('acceptance', 'Record the acceptance before starting a revision');
    return;
} if (!r.is_active || r.project_id !== s.project_id || r.package_id !== s.package_id)
    throw missing(); if (action === 'record' && (!sameSource(r.source_snapshot, s) || r.acceptance.status === 'accepted'))
    throw stale(); if (action === 'revise' && r.acceptance.status !== 'accepted' && sameSource(r.source_snapshot, s))
    invalid('revision', 'Edit the current pending Acceptance instead'); }
const blank = (): Acceptance => ({ status: 'pending', acceptance_date: '', officer_name: '', voucher_ref: '', booked_amount: '0', notes: '' });
export function apply(base: Record | null, a: Actor, c: Capabilities, raw: Command, s: Source, at: string, newID: string): Record { const q = validate(raw); can(base, a, c, q.action, s); if (q.version !== (base?.version ?? 0))
    throw stale(); const out: Record = base ? structuredClone(base) : { id: newID, project_id: s.project_id, package_id: s.package_id, version: 0, revision: 1, source_snapshot: structuredClone(s), acceptance: blank(), is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id }; id(out.id); id(a.id); if (q.action === 'revise') {
    out.revision++;
    out.source_snapshot = structuredClone(s);
    out.acceptance = blank();
}
else
    out.acceptance = acceptance(q.acceptance!, s, indiaDate(new Date(at))); out.version = q.version + 1; out.updated_at = at; out.updated_by_id = a.id; return out; }
export function accepted(r: Record | null, s: Source) { guard(s); if (!r?.is_active)
    throw missing(); if (r.project_id !== s.project_id || r.package_id !== s.package_id || !sameSource(r.source_snapshot, s))
    throw stale(); if (r.version < 1 || r.revision < 1 || r.acceptance.status !== 'accepted')
    invalid('acceptance', 'Current issued Acceptance is required'); acceptance(r.acceptance, s, indiaDate(new Date(r.updated_at))); return structuredClone(r); }
export function financialEvent(before: Record | null, after: Record, action: string) { return after.acceptance.status === 'accepted' || action === 'revise' && before?.acceptance.status === 'accepted'; }
