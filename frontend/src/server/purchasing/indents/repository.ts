import { invalid } from '../core/validation';
import type { DB } from '../../db';
import { jsonStore, type Entity } from '../core/json-store';
import type { Data } from './types';
export const store = jsonStore<Data>({ table: 'purchase_indents', events: 'purchase_indent_events', module: 'purchase_indents', type: 'purchase_indent', stale: 'Purchase indent or linked material requirement changed; reload the saved version', columns: d => ({ doer_id: d.doer_id, resolver_id: d.resolver_id }) });
export type Indent = Entity<Data>;
export async function save(db: DB, record: Parameters<typeof store.save>[1], actor: string) {
    const saved = await store.save(db, record, actor);
    for (const line of record.data.released) {
        const result = await db.query('INSERT INTO purchase_indent_releases(indent_id,demand_id,quantity,snapshot,created_by_id)VALUES($1,$2,$3::numeric,$4,$5)ON CONFLICT DO NOTHING', [record.id, line.demand_id, line.quantity, JSON.stringify(line), line.actor_id]);
        if (!result.rowCount) {
            const row = (await db.query('SELECT snapshot=$3::jsonb AS same FROM purchase_indent_releases WHERE indent_id=$1 AND demand_id=$2', [record.id, line.demand_id, JSON.stringify(line)])).rows[0];
            if (!row?.same)
                invalid('released', 'Released source evidence cannot be rewritten');
        }
    }
    return saved;
}
