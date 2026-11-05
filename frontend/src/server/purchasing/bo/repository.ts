import type { DB } from '../../db';
import { timestamp, timeSQL } from '../../inbox/repository';
import type { Record, Line } from './types';
import { PurchaseError } from '../core/validation';
import { randomUUID } from 'node:crypto';
import { journal } from '../core/journal';
const fields = `id,project_id,preparer_id,name,version,state,is_active,${timeSQL('completed_at')} AS completed_at,completed_by_id,${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at,created_by_id,updated_by_id`;
async function view(db: DB, row: Omit<Record, 'lines'>): Promise<Record> {
    const lines = (await db.query("SELECT id,item_id,quantity::text,required_date::text,reference,notes,item_snapshot,demand_id FROM purchase_bo_lines WHERE bo_id=$1 AND is_active ORDER BY sort_order,id", [row.id])).rows as Line[];
    return { ...row, created_at: timestamp(row.created_at)!, updated_at: timestamp(row.updated_at)!, completed_at: timestamp(row.completed_at), lines };
}
export async function list(db: DB, project: string) { const rows = (await db.query(`SELECT ${fields} FROM purchase_bo_lists WHERE project_id=$1 ORDER BY created_at,id`, [project])).rows, out: Record[] = []; for (const row of rows)
    out.push(await view(db, row)); return out; }
export async function get(db: DB, project: string, id: string) { const row = (await db.query(`SELECT ${fields} FROM purchase_bo_lists WHERE project_id=$1 AND id=$2`, [project, id])).rows[0]; return row ? view(db, row) : null; }
export const stale = () => new PurchaseError(409, 'BO list or linked material requirement changed; reload the saved version');
export async function save(db: DB, record: Record, actor: string) {
    const result = record.version === 1 ? await db.query('INSERT INTO purchase_bo_lists(id,project_id,preparer_id,name,state,is_active,version,completed_at,completed_by_id,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,true,1,$6,$7,$8,$8) RETURNING id', [record.id, record.project_id, record.preparer_id, record.name, record.state, record.completed_at, record.completed_by_id, actor]) : await db.query('UPDATE purchase_bo_lists SET name=$3,state=$4,version=version+1,completed_at=$5,completed_by_id=$6,updated_at=clock_timestamp(),updated_by_id=$7 WHERE project_id=$1 AND id=$2 AND version=$8 RETURNING id', [record.project_id, record.id, record.name, record.state, record.completed_at, record.completed_by_id, actor, record.version - 1]);
    if (!result.rowCount)
        throw stale();
    for (const [index, line] of record.lines.entries()) {
        const row = await db.query(`INSERT INTO purchase_bo_lines(id,bo_id,item_id,quantity,required_date,reference,notes,item_snapshot,demand_id,sort_order,is_active,created_by_id,updated_by_id)VALUES($1,$2,$3,$4::numeric,$5::date,$6,$7,$8,$9,$10,true,$11,$11) ON CONFLICT(id)DO UPDATE SET item_id=EXCLUDED.item_id,quantity=EXCLUDED.quantity,required_date=EXCLUDED.required_date,reference=EXCLUDED.reference,notes=EXCLUDED.notes,item_snapshot=EXCLUDED.item_snapshot,demand_id=EXCLUDED.demand_id,sort_order=EXCLUDED.sort_order,is_active=true,updated_at=clock_timestamp(),updated_by_id=EXCLUDED.updated_by_id WHERE purchase_bo_lines.bo_id=EXCLUDED.bo_id RETURNING id`, [line.id, record.id, line.item_id, line.quantity, line.required_date, line.reference, line.notes, JSON.stringify(line.item_snapshot), line.demand_id, index, actor]);
        if (!row.rowCount)
            throw stale();
    }
    await db.query('UPDATE purchase_bo_lines SET is_active=false,updated_at=clock_timestamp(),updated_by_id=$2 WHERE bo_id=$1 AND is_active AND NOT(id=ANY($3::uuid[]))', [record.id, actor, record.lines.map(l => l.id)]);
    return (await get(db, record.project_id, record.id))!;
}
export async function event(db: DB, before: Record | null, after: Record, action: string, reason: string, actor: string) { const id = randomUUID(); await db.query('INSERT INTO purchase_bo_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7)', [id, after.id, action, reason, JSON.stringify(before), JSON.stringify(after), actor]); await journal(db, { id, module: 'purchase_bo', type: 'purchase_bo_list', entity: after.id, version: after.version, action, actor, project: after.project_id, at: after.updated_at }); }
export async function history(db: DB, project: string, id: string) { return (await db.query(`SELECT e.id,e.subject_id,e.action,e.reason,e.before_snapshot AS before,e.after_snapshot AS after,e.created_by_id,${timeSQL('e.created_at')} AS created_at FROM purchase_bo_events e JOIN purchase_bo_lists b ON b.id=e.subject_id WHERE b.project_id=$1 AND e.subject_id=$2 ORDER BY e.created_at DESC,e.id`, [project, id])).rows.map(row => ({ ...row, created_at: timestamp(row.created_at), created_by_id: row.created_by_id ?? '00000000-0000-0000-0000-000000000000' })); }
// Transaction-bound linkage for sourcing; only a unique active completed BO is authority.
export async function demandLink(db: DB, project: string, demand: string) { const rows = (await db.query("SELECT b.id AS bo_id,b.version AS bo_version,l.id AS line_id,l.demand_id,l.item_id,l.quantity::text AS quantity FROM purchase_bo_lists b JOIN purchase_bo_lines l ON l.bo_id=b.id WHERE b.project_id=$1 AND b.is_active AND b.state='completed' AND l.is_active AND l.demand_id=$2 ORDER BY b.id,l.id LIMIT 2", [project, demand])).rows; if (!rows.length)
    return null; if (rows.length !== 1)
    throw new PurchaseError(422, 'Demand links to multiple active completed BO lines; reconcile source before sourcing', 'source'); if (!rows[0].demand_id || !rows[0].item_id)
    throw new PurchaseError(422, 'Completed BO line lacks published item/demand evidence', 'source'); return rows[0] as {
    bo_id: string;
    bo_version: number;
    line_id: string;
    demand_id: string;
    item_id: string;
    quantity: string;
}; }
