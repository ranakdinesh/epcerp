import type { DB } from '../../db';
import { timestamp, timeSQL } from '../../inbox/repository';
import type { Snapshot } from '../core/catalogue';
export type Allocation = {
    location: string;
    quantity: string;
};
export type Demand = {
    id: string;
    project_id: string;
    item_id: string;
    engineering_key: string;
    item_snapshot: Snapshot;
    quantity: string;
    required_date: string;
    allocations: Allocation[];
    alternatives: string[];
    is_active: boolean;
    version: number;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
};
const select = `id,project_id,item_id,engineering_key,item_snapshot,quantity::text,required_date::text,allocations,alternatives,is_active,version,${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at,created_by_id,updated_by_id`;
const view = (r: Demand): Demand => ({ ...r, created_at: timestamp(r.created_at)!, updated_at: timestamp(r.updated_at)! });
export async function demands(db: DB, project: string) { return (await db.query(`SELECT ${select} FROM project_material_requirements WHERE project_id=$1 ORDER BY created_at,id`, [project])).rows.map(view); }
export async function demand(db: DB, project: string, id: string) { const r = (await db.query(`SELECT ${select} FROM project_material_requirements WHERE project_id=$1 AND id=$2`, [project, id])).rows[0]; return r ? view(r) : null; }
export async function save(db: DB, record: Pick<Demand, 'id' | 'project_id' | 'item_id' | 'engineering_key' | 'item_snapshot' | 'quantity' | 'required_date' | 'allocations' | 'alternatives' | 'is_active' | 'version'>, actor: string) {
    const r = record.version === 1 ? await db.query(`INSERT INTO project_material_requirements(id,project_id,item_id,engineering_key,item_snapshot,quantity,required_date,allocations,alternatives,is_active,version,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6::numeric,$7::date,$8,$9,$10,1,$11,$11) RETURNING id`, [record.id, record.project_id, record.item_id, record.engineering_key, JSON.stringify(record.item_snapshot), record.quantity, record.required_date, JSON.stringify(record.allocations), JSON.stringify(record.alternatives), record.is_active, actor])
        : await db.query(`UPDATE project_material_requirements SET quantity=$3::numeric,required_date=$4::date,allocations=$5,alternatives=$6,is_active=$7,version=version+1,updated_at=clock_timestamp(),updated_by_id=$8 WHERE project_id=$1 AND id=$2 AND version=$9 RETURNING id`, [record.project_id, record.id, record.quantity, record.required_date, JSON.stringify(record.allocations), JSON.stringify(record.alternatives), record.is_active, actor, record.version - 1]);
    return r.rowCount ? demand(db, record.project_id, record.id) : null;
}
