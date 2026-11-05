import type { DB } from '../../db';
import { timeSQL, timestamp } from '../../inbox/repository';
import { randomUUID } from 'node:crypto';
import { journal } from './journal';
import { PurchaseError } from './validation';
export type Entity<D> = {
    id: string;
    project_id: string;
    data: D;
    version: number;
    is_active: boolean;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
};
type Config<D> = {
    table: string;
    events: string;
    module: string;
    type: string;
    stale: string;
    columns?: (data: D) => Record<string, string>;
};
// Configurations are static server code. Identifiers are checked before interpolation.
export function jsonStore<D>(config: Config<D>) {
    for (const identifier of [config.table, config.events])
        if (!/^purchase_[a-z_]+$/.test(identifier))
            throw new Error('Invalid purchase store identifier');
    const fields = `id,project_id,data,version,is_active,${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at,created_by_id,updated_by_id`;
    const view = (row: Entity<D>): Entity<D> => ({ ...row, created_at: timestamp(row.created_at)!, updated_at: timestamp(row.updated_at)! });
    const stale = () => new PurchaseError(409, config.stale);
    return {
        async list(db: DB, project: string) { return (await db.query(`SELECT ${fields} FROM ${config.table} WHERE project_id=$1 ORDER BY created_at,id`, [project])).rows.map(view); },
        async get(db: DB, project: string, id: string) { const row = (await db.query(`SELECT ${fields} FROM ${config.table} WHERE project_id=$1 AND id=$2`, [project, id])).rows[0]; return row ? view(row) : null; },
        async save(db: DB, record: Pick<Entity<D>, 'id' | 'project_id' | 'data' | 'version' | 'is_active'>, actor: string) {
            const extra = Object.entries(config.columns?.(record.data) ?? {});
            for (const [key] of extra)
                if (!/^[a-z_]+$/.test(key))
                    throw new Error('Invalid purchase source column');
            const values: unknown[] = [record.id, record.project_id, JSON.stringify(record.data), actor, ...extra.map(([, value]) => value)];
            let result;
            if (record.version === 1) {
                result = await db.query(`INSERT INTO ${config.table}(id,project_id,data,is_active,version,created_by_id,updated_by_id${extra.map(([key]) => ',' + key).join('')})VALUES($1,$2,$3,true,1,$4,$4${extra.map((_, i) => ',$' + (i + 5)).join('')}) RETURNING ${fields}`, values);
            }
            else {
                values.push(record.is_active, record.version - 1);
                result = await db.query(`UPDATE ${config.table} SET data=$3,updated_by_id=$4${extra.map(([key], i) => ',' + key + '=$' + (i + 5)).join('')},is_active=$${values.length - 1},version=version+1,updated_at=clock_timestamp() WHERE id=$1 AND project_id=$2 AND version=$${values.length} RETURNING ${fields}`, values);
            }
            if (!result.rowCount)
                throw stale();
            return view(result.rows[0]);
        },
        async event(db: DB, before: Entity<D> | null, after: Entity<D>, action: string, reason: string, actor: string) { const id = randomUUID(); await db.query(`INSERT INTO ${config.events}(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7)`, [id, after.id, action, reason, JSON.stringify(before), JSON.stringify(after), actor]); await journal(db, { id, module: config.module, type: config.type, entity: after.id, version: after.version, action, actor, project: after.project_id, at: after.updated_at }); },
        async history(db: DB, project: string, id: string) { return (await db.query(`SELECT e.id,e.subject_id,e.action,e.reason,e.before_snapshot AS before,e.after_snapshot AS after,e.created_by_id,${timeSQL('e.created_at')} AS created_at FROM ${config.events} e JOIN ${config.table} r ON r.id=e.subject_id WHERE r.project_id=$1 AND r.id=$2 ORDER BY e.created_at DESC,e.id`, [project, id])).rows.map(row => ({ ...row, created_at: timestamp(row.created_at), created_by_id: row.created_by_id ?? '00000000-0000-0000-0000-000000000000' })); }, stale,
    };
}
