import 'server-only';
import type { DB } from '../../db';
import { assignmentIdentity } from '../../auth/access';
import { demand } from '../materials/repository';
import { purchaseTaskItem } from '../../master/items/task-reader';
import { policy } from '../core/catalogue';
import { invalid, PurchaseError } from '../core/validation';
import { holdEvidence } from '../holds/evidence';
import type { Input, Source } from './types';
export async function identities(db: DB, data: Input) {
    if (data.doer_id === data.resolver_id)
        invalid('resolver_id', 'Procurement doer and problem resolver must be different people');
    for (const id of [data.doer_id, data.resolver_id])
        if (!(await assignmentIdentity(db, id))?.active)
            invalid('doer_id', 'Assign active doer and resolver identities');
}
export async function lineSource(db: DB, project: string, id: string): Promise<Source> {
    const d = await demand(db, project, id), p = (await db.query('SELECT client_id FROM projects WHERE id=$1', [project])).rows[0];
    if (!d || !p)
        throw new PurchaseError(404, 'Purchase indent not found');
    const client = (await db.query('SELECT is_active FROM clients WHERE id=$1', [p.client_id])).rows[0], item = await purchaseTaskItem(db, d.item_id), config = item.snapshot.configuration;
    let active = d.is_active && !!client?.is_active && item.approved_for_purchase;
    const sourceItem = { ...d.item_snapshot, manufacturer_id: item.manufacturer_id, category_code: config.category_code, family_code: config.family_code, configuration_code: config.code };
    const verdict = await policy(db, project, sourceItem);
    for (const alt of d.alternatives) {
        const other = await purchaseTaskItem(db, alt);
        active = active && other.approved_for_purchase;
        const result = await policy(db, project, { ...sourceItem, manufacturer_id: other.manufacturer_id, configuration_code: other.snapshot.configuration.code });
        if (result.status !== 'eligible') {
            verdict.status = 'blocked';
            verdict.reasons.push('An alternative manufacturer is not client-permitted');
        }
    }
    return { active, quantity: d.quantity, demand_version: d.version, item: d.item_snapshot, required_date: d.required_date, allocations: d.allocations, client_id: p.client_id, policy: { status: verdict.status, reasons: verdict.reasons, rules: verdict.rules.map(r => ({ id: r.id, version: r.version })) }, holds: await holdEvidence(db, project, id, config.category_code) };
}
