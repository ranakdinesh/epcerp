import 'server-only';
import { transaction, type DB } from '../../db';
import { purchaseTaskCatalogue } from '../../master/items/task-reader';
import { actor, project, scope } from '../core/access';
import { workspace } from '../workspace/repository';
import { catalogue as item, permitted } from '../core/catalogue';
import { object, string, version, uuid, invalid, PurchaseError } from '../core/validation';
import { timeSQL, timestamp } from '../../inbox/repository';
import { publish } from './writer';
import * as repo from './repository';
async function access(db: DB, token: string, p: string, write = false) { const code = 'projects.items.' + (write ? 'write' : 'read'), a = await actor(db, token, [code]), parent = await project(db, p, true); await scope(db, a, parent, await workspace(db, parent.id), code, write); return { a, p: parent.id }; }
export async function list(token: string, p: string) { return transaction(async (db) => { const c = await access(db, token, p); return repo.demands(db, c.p); }); }
export async function history(token: string, p: string, id: string) { return transaction(async (db) => { const c = await access(db, token, p); id = uuid(id); if (!await repo.demand(db, c.p, id))
    throw new PurchaseError(404, 'Project material requirement not found'); return (await db.query(`SELECT id,subject_id,action,reason,before_snapshot AS before,after_snapshot AS after,created_by_id,${timeSQL('created_at')} AS created_at FROM project_material_events WHERE subject_id=$1 ORDER BY created_at DESC,id`, [id])).rows.map(r => ({ ...r, created_at: timestamp(r.created_at)! })); }); }
export async function catalogue(token: string, p: string) { return transaction(async (db) => { const c = await access(db, token, p), out = []; for (const r of await purchaseTaskCatalogue(db)) {
    if (!r.is_active || r.status !== 'approved')
        continue;
    let s;
    try {
        s = await permitted(db, c.p, await item(db, r.id));
    }
    catch (e) {
        if (e instanceof PurchaseError && [404, 422].includes(e.status))
            continue;
        throw e;
    }
    out.push({ id: r.id, sku: r.sku, description: r.description, engineering_key: r.engineering_key, uom: s.uom, manufacturer: s.manufacturer, category: s.category_code, configuration: s.family_code, model: r.model });
} return out; }); }
export async function save(token: string, p: string, id: string | undefined, raw: Record<string, unknown>) { const r = object(raw, ['item_id', 'quantity', 'required_date', 'allocations', 'alternatives', 'is_active', 'version', 'reason']); if (r.is_active === undefined)
    r.is_active = true; if (typeof r.is_active !== 'boolean')
    invalid('is_active', 'Use true or false'); if (r.allocations != null && !Array.isArray(r.allocations) || r.alternatives != null && !Array.isArray(r.alternatives))
    invalid('allocations', 'Use arrays for allocations and alternatives'); const allocations = (r.allocations as unknown[] ?? []).map(v => { const a = object(v, ['location', 'quantity']); return { location: string(a.location, 'location', ''), quantity: string(a.quantity, 'quantity', '') }; }), alternatives = (r.alternatives as unknown[] ?? []).map(v => uuid(v)); return transaction(async (db) => { const c = await access(db, token, p, true); return publish(db, c.p, id ? uuid(id) : undefined, { item_id: string(r.item_id, 'item_id', ''), quantity: string(r.quantity, 'quantity', ''), required_date: string(r.required_date, 'required_date', ''), allocations, alternatives, is_active: r.is_active as boolean, version: version(r.version ?? 1, 'version', 1), reason: string(r.reason, 'reason', '') }, c.a.id); }); }
