import { invalid, trim } from '../core/validation';
import type { Attempt, Nomination, Source, Draft } from './types';
import { nominations, id } from './validation';
import { money, snapshot, missing, evaluate } from './policy';
export function shortlist(raw: Nomination[], s: Source): Attempt {
    const rows = nominations(raw), proposals: NonNullable<Attempt['proposals']> = [];
    let group = '';
    for (const n of rows) {
        const q = s.quotes.find(q => q.key === n.quote_key);
        if (!q)
            throw missing();
        if (group && group !== q.group_id)
            invalid('proposals', 'Both proposals must belong to the same current group');
        group = q.group_id;
        if (n.best_price) {
            const price = money(q.total);
            for (const other of s.quotes)
                if (other.group_id === group && other.eligible && money(other.total) < price)
                    invalid('best_price', 'Best price must be the lowest eligible group total');
        }
        proposals.push({ ...n, snapshot: snapshot(q, s) });
    }
    const chosen = structuredClone((proposals.find(p => p.best_price) ?? proposals[0]).snapshot);
    chosen.proposals = proposals;
    return chosen;
}
export function selectProposal(raw: Attempt, key: string): Attempt {
    const a = structuredClone(raw);
    if (!a.proposals?.length) {
        if (key && key !== a.quote_key)
            invalid('selected_quote_key', 'Select the submitted quotation');
        return a;
    }
    const p = a.proposals.find(p => p.quote_key === key);
    if (!p)
        invalid('selected_quote_key', 'Choose one of the frozen submitted proposals');
    return { ...structuredClone(p.snapshot), id: a.id, state: a.state, submitted_by_id: a.submitted_by_id, submitted_at: a.submitted_at, reason: a.reason, proposals: a.proposals };
}
export function validateDraft(raw: Draft, s: Source, attempts: Attempt[]): Draft {
    const draft = structuredClone(raw);
    if (draft.version < 0)
        invalid('version', 'Use a nonnegative draft version');
    id(draft.group_id);
    if (!s.groups.some(g => g.id === draft.group_id))
        invalid('group_id', 'Choose a current purchase group');
    draft.proposals = nominations(draft.proposals);
    draft.comment = trim(draft.comment);
    if ([...draft.comment].length > 2000)
        invalid('comment', 'Use at most 2000 characters');
    const latest = evaluate(attempts, s).latest.get(draft.group_id);
    if (latest && ['approved', 'pending_accounts'].includes(latest.effective_state ?? ''))
        invalid('group_id', 'This group is frozen for Finance review or approved');
    for (const n of draft.proposals) {
        const q = s.quotes.find(q => q.key === n.quote_key);
        if (!q)
            throw missing();
        if (q.group_id !== draft.group_id || !q.eligible)
            invalid('proposals', 'Choose eligible offers from this group');
        if (n.best_price) {
            const price = money(q.total);
            for (const other of s.quotes)
                if (other.group_id === draft.group_id && other.eligible && money(other.total) < price)
                    invalid('best_price', 'Best price must be the lowest eligible group total');
        }
    }
    return draft;
}
