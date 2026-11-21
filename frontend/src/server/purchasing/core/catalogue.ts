import 'server-only';
import type { DB } from '../../db';
import { purchaseTaskItem } from '../../master/items/task-reader';
import { indiaDate } from '../../personal-tasks/validation';
import { PurchaseError, invalid } from './validation';
export type Snapshot = {
    id: string;
    sku: string;
    description: string;
    engineering_key: string;
    uom: string;
    manufacturer: string;
    manufacturer_id: string;
    category_code: string;
    family_code: string;
    configuration_code: string;
};
export async function catalogue(db: DB, id: string): Promise<Snapshot> {
    const item = await purchaseTaskItem(db, id);
    if (!item.approved_for_purchase)
        invalid('item_id', 'Select an active approved catalogue item with current configuration and manufacturer eligibility');
    const config = item.snapshot.configuration;
    return { id: item.id, sku: item.sku, description: item.description, engineering_key: item.engineering_key, uom: config.uom, manufacturer: item.snapshot.manufacturer.name, manufacturer_id: item.manufacturer_id, category_code: config.category_code, family_code: config.family_code, configuration_code: config.code };
}
export async function policy(db: DB, project: string, item: Snapshot) {
    const p = (await db.query('SELECT client_id FROM projects WHERE id=$1', [project])).rows[0];
    if (!p)
        throw new PurchaseError(404, 'Project material requirement not found');
    await db.query('SELECT pg_advisory_xact_lock(91827148)');
    const rows = (await db.query('SELECT id,data,version,is_active FROM client_purchase_rules WHERE client_id=$1 ORDER BY created_at,id', [p.client_id])).rows;
    const reasons: string[] = [], rules: {
        id: string;
        version: number;
        evidence: string;
    }[] = [], today = indiaDate(new Date());
    for (const row of rows) {
        const d = row.data;
        if (!row.is_active || d.project_id && d.project_id !== project || d.effective_from > today || d.effective_until && d.effective_until < today || !['*', item.category_code, item.family_code, item.configuration_code].includes(d.scope_code))
            continue;
        rules.push({ id: row.id, version: row.version, evidence: d.evidence });
        if (d.excluded.includes(item.manufacturer_id))
            reasons.push('Manufacturer excluded by ' + d.scope_code + ' policy');
        if (d.mode === 'approved_only' && !d.allowed.includes(item.manufacturer_id))
            reasons.push('Manufacturer outside approved list for ' + d.scope_code);
    }
    return { status: reasons.length ? 'blocked' : 'eligible', reasons, rules };
}
export async function permitted(db: DB, project: string, item: Snapshot) {
    const verdict = await policy(db, project, item);
    if (verdict.status === 'blocked')
        invalid('item_id', 'Client manufacturer restriction: ' + verdict.reasons.join('; '));
    return item;
}
