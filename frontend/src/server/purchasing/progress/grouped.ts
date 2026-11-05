import { isDeepStrictEqual } from 'node:util';
import type { DB } from '../../db';
import { isoDate, invalid } from '../core/validation';
import { quantity } from '../core/decimal';
import type { Record as BO } from '../bo/types';
import type { Line } from '../groups/types';
import { get as groups } from '../groups/repository';
import { list as documents } from '../indent-documents/repository';
import { get as quotations } from '../quotations/repository';
import { source as approvalSource } from '../approvals/source';
import { get as approvals } from '../approvals/repository';
import { evaluate } from '../approvals/policy';
import type { Evidence } from './types';
export function lineMatches(a: Line, b: Line | undefined) { if (!b || !a.quantity || !b.quantity || !a.required_date || !isoDate(a.required_date))
    return false; const item = (l: Line) => { const s = l.item_snapshot; return { id: s.id, sku: s.sku, description: s.description, engineering_key: s.engineering_key, uom: s.uom, manufacturer: s.manufacturer }; }; try {
    return a.id === b.id && quantity(a.quantity.replace(/(\.\d*?[1-9])0+$/, '$1').replace(/\.0+$/, '')) === quantity(b.quantity.replace(/(\.\d*?[1-9])0+$/, '$1').replace(/\.0+$/, '')) && a.required_date === b.required_date && isDeepStrictEqual(item(a), item(b));
}
catch {
    return false;
} }
export async function grouped(db: DB, p: string, b: BO): Promise<Evidence[]> { const grouping = await groups(db, p, b.id); if (!grouping?.is_active)
    return []; const lines = new Map(b.lines.map(l => [l.id, l])), assigned = new Set<string>(), expected = new Map<string, typeof grouping.groups[number]>(); let scopeComplete = lines.size > 0, vendorsReady = true; for (const g of grouping.groups) {
    if (!g.id || g.scope_revision < 1)
        invalid('groups', 'Use distinct current group revisions');
    if (!g.line_ids.length || new Set(g.line_ids).size !== g.line_ids.length)
        scopeComplete = false;
    for (const id of g.line_ids) {
        if (assigned.has(id) || !lines.has(id))
            scopeComplete = false;
        assigned.add(id);
    }
    if (g.vendor_ids.length < 3 || new Set(g.vendor_ids).size !== g.vendor_ids.length || g.vendor_ids.some(v => !v))
        vendorsReady = false;
    for (const v of g.vendor_ids)
        expected.set(g.id + ':' + v, g);
} for (const id of lines.keys())
    if (!assigned.has(id))
        scopeComplete = false; const docs = await documents(db, p, b.id), current = new Map<string, typeof docs[number]>(), documentIDs = new Set<string>(); for (const d of docs) {
    if (!d.is_active)
        continue;
    const key = d.group_id + ':' + d.vendor_id, g = expected.get(key);
    if (!g || d.data.scope_revision !== g.scope_revision || d.version < 1 || !d.id || d.data.lines.length !== g.line_ids.length)
        continue;
    const seen = new Set<string>();
    let valid = true;
    for (const line of d.data.lines) {
        if (seen.has(line.id) || !g.line_ids.includes(line.id) || !lineMatches(line, lines.get(line.id)))
            valid = false;
        seen.add(line.id);
    }
    if (!valid)
        continue;
    if (current.has(key) || documentIDs.has(d.id))
        invalid('documents', 'Multiple current vendor documents cannot supply one scope');
    current.set(key, d);
    documentIDs.add(d.id);
} const quotes = await quotations(db, p, b.id), byDoc = new Map((quotes?.is_active ? quotes.quotes : []).map(q => [q.document_id, q])); if (quotes?.is_active && byDoc.size !== quotes.quotes.length)
    invalid('quotations', 'Use one current quotation per document'); let sent = 0, received = 0; for (const d of current.values()) {
    if (!d.data.sent_date || !isoDate(d.data.sent_date))
        continue;
    sent++;
    const q = byDoc.get(d.id);
    if (!q || !isoDate(q.received_date))
        continue;
    const priced = q.calculation.lines.filter(l => l.landed !== null && l.line_id === l.source.id && lineMatches(l.source, d.data.lines.find(s => s.id === l.source.id))).map(l => l.source.id);
    if (priced.length === d.data.lines.length && new Set(priced).size === priced.length && d.data.lines.every(l => priced.includes(l.id)))
        received++;
} const source = await approvalSource(db, p, b.id), approval = await approvals(db, p, b.id), evaluated = evaluate(approval?.is_active ? approval.attempts : [], source), approved = [...evaluated.latest.values()].filter(a => a.effective_state === 'approved'), issued = b.state === 'completed' && grouping.state === 'created' && scopeComplete && vendorsReady && expected.size > 0; return [{ number: 2, state: issued && current.size === expected.size ? 'completed' : 'in_progress', details: [...lines.keys()].filter(id => assigned.has(id)).length + '/' + lines.size + ' items grouped; ' + current.size + '/' + expected.size + ' vendor indents generated' }, { number: 3, state: issued && sent === expected.size && received === expected.size ? 'completed' : sent > 0 || received > 0 ? 'in_progress' : 'not_started', details: sent + '/' + expected.size + ' invitations sent; ' + received + '/' + expected.size + ' quotations fully priced' }, { number: 4, state: grouping.groups.length > 0 && approved.length === grouping.groups.length ? 'completed' : !!approval?.attempts.length || approved.length > 0 ? 'in_progress' : 'not_started', details: approved.length + '/' + grouping.groups.length + ' groups approved by Accounts' }]; }
