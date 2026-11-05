import { isDeepStrictEqual } from 'node:util';
import { PurchaseError, invalid, isoDate } from '../core/validation';
import { id } from '../sourcing/values';
import { text } from '../orders/policy';
import { indiaDate } from '../../personal-tasks/validation';
import type { Actor } from '../core/access';
import type { Record, Source, Capabilities, Command } from './types';
export const forbidden = () => new PurchaseError(403, 'Current vendor-acceptance permission and stage10 scope are required');
export const missing = () => new PurchaseError(404, 'Vendor acceptance or its released PO not found');
export const stale = () => new PurchaseError(409, 'Vendor acceptance or released PO changed; retain your draft, reload and review');
export function accessGuard(a: Actor, c: Capabilities, write: boolean) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!c.read || write && (!c.write || !c.work))
    throw forbidden(); }
export function guard(s: Source) { if (!s.available || s.blockers.length || !s.order)
    invalid('source', 'An actual released PO and current project/task prerequisites are required'); const o = s.order; if (o.project_id !== s.project_id || o.package_id !== s.package_id)
    invalid('source', 'Released PO belongs to another project or package'); for (const v of [o.id, o.project_id, o.package_id, o.supplier_id, o.released_by_id])
    id(v); if (o.version < 2 || !o.number || !isoDate(o.date) || !o.released_at || o.released_at.startsWith('0001-01-01T00:00:00') || !o.supplier_name || !o.quantity || !o.total || !o.uom || !o.currency)
    invalid('purchase_order', 'The released PO evidence is incomplete'); }
export function validate(raw: Command) { const q = structuredClone(raw); if (q.version < 0)
    invalid('version', 'Use the current nonnegative version'); if (q.action !== 'record')
    invalid('action', 'Choose record'); q.reason = text('reason', q.reason, 2000); const r = q.response; r.status = text('status', r.status, 32); if (!['pending', 'accepted', 'accepted_with_exceptions'].includes(r.status))
    invalid('status', 'Choose pending, accepted or accepted with exceptions'); r.signatory_name = text('signatory_name', r.signatory_name, 200, false); r.document_reference = text('document_reference', r.document_reference, 300, false); r.remarks = text('remarks', r.remarks, 4000, r.status === 'accepted_with_exceptions'); r.accepted_date = text('accepted_date', r.accepted_date, 10, false); if (r.status === 'pending' && r.accepted_date)
    invalid('accepted_date', 'A pending response has no vendor acceptance date'); if (r.status !== 'pending' && !isoDate(r.accepted_date))
    invalid('accepted_date', 'Provide the actual vendor acceptance date'); return q; }
export function apply(base: Record | null, a: Actor, caps: Capabilities, raw: Command, source: Source, at: string, newID: string): Record { const q = validate(raw); accessGuard(a, caps, true); guard(source); if (q.response.status !== 'pending' && (q.response.accepted_date < source.order!.date || q.response.accepted_date > indiaDate(new Date(at))))
    invalid('accepted_date', 'Acceptance date must be between the issued PO date and today'); const out: Record = base ? structuredClone(base) : { id: newID, project_id: source.project_id, package_id: source.package_id, order_snapshot: structuredClone(source.order!), is_active: true, created_at: at, created_by_id: a.id, version: 0, response: q.response, updated_at: at, updated_by_id: a.id }; if (!base) {
    if (q.version !== 0)
        throw stale();
}
else {
    if (!base.is_active || base.project_id !== source.project_id || base.package_id !== source.package_id)
        throw missing();
    if (q.version !== base.version || !isDeepStrictEqual(base.order_snapshot, source.order))
        throw stale();
} id(out.id); id(a.id); out.version = q.version + 1; out.response = q.response; out.updated_at = at; out.updated_by_id = a.id; return out; }
export function accepted(r: Record | null, s: Source) { guard(s); if (!r?.is_active)
    throw missing(); if (r.project_id !== s.project_id || r.package_id !== s.package_id || !isDeepStrictEqual(r.order_snapshot, s.order))
    throw stale(); if (r.response.status !== 'accepted' || !isoDate(r.response.accepted_date) || r.response.accepted_date < r.order_snapshot.date || r.version < 1)
    invalid('acceptance', 'Vendor acceptance without unresolved exceptions is required'); return structuredClone(r); }
