import { transaction } from '../../db';
import { invalid, trim } from '../core/validation';
import { now } from '../core/time';
import { actor, actions } from './access';
import { id, nominations } from './validation';
import { source } from './source';
import { missing, money } from './policy';
import { forbidden, workspaceBound } from './service';
import type { Source, ApprovalQuote, Attempt, Nomination } from './types';
export function comparison(s: Source, group: string, keys: string[]) {
    if (keys.length > 50)
        invalid('quote_keys', 'Print at most 50 vendor revisions');
    const selected = new Map<string, ApprovalQuote>();
    for (const q of s.quotes) {
        if (q.group_id !== group || keys.length && !keys.includes(q.key))
            continue;
        const old = selected.get(q.document_id);
        if (!old) {
            selected.set(q.document_id, q);
            continue;
        }
        if (keys.length)
            invalid('quote_keys', 'Choose one revision per vendor column');
        if (q.eligible && !old.eligible || q.eligible && old.eligible && (money(q.total) < money(old.total) || money(q.total) === money(old.total) && q.revision > old.revision) || !q.eligible && !old.eligible && q.revision > old.revision)
            selected.set(q.document_id, q);
    }
    const rows = [...selected.values()];
    if (keys.length && rows.length !== keys.length)
        throw missing();
    return rows;
}
export function financeNotes(raw: Attempt, notes: Record<string, string>) { if (Object.keys(notes).length > 100 || raw.state !== 'pending_accounts')
    invalid('finance_notes', 'Draft notes require a pending submission'); for (const [id, note] of Object.entries(notes))
    if (!raw.lines.some(l => l.line_id === id) || typeof note !== 'string' || [...note].length > 2000)
        invalid('finance_notes', 'Use submitted item identifiers and notes of at most 2000 characters'); const a = structuredClone(raw); for (const l of a.lines)
    l.comment = trim(notes[l.line_id] ?? ''); for (const p of a.proposals ?? [])
    for (const l of p.snapshot.lines)
        l.comment = trim(notes[l.line_id] ?? ''); a.review_comment = 'Draft Finance notes - no decision recorded'; return a; }
export async function printModel(token: string, p: string, b: string, group: string, attemptID = '', proposals: Nomination[] = [], keys: string[] = [], notes: Record<string, string> = {}) {
    return transaction(async (db) => {
        const a = await actor(db, token);
        id(p);
        id(b);
        id(group);
        if (attemptID)
            id(attemptID);
        if (proposals.length)
            proposals = nominations(proposals);
        if (Object.keys(notes).length && !attemptID)
            invalid('finance_notes', 'Draft Finance notes require a frozen submission');
        const { caps, project } = await actions(db, a, p);
        if (!caps.read || Object.keys(notes).length && !caps.review_read)
            throw forbidden();
        const s = await source(db, p, b), g = s.groups.find(g => g.id === group);
        if (!g?.name)
            throw missing();
        let attempt: Attempt | null = null, quotes: ApprovalQuote[] = [];
        if (attemptID) {
            attempt = (await workspaceBound(db, a, p, b)).attempts.find(v => v.id === attemptID && v.group_id === group) ?? null;
            if (!attempt)
                throw missing();
            if (Object.keys(notes).length)
                attempt = financeNotes(attempt, notes);
        }
        else if (!proposals.length)
            quotes = comparison(s, group, keys);
        else {
            quotes = s.quotes.filter(q => q.group_id === group && proposals.some(n => n.quote_key === q.key));
            if (proposals.some(n => !quotes.some(q => q.key === n.quote_key)))
                throw missing();
        }
        return { project_name: project.name, project_number: project.project_number, group_id: group, group_name: g.name, currency: s.currency, quotes, attempt, nominations: proposals, finance: caps.review_read, generated_at: now() };
    });
}
