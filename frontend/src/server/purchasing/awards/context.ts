import type { DB } from '../../db';
import { actor, project, scope, type Actor } from '../core/access';
import { workspace } from '../workspace/repository';
import { AppError } from '../../errors';
import { list } from '../bo/repository';
import { approved, commercial } from '../approvals/service';
import { revisions } from '../quotations/repository';
import type { Attempt, ApprovalLine, CommercialPrice } from '../approvals/types';
import { invalid } from '../core/validation';
export type Candidate = {
    bo_id: string;
    attempt: Attempt;
    lines: {
        demand_id: string;
        approval: ApprovalLine;
        price: CommercialPrice;
        delivery_days: number;
        payment_terms: string;
        warranty: string;
    }[];
};
export async function authorize(db: DB, token: string, p: string, write: boolean) { const code = write ? 'projects.purchase_awards.write' : 'projects.purchase_sourcing.read', a = await actor(db, token, [code]); await access(db, a, p, write); return a; }
export async function access(db: DB, a: Actor, p: string, write: boolean) { const parent = await project(db, p, true); await scope(db, a, parent, await workspace(db, p), write ? 'projects.purchase_awards.write' : 'projects.purchase_sourcing.read', write); }
export async function candidates(db: DB, p: string): Promise<Candidate[]> { const out: Candidate[] = []; for (const bo of await list(db, p)) {
    if (!bo.is_active || bo.state !== 'completed')
        continue;
    let attempts;
    try {
        attempts = await approved(db, p, bo.id, false);
    }
    catch (e) {
        if (e instanceof AppError && [422, 403].includes(e.status))
            continue;
        throw e;
    }
    const prices = await commercial(db, p, bo.id, false), quotes = await revisions(db, p, bo.id);
    for (const a of attempts) {
        const lines: Candidate['lines'] = [];
        for (const l of a.lines) {
            const b = bo.lines.find(b => b.id === l.line_id && b.demand_id && b.item_id === l.item.id);
            if (!b?.demand_id)
                invalid('source', 'Approved item has no current published requirement');
            const price = prices.find(p => p.attempt_id === a.id && p.line_id === l.line_id && p.item_id === l.item.id)?.price;
            if (!price)
                invalid('price', 'Current approved item pricing is unavailable');
            const q = quotes.find(q => q.document_id === a.document_id && q.revision === a.revision), ql = q?.lines.find(v => v.line_id === l.line_id);
            let warranty = ql?.coverage_type ?? '';
            if (ql?.warranty_months != null)
                warranty += ' · ' + ql.warranty_months + ' months';
            if (ql?.notes)
                warranty += ' · ' + ql.notes;
            lines.push({ demand_id: b.demand_id, approval: l, price, delivery_days: q?.delivery_days ?? 0, payment_terms: q?.payment_terms ?? '', warranty });
        }
        out.push({ bo_id: bo.id, attempt: a, lines });
    }
} return out; }
