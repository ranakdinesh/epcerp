import { isDeepStrictEqual } from 'node:util';
import { PurchaseError, invalid, isoDate, trim } from '../core/validation';
import { id } from '../sourcing/values';
import { Rational } from '../core/rational';
import type { Actor } from '../core/access';
import { indiaDate } from '../../personal-tasks/validation';
import type { Source, Capabilities, Order, Command, Details } from './types';
export const forbidden = () => new PurchaseError(403, 'Purchase-order access and assigned responsibility are required');
export const missing = () => new PurchaseError(404, 'Purchase order or its source not found');
export const stale = () => new PurchaseError(409, 'Purchase order or source changed; retain your draft, reload and review');
export function text(field: string, value: string, max: number, required = true) { value = trim(value); if (required && !value || [...value].length > max)
    invalid(field, 'Provide a value within the field limit'); return value; }
export function value(field: string, s: string, precision: number, positive = false) { if (!/^(0|[1-9][0-9]*)(\.[0-9]{1,4})?$/.test(s) || s.length > 22)
    invalid(field, 'Use an exact nonnegative decimal'); const [w, f = ''] = s.split('.'), r = new Rational(BigInt(w + f), BigInt(10) ** BigInt(f.length)); if (f.length > precision || positive && r.numerator <= BigInt(0))
    invalid(field, 'Use the permitted decimal precision and a positive quantity'); return r; }
export function guard(s: Source) { if (!s.available || s.blockers.length)
    invalid('source', 'Resolve current supplier, Finance, technical and project blockers before preparing or releasing the PO'); for (const v of [s.project_id, s.package_id, s.sourcing_id, s.supplier.id, s.technical.case_id, s.technical.approval_file_id, s.item_id])
    id(v); if (s.sourcing_version < 1 || s.technical.version < 1 || s.technical.revision < 1 || !s.technical.qap_approved || !s.technical.conditions_closed || !s.loi_reference || !s.financial_reference || !s.supplier_verification || !isoDate(s.loi_date) || !isoDate(s.technical.approval_date) || !s.technical.reference || !s.supplier.name || !s.supplier.address || !s.description || !s.uom || !/^[A-Z]{3}$/.test(s.currency))
    invalid('source', 'Actual released commercial, verified supplier and approved technical evidence is incomplete'); if (s.delivery_days < 1 || s.delivery_days > 3650 || !s.payment_terms || !s.warranty || !s.loi_terms)
    invalid('terms', 'Actual approved delivery, payment, warranty and LOI terms are required'); const q = value('quantity', s.quantity, 4, true), base = value('base', s.base, 2); if (s.pricing_mode === 'unit_price') {
    const rate = value('unit_price', s.unit_price, 4);
    if (q.multiply(rate).fixed() !== base.fixed())
        invalid('base', 'Exact approved unit rate and quantity do not match the basic amount');
}
else if (s.pricing_mode !== 'line_total' || s.unit_price)
    invalid('pricing_mode', 'Use the exact approved unit-rate or direct-line-total pricing mode'); const discount = value('discount', s.discount, 2), tax = value('tax', s.tax, 2), freight = value('freight', s.freight, 2), other = value('other', s.other, 2), total = value('total', s.total, 2); if (discount.compare(base) > 0)
    invalid('discount', 'Discount exceeds the approved basic value'); if (base.subtract(discount).add(tax).add(freight).add(other).compare(total) !== 0)
    invalid('total', 'Order amounts must equal the exact current Finance-approved total'); }
export function sameSource<T extends {
    available: boolean;
    blockers: string[];
}>(a: T, b: T) { const facts = (s: T) => { const { available, blockers, ...facts } = s; void available; void blockers; return facts; }; return isDeepStrictEqual(facts(a), facts(b)); }
export function accessGuard(a: Actor, c: Capabilities, action: string) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!c.read)
    throw forbidden(); if (action === 'read')
    return; if (!c.work || !c.write || action === 'release' && !c.release || action === 'client_decision' && !c.record_client)
    throw forbidden(); }
export function validate(q: Command) { q = { ...q, reason: text('reason', q.reason, 2000) }; if (q.version < 0)
    invalid('version', 'Use the current nonnegative version'); if (!['prepare', 'release', 'submit_client', 'client_decision'].includes(q.action))
    invalid('action', 'Choose prepare, submit_client, client_decision or release'); if ((q.action === 'prepare') !== (q.details !== null))
    invalid('details', 'Only preparation accepts complete order details'); if ((q.action === 'submit_client') !== (q.submission !== null))
    invalid('submission', 'Only client submission accepts its reference and date'); if ((q.action === 'client_decision') !== (q.client_decision !== null))
    invalid('client_decision', 'Only client decision accepts signed approval evidence'); return q; }
export function details(raw: Details, s: Source, today: string) { const d = { ...raw }; for (const [field, n, required] of [['issuer_name', 200, true], ['issuer_address', 2000, true], ['issuer_tax_reference', 80, false], ['issuer_registration', 120, false], ['issuer_email', 254, false], ['bill_to', 2000, true], ['delivery_address', 2000, true]] as const)
    d[field] = text(field, d[field], n, required); if (d.issuer_email && !/^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(d.issuer_email))
    invalid('issuer_email', 'Use a valid email address'); if (!isoDate(d.date) || d.date < s.loi_date || d.date < s.technical.approval_date || d.date > today)
    invalid('date', 'PO date must be between LOI/client approval and today'); return d; }
