import type { DB } from '../../db';
import { timestamp, timeSQL } from '../../inbox/repository';
import { journal } from '../../purchasing/core/journal';
import { randomUUID } from 'node:crypto';
import type { Budget, Data, Head, Allocation, Event } from './types';
import { stale, invalid, missing } from './validation';
const exact = `${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at`;
// pgx Numeric.Value represents every numeric zero as the unscaled string '0'.
const numeric = (s: string | null) => s !== null && !/[1-9]/.test(s) ? '0' : s;
const audit = (r: Record<string, unknown>) => ({
    created_at: timestamp(r.created_at as string)!, updated_at: timestamp(r.updated_at as string)!, created_by_id: r.created_by_id as string | null, updated_by_id: r.updated_by_id as string | null
});
export async function get(db: DB, p: string): Promise<Budget | null> {
    const r = (await db.query(`SELECT *,total::text,${exact} FROM project_budgets WHERE project_id=$1`, [p])).rows[0];
    if (!r)
        return null;
    const heads = (await db.query(`SELECT *,amount::text,${exact} FROM project_budget_heads WHERE project_id=$1 ORDER BY sort_order,id`, [p])).rows.map(h => ({
        ...audit(h), id: h.id, parent_id: h.parent_id, name: h.name, kind: h.kind, amount: numeric(h.amount)!, owner_id: h.owner_id, is_active: h.is_active
    } as Head));
    const allocations = (await db.query(`SELECT *,amount::text,unit_price::text,pricing_quantity::text,${exact} FROM project_budget_allocations WHERE project_id=$1 ORDER BY sort_order,id`, [p])).rows.map(a => ({
        ...audit(a), id: a.id, bo_id: a.bo_id, line_id: a.line_id, head_id: a.head_id, amount: numeric(a.amount)!, owner_id: a.owner_id, is_active: a.is_active, unit_price: numeric(a.unit_price), pricing_quantity: numeric(a.pricing_quantity), source: a.source_snapshot
    } as Allocation));
    return {
        ...audit(r), project_id: p, version: r.version, revision: r.revision, state: r.state, is_active: r.is_active, approved: r.approved_snapshot, data: {
            total: numeric(r.total)!, currency: r.currency, owner_id: r.owner_id, notes: r.notes, ...(r.pricing_mode ? { pricing_mode: r.pricing_mode } : {}), heads, allocations
        }
    };
}
export async function save(db: DB, b: Budget, actor: string) {
    try {
        const d = b.data, r = b.version === 1 ? await db.query("INSERT INTO project_budgets(project_id,version,revision,state,total,currency,owner_id,notes,pricing_mode,is_active,created_by_id,updated_by_id)VALUES($1,1,$2,'draft',$3,$4,$5,$6,$7,true,$8,$8)RETURNING project_id", [b.project_id, b.revision, d.total, d.currency, d.owner_id, d.notes, d.pricing_mode ?? '', actor]) : await db.query('UPDATE project_budgets SET version=version+1,revision=$3,state=$4,approved_snapshot=$5,approval_decision_id=$6,total=$7,currency=$8,owner_id=$9,notes=$10,pricing_mode=$11,is_active=$12,updated_at=clock_timestamp(),updated_by_id=$13 WHERE project_id=$1 AND version=$2 RETURNING project_id', [b.project_id, b.version - 1, b.revision, b.state, JSON.stringify(b.approved), b.approved?.decision_id ?? null, d.total, d.currency, d.owner_id, d.notes, d.pricing_mode ?? '', b.is_active, actor]);
        if (!r.rowCount)
            throw stale();
        for (const [index, h] of d.heads.entries()) {
            const r = await db.query(`INSERT INTO project_budget_heads(id,project_id,sort_order,parent_id,name,kind,amount,owner_id,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT(id)DO UPDATE SET sort_order=EXCLUDED.sort_order,parent_id=EXCLUDED.parent_id,name=EXCLUDED.name,kind=EXCLUDED.kind,amount=EXCLUDED.amount,owner_id=EXCLUDED.owner_id,is_active=EXCLUDED.is_active,updated_at=EXCLUDED.updated_at,updated_by_id=EXCLUDED.updated_by_id WHERE project_budget_heads.project_id=EXCLUDED.project_id`, [h.id, b.project_id, index, h.parent_id, h.name, h.kind, h.amount, h.owner_id, h.is_active, h.created_at, h.updated_at, h.created_by_id, actor]);
            if (r.rowCount !== 1)
                invalid('heads', 'A head identifier belongs to a different project');
        }
        for (const active of [false, true])
            for (const [index, a] of d.allocations.entries()) {
                if (a.is_active !== active)
                    continue;
                const r = await db.query(`INSERT INTO project_budget_allocations(id,project_id,sort_order,bo_id,line_id,head_id,amount,owner_id,is_active,unit_price,pricing_quantity,source_snapshot,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT(id)DO UPDATE SET sort_order=EXCLUDED.sort_order,head_id=EXCLUDED.head_id,amount=EXCLUDED.amount,owner_id=EXCLUDED.owner_id,is_active=EXCLUDED.is_active,unit_price=EXCLUDED.unit_price,pricing_quantity=EXCLUDED.pricing_quantity,source_snapshot=EXCLUDED.source_snapshot,updated_at=EXCLUDED.updated_at,updated_by_id=EXCLUDED.updated_by_id WHERE project_budget_allocations.project_id=EXCLUDED.project_id AND project_budget_allocations.bo_id=EXCLUDED.bo_id AND project_budget_allocations.line_id=EXCLUDED.line_id`, [a.id, b.project_id, index, a.bo_id, a.line_id, a.head_id, a.amount, a.owner_id, a.is_active, a.unit_price, a.pricing_quantity, JSON.stringify(a.source), a.created_at, a.updated_at, a.created_by_id, actor]);
                if (r.rowCount !== 1)
                    invalid('allocations', 'An allocation identifier belongs to a different project or source item');
            }
        const saved = await get(db, b.project_id);
        if (!saved)
            throw missing();
        return saved;
    }
    catch (e) {
        if ((e as {
            code?: string;
        }).code === '23503')
            invalid('data', 'A budget reference is unavailable or belongs to another project');
        if ((e as {
            code?: string;
        }).code === '23505')
            invalid('data', 'A budget record or active allocation already exists');
        throw e;
    }
}
export async function event(db: DB, before: Budget | null, after: Budget, action: string, reason: string, actor: string) {
    const id = randomUUID();
    await db.query('INSERT INTO project_budget_events(id,project_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7)', [id, after.project_id, action, reason, JSON.stringify(before), JSON.stringify(after), actor]);
    await journal(db, {
        id, module: 'project_budgets', type: 'project_budget', entity: after.project_id, version: after.version, action, actor, project: after.project_id, at: after.updated_at
    });
}
export async function history(db: DB, p: string): Promise<Event[]> {
    return (await db.query(`SELECT id,project_id,action,reason,before_snapshot AS before,after_snapshot AS after,created_by_id,${timeSQL('created_at')} AS created_at FROM project_budget_events WHERE project_id=$1 ORDER BY created_at DESC,id DESC LIMIT 200`, [p])).rows.map(e => ({ ...e, created_at: timestamp(e.created_at)! } as Event));
}
export function canonicalData(d: Data) {
    const audit = (h: Head | Allocation) => ({
        created_at: h.created_at, updated_at: h.updated_at, created_by_id: h.created_by_id, updated_by_id: h.updated_by_id
    });
    return {
        total: d.total, currency: d.currency, owner_id: d.owner_id, notes: d.notes, ...(d.pricing_mode ? { pricing_mode: d.pricing_mode } : {}), heads: d.heads.map(h => ({
            ...audit(h), id: h.id, parent_id: h.parent_id, name: h.name, kind: h.kind, amount: h.amount, owner_id: h.owner_id, is_active: h.is_active
        })), allocations: d.allocations.map(a => ({
            ...audit(a), id: a.id, bo_id: a.bo_id, line_id: a.line_id, head_id: a.head_id, amount: a.amount, owner_id: a.owner_id, is_active: a.is_active, unit_price: a.unit_price, pricing_quantity: a.pricing_quantity, source: a.source ? {
                bo_id: a.source.bo_id, line_id: a.source.line_id, bo_version: a.source.bo_version, bo_name: a.source.bo_name, quantity: a.source.quantity, sku: a.source.sku, description: a.source.description, uom: a.source.uom, manufacturer: a.source.manufacturer, item_id: a.source.item_id, item_snapshot: {
                    sku: a.source.item_snapshot.sku, description: a.source.item_snapshot.description, uom: a.source.item_snapshot.uom, manufacturer: a.source.item_snapshot.manufacturer
                }
            } : null
        }))
    };
}
