import { object, string, version, invalid, isoDate } from '../core/validation';
import type { Command } from './types';
import { id, text, mode, amount, rate } from './values';
export function date(field: string, value: string) { if (!isoDate(value))
    invalid(field, 'Use a calendar date'); return value; }
const strings = ['action', 'reason', 'approver_id', 'offer_id', 'supplier_id', 'due_date', 'rfq_terms', 'sent_date', 'channel', 'dispatch_reference', 'quote_reference', 'evidence_reference', 'valid_until', 'pricing_mode', 'line_total', 'unit_price', 'discount', 'tax', 'freight', 'other', 'payment_terms', 'warranty', 'compliance_notes', 'loi_terms'] as const;
export function input(raw: Record<string, unknown>): Command { const value = object(raw, ['version', ...strings, 'delivery_days', 'compliant', 'approve']), c = { version: version(value.version ?? 0), delivery_days: Number(value.delivery_days ?? 0), compliant: value.compliant ?? null, approve: value.approve ?? null } as Command; for (const field of strings)
    c[field] = string(value[field], field, ''); if (!Number.isInteger(c.delivery_days))
    invalid('delivery_days', 'Use 0–3650 delivery days'); for (const key of ['compliant', 'approve'] as const)
    if (c[key] !== null && typeof c[key] !== 'boolean')
        invalid(key, 'Record an explicit decision'); return validate(c); }
export function validate(raw: Command) {
    const c = { ...raw };
    if (c.version < 0)
        invalid('version', 'Use the current version');
    c.reason = text('reason', c.reason, 2000);
    if (!['configure', 'rfq', 'dispatch', 'no_bid', 'quote', 'propose', 'decide', 'reopen', 'release'].includes(c.action))
        invalid('action', 'Choose a sourcing action');
    if (c.action === 'configure')
        id(c.approver_id);
    else if (c.action === 'rfq') {
        id(c.supplier_id);
        if (c.offer_id)
            id(c.offer_id);
        date('due_date', c.due_date);
        c.rfq_terms = text('rfq_terms', c.rfq_terms, 4000);
    }
    else if (c.action === 'no_bid')
        id(c.offer_id);
    else if (['dispatch', 'release'].includes(c.action)) {
        if (c.action === 'dispatch')
            id(c.offer_id);
        date('sent_date', c.sent_date);
        if (!['email', 'whatsapp', 'manual'].includes(c.channel))
            invalid('channel', 'Choose email, whatsapp or manual');
        c.dispatch_reference = text('dispatch_reference', c.dispatch_reference, 1000);
    }
    else if (c.action === 'quote') {
        id(c.offer_id);
        date('valid_until', c.valid_until);
        for (const [field, n] of [['quote_reference', 200], ['evidence_reference', 1000], ['payment_terms', 2000], ['warranty', 1000], ['compliance_notes', 2000]] as const)
            c[field] = text(field, c[field], n);
        if (c.delivery_days < 0 || c.delivery_days > 3650)
            invalid('delivery_days', 'Use 0–3650 delivery days');
        if (c.compliant === null)
            invalid('compliant', 'Record an explicit technical compliance decision');
        c.pricing_mode = mode(c.pricing_mode);
        if (c.pricing_mode === 'line_total') {
            if (c.unit_price)
                invalid('unit_price', 'Use only the direct line total');
            c.line_total = amount('line_total', c.line_total);
        }
        else {
            if (c.line_total)
                invalid('line_total', 'Use only the unit rate');
            c.unit_price = rate(c.unit_price);
        }
        for (const field of ['discount', 'tax', 'freight', 'other'] as const)
            c[field] = amount(field, c[field]);
    }
    else if (c.action === 'propose') {
        id(c.offer_id);
        c.loi_terms = text('loi_terms', c.loi_terms, 4000);
    }
    else if (c.action === 'decide' && c.approve === null)
        invalid('approve', 'Record an explicit approval or rejection');
    return c;
}
