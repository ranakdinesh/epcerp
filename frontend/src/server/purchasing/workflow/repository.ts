import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import { timeSQL, timestamp } from '../../inbox/repository';
import { journal } from '../core/journal';
import type { Package } from './types';
import { stale, duplicate, text } from './policy';
import { invalid } from '../core/validation';
const fields = `id,project_id,indent_id,demand_id,code,description,source_snapshot,version,is_active,created_by_id,updated_by_id,steps,${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at`;
const view = (r: Package): Package => ({ ...r, created_at: timestamp(r.created_at)!, updated_at: timestamp(r.updated_at)! });
export async function get(db: DB, p: string, id: string) { const row = (await db.query(`SELECT ${fields} FROM purchase_work_packages WHERE project_id=$1 AND id=$2`, [p, id])).rows[0]; return row ? view(row) : null; }
export async function exists(db: DB, p: string, indent: string, demand: string) { return !!(await db.query('SELECT id FROM purchase_work_packages WHERE project_id=$1 AND indent_id=$2 AND demand_id=$3', [p, indent, demand])).rowCount; }
export async function save(db: DB, r: Package, actor: string) { let result; try {
    result = r.version === 1 ? await db.query(`INSERT INTO purchase_work_packages(id,project_id,indent_id,demand_id,code,description,source_snapshot,steps,version,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,1,$9,$9) RETURNING ${fields}`, [r.id, r.project_id, r.indent_id, r.demand_id, r.code, r.description, JSON.stringify(r.source_snapshot), JSON.stringify(r.steps), actor]) : await db.query(`UPDATE purchase_work_packages SET steps=$3,version=version+1,updated_at=clock_timestamp(),updated_by_id=$4 WHERE project_id=$1 AND id=$2 AND version=$5 RETURNING ${fields}`, [r.project_id, r.id, JSON.stringify(r.steps), actor, r.version - 1]);
}
catch (e) {
    if ((e as {
        code?: string;
    }).code === '23505')
        throw duplicate();
    throw e;
} if (!result.rowCount)
    throw stale(); return view(result.rows[0]); }
export async function event(db: DB, before: Package | null, after: Package, action: string, reason: string, actor: string, at: string) { const id = randomUUID(); await db.query('INSERT INTO purchase_work_package_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8)', [id, after.id, action, reason, JSON.stringify(before), JSON.stringify(after), at, actor]); await journal(db, { id, module: 'purchase_workflow', type: 'purchase_package', entity: after.id, version: after.version, action, actor, project: after.project_id, at }); }
export async function history(db: DB, p: string, id: string) { return (await db.query(`SELECT e.id,e.action,e.reason,e.before_snapshot AS before,e.after_snapshot AS after,e.created_by_id,${timeSQL('e.created_at')} AS created_at FROM purchase_work_package_events e JOIN purchase_work_packages p ON p.id=e.subject_id WHERE p.project_id=$1 AND e.subject_id=$2 ORDER BY e.created_at DESC,e.id LIMIT 200`, [p, id])).rows.map(r => ({ ...r, created_at: timestamp(r.created_at)! })); }
export type Filter = {
    page?: number;
    page_size?: number;
    search?: string;
    sort?: string;
    direction?: string;
};
export function filter(raw: Filter) { const page = raw.page || 1, size = raw.page_size || 20, sort = raw.sort || 'code', direction = raw.direction || 'asc'; if (!Number.isInteger(page) || page < 1 || page > 100000 || ![20, 50, 100].includes(size))
    invalid('page', 'Use a valid page and 20/50/100 rows'); if (!['code', 'description', 'updated'].includes(sort) || !['asc', 'desc'].includes(direction))
    invalid('sort', 'Use a supported sort field and direction'); return { page, size, sort, direction, search: text('search', raw.search ?? '', 120, false) }; }
export async function page(db: DB, p: string, f: ReturnType<typeof filter>) { const pattern = '%' + f.search.replace(/[\\%_]/g, c => '\\' + c) + '%', total = (await db.query('SELECT count(*)::text AS total FROM purchase_work_packages WHERE project_id=$1 AND is_active AND(code ILIKE $2 OR description ILIKE $2)', [p, pattern])).rows[0].total, sort = { code: 'p.code', description: 'p.description', updated: 'p.updated_at' }[f.sort]; if (!sort || !['asc', 'desc'].includes(f.direction))
    throw new Error('Unvalidated purchase sorting'); const rows = (await db.query(`SELECT p.id,p.code,p.description,coalesce(p.source_snapshot->>'quantity','') AS quantity,coalesce(p.source_snapshot->>'uom','') AS uom,p.version,${timeSQL('p.updated_at')} AS updated_at,coalesce(s.data->'decision_snapshot'->'offer'->'supplier_snapshot'->>'name','') AS supplier,coalesce(s.state,'not_started') AS loi_state,coalesce(t.state,'not_started') AS technical_state,coalesce(o.state,'not_started') AS order_state,coalesce(o.data->'client_approval'->>'status','not_submitted') AS client_po_state FROM purchase_work_packages p LEFT JOIN purchase_sourcing_cases s ON s.package_id=p.id AND s.project_id=p.project_id AND s.is_active LEFT JOIN purchase_technical_cases t ON t.package_id=p.id AND t.project_id=p.project_id AND t.is_active LEFT JOIN purchase_orders o ON o.package_id=p.id AND o.project_id=p.project_id AND o.is_active WHERE p.project_id=$1 AND p.is_active AND(p.code ILIKE $2 OR p.description ILIKE $2)ORDER BY ${sort} ${f.direction},p.id LIMIT $3 OFFSET $4`, [p, pattern, f.size, (f.page - 1) * f.size])).rows.map(r => ({ ...r, updated_at: timestamp(r.updated_at)! })); return { rows, total: Number(total) }; }