export function fiscalYear(date: string) { if (!isoDate(date))
    invalid('date', 'Use a valid PO date'); return Number(date.slice(0, 4)) - (Number(date.slice(5, 7)) < 4 ? 1 : 0); }
export function number(date: string, sequence: bigint) { const year = fiscalYear(date); if (sequence < BigInt(1) || sequence > BigInt(999999999))
    invalid('number', 'Purchase-order sequence exceeds its supported range'); return `PO/VTP/${String(year).padStart(4, '0')}-${String((year + 1) % 100).padStart(2, '0')}/${sequence.toString().padStart(6, '0')}`; }
export function clientCleared(o: Order) { const a = o.client_approval; return o.draft_revision > 0 && a.status === 'approved' && a.draft_revision === o.draft_revision && !!a.submission && !!a.decision && a.decision.status === 'approved' && !!a.decision.file_id && !!a.recorded_by_id && !!a.recorded_at; }
export function basicAmount(mode: string, rate: string, lineTotal: string, quantity: string) { if (mode === 'line_total')
    return value('base', lineTotal, 2).fixed(); if (mode && mode !== 'unit_price')
    invalid('pricing_mode', 'Unsupported approved price mode'); return value('unit_price', rate, 4).multiply(value('quantity', quantity, 4, true)).fixed(); }
export function apply(base: Order | null, a: Actor, caps: Capabilities, raw: Command, s: Source, at: string, newID: string, number: string): Order {
    const q = validate(raw);
    accessGuard(a, caps, q.action);
    guard(s);
    const version = base?.version ?? 0;
    if (base) {
        if (!base.is_active)
            throw missing();
        if (base.state !== 'draft')
            invalid('state', 'Released orders are immutable; use a governed amendment workflow');
    }
    if (q.version !== version)
        throw stale();
    const out: Order = base ? structuredClone(base) : { draft_revision: 0, client_approval: { status: '', draft_revision: 0, submitted_by_id: '', recorded_by_id: '' }, id: newID, project_id: s.project_id, package_id: s.package_id, version: 1, state: 'draft', number: '', source_snapshot: s, details: { date: '', issuer_name: '', issuer_address: '', issuer_tax_reference: '', issuer_registration: '', issuer_email: '', bill_to: '', delivery_address: '' }, released_at: null, released_by_id: '', is_active: true, created_at: at, created_by_id: a.id, updated_at: '0001-01-01T00:00:00Z', updated_by_id: '' };
    if (base)
        out.version++;
    const today = indiaDate(new Date(at));
    if (q.action === 'prepare') {
        out.details = details(q.details!, s, today);
        out.draft_revision++;
        out.client_approval = { status: 'not_submitted', draft_revision: out.draft_revision, submitted_by_id: '', recorded_by_id: '' };
        out.source_snapshot = structuredClone(s);
    }
    else {
        if (!base)
            invalid('order', 'Prepare the complete order before client submission or release');
        if (!sameSource(out.source_snapshot, s))
            throw stale();
        details(out.details, s, today);
        if (q.action === 'submit_client') {
            if (out.draft_revision < 1)
                invalid('order', 'Review and save the existing draft before client submission');
            if (['submitted', 'approved'].includes(out.client_approval.status))
                invalid('state', 'Revise the draft before submitting it again');
            const submission = { ...q.submission!, reference: text('reference', q.submission!.reference, 200) };
            if (!isoDate(submission.date) || submission.date < out.details.date || submission.date > today)
                invalid('date', 'Submission date must be between draft date and today');
            out.client_approval = { status: 'submitted', draft_revision: out.draft_revision, submission, submitted_by_id: a.id, recorded_by_id: '' };
        }
        else if (q.action === 'client_decision') {
            if (out.client_approval.status !== 'submitted' || out.client_approval.draft_revision !== out.draft_revision)
                invalid('state', 'Submit this exact draft to the client before recording a decision');
            const d = { ...q.client_decision! };
            if (!['approved', 'rejected'].includes(d.status))
                invalid('status', 'Choose approved or rejected');
            for (const [field, n] of [['reference', 200], ['signatory', 200], ['designation', 200]] as const)
                d[field] = text(field, d[field], n);
            if (!isoDate(d.date) || d.date < out.client_approval.submission!.date || d.date > today)
                invalid('date', 'Decision date must be between submission and today');
            if (d.status === 'approved') {
                try {
                    id(d.file_id);
                }
                catch {
                    invalid('file_id', 'Upload the client stamped and signed PO');
                }
            }
            else if (d.file_id)
                id(d.file_id);
            Object.assign(out.client_approval, { status: d.status, decision: d, recorded_by_id: a.id, recorded_at: at });
        }
        else if (q.action === 'release') {
            if (!clientCleared(out))
                invalid('client_approval', 'Record client stamp/sign approval for the current draft before PO release');
            if (!number)
                invalid('number', 'Allocate an owned PO number before release');
            Object.assign(out, { state: 'released', number, released_by_id: a.id, released_at: at });
        }
    }
    out.updated_at = at;
    out.updated_by_id = a.id;
    return out;
}
