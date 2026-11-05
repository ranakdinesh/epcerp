import { createPDF } from '../documents/pdf';
import { printModel } from './print-model';
import type { Attempt, Nomination } from './types';
export async function print(token: string, p: string, b: string, g: string, a = '', proposals: Nomination[] = [], keys: string[] = [], draftNotes: Record<string, string> = {}) {
    const s = await printModel(token, p, b, g, a, proposals, keys, draftNotes), rows: string[][] = [], notes: string[] = [], nominations = new Map(s.nominations.map(n => [n.quote_key, n]));
    let title = 'Group quotation comparison';
    const add = (q: Attempt, n?: Nomination) => { const tags: string[] = []; if (n?.best_price) {
        tags.push('Lowest price');
        notes.push(q.vendor + ' - Lowest price: ' + n.best_price_comment);
    } if (n?.preferred) {
        tags.push('Preferred');
        notes.push(q.vendor + ' - Preferred: ' + n.preferred_comment);
    } for (const l of q.lines) {
        const price = l.price, pricing = price ? 'Mode ' + price.pricing_mode + '; Unit ' + price.unit_price + '; discount ' + price.discount_percent + '%; tax ' + price.tax_percent + '%; charges ' + price.allocated_charges : '';
        const row = [q.vendor + '\n' + q.reference + ' R' + q.revision + '\n' + tags.join(' / '), l.item.description + '\n' + l.item.sku + '\n' + pricing + (l.months != null ? '\n' + l.coverage + ' ' + l.months + ' months' : '') + (l.executive_item_note ? '\n' + l.executive_item_note : ''), l.quantity + ' ' + l.item.uom, l.quoted];
        if (s.finance)
            row.push(l.budget ?? '', l.difference ?? '', (l.decision ? l.decision + ' - ' : '') + l.comment);
        rows.push(row);
    } notes.push(q.vendor + ' - total ' + s.currency + ' ' + q.total + ' | Valid until ' + (q.valid_until ?? '') + ' | ' + (q.terms ?? '')); };
    if (s.attempt) {
        title = 'Shortlisted quotation proposals';
        const a = s.attempt;
        notes.push('Submission ' + a.id + ' | ' + a.effective_state + ' | ' + a.message, 'Submitted ' + a.submitted_at + ' by ' + a.submitted_by_id, 'Executive summary: ' + a.reason, 'Finance summary: ' + (a.review_comment ?? ''));
        if (a.proposals?.length)
            for (const proposal of a.proposals)
                add({ ...proposal.snapshot, ...(a.quote_key === proposal.quote_key && a.state !== 'pending_accounts' ? { lines: a.lines } : {}) }, proposal);
        else
            add(a);
        if (a.return_action)
            notes.push('Finance return instruction: ' + a.return_action.replaceAll('_', ' '));
    }
    else {
        if (s.nominations.length) {
            title = 'Draft shortlisted quotation proposals';
            notes.push('Draft selection - not submitted for approval');
        }
        for (const q of s.quotes) {
            add({ ...q, currency: s.currency, quote_key: q.key, lines: q.lines.map(l => ({ ...l, months: l.months ?? undefined, decision: null, comment: '', executive_item_note: l.notes })), state: '', id: '', budget_revision: 0, verification_fingerprint: '', submitted_by_id: '', submitted_at: '', reason: '', reviewed_by_id: null, reviewed_at: null, decision_id: null }, nominations.get(q.key) ?? (!s.nominations.length && q.lowest ? { quote_key: q.key, best_price: true, preferred: false, best_price_comment: 'Lowest eligible landed group total', preferred_comment: '' } : undefined));
            if (!q.eligible)
                notes.push(q.vendor + ' R' + q.revision + ' excluded: ' + q.exclusions.join('; '));
        }
    }
    const columns = [{ title: 'Vendor / revision', width: 180 }, { title: 'Item / specification', width: 230 }, { title: 'Quantity', width: 55 }, { title: 'Amount incl. tax', width: 85 }];
    if (s.finance)
        columns.push({ title: 'Budget', width: 65 }, { title: 'Difference', width: 65 }, { title: 'Finance notes', width: 113 });
    return createPDF({ title: 'VTPL CORE | ' + title, subtitle: s.project_number + ' - ' + s.project_name + ' | ' + s.group_name, stamp: 'Generated ' + s.generated_at, landscape: true }, p => { p.table(columns, rows); for (const note of notes)
        p.paragraph(note, false, 9); });
}
