import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { timeSQL, timestamp } from '../../inbox/repository';
import { purchaseRuleScope } from '../../master/categories/purchase-reader';
import { ManufacturerRepository } from '../../master/manufacturers/repository';
import { PurchaseError, uuid, invalid } from '../core/validation';
import { journal } from '../core/journal';
import { input } from './validation';
import type { Record as Rule } from './types';
const missing = () => new PurchaseError(404, 'Client purchase rule not found'), stale = () => new PurchaseError(409, 'Client purchase rule changed; reload before saving');
const fields = `id,client_id,data,is_active,version,created_by_id,updated_by_id,${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at`;
const view = (r: Rule): Rule => ({ ...r, created_at: timestamp(r.created_at)!, updated_at: timestamp(r.updated_at)! });
async function get(db: DB, c: string, id: string) { const r = (await db.query(`SELECT ${fields} FROM client_purchase_rules WHERE client_id=$1 AND id=$2`, [c, id])).rows[0]; return r ? view(r) : null; }
export async function list(token: string, client: string) { return transaction(async (db) => { await requireAnyPermission(db, token, ['settings.purchase_rules.read'], true); client = uuid(client); return (await db.query(`SELECT ${fields} FROM client_purchase_rules WHERE client_id=$1 ORDER BY created_at,id`, [client])).rows.map(view); }); }
export async function history(token: string, client: string, id: string) { return transaction(async (db) => { await requireAnyPermission(db, token, ['settings.purchase_rules.read'], true); client = uuid(client); id = uuid(id); if (!await get(db, client, id))
    throw missing(); return (await db.query(`SELECT e.id,e.subject_id,e.action,e.reason,e.before_snapshot AS before,e.after_snapshot AS after,e.created_by_id,${timeSQL('e.created_at')} AS created_at FROM client_purchase_rule_events e WHERE e.subject_id=$1 ORDER BY e.created_at DESC,e.id`, [id])).rows.map(r => ({ ...r, created_at: timestamp(r.created_at)! })); }); }
export async function save(token: string, client: string, id: string | undefined, raw: Record<string, unknown>) { return transaction(async (db) => { const a = await requireAnyPermission(db, token, ['settings.purchase_rules.write'], true); client = uuid(client); if (id)
    id = uuid(id); const q = input(raw), d = q.data; await db.query('SELECT pg_advisory_xact_lock(91827148)'); if (!(await db.query('SELECT is_active FROM clients WHERE id=$1', [client])).rows[0]?.is_active)
    invalid('client_id', 'Select an active client'); if (d.project_id && (await db.query('SELECT client_id FROM projects WHERE id=$1', [d.project_id])).rows[0]?.client_id !== client)
    invalid('project_id', 'Project belongs to another client'); if (d.scope_code !== '*' && !await purchaseRuleScope(db, d.scope_code))
    invalid('scope_code', 'Unknown or inactive category/configuration scope'); const manufacturers = new ManufacturerRepository(db); for (const key of [...d.allowed, ...d.excluded])
    if (!(await manufacturers.get(key))?.is_active)
        invalid('allowed', 'Manufacturer is inactive or unavailable'); const before = id ? await get(db, client, id) : null; if (id && !before)
    throw missing(); if (q.version !== (before?.version ?? 1))
    throw stale(); const key = id ?? randomUUID(), values = [key, client, d.project_id, JSON.stringify(d), q.is_active, a.id]; const r = before ? await db.query(`UPDATE client_purchase_rules SET project_id=$3,data=$4,is_active=$5,updated_by_id=$6,version=version+1,updated_at=clock_timestamp() WHERE id=$1 AND client_id=$2 AND version=$7 RETURNING ${fields}`, [...values, q.version]) : await db.query(`INSERT INTO client_purchase_rules(id,client_id,project_id,data,is_active,created_by_id,updated_by_id,version)VALUES($1,$2,$3,$4,$5,$6,$6,1)RETURNING ${fields}`, values); if (!r.rowCount)
    throw stale(); const after = view(r.rows[0]), event = randomUUID(), action = before ? 'revise' : 'create'; await db.query('INSERT INTO client_purchase_rule_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7)', [event, key, action, q.reason, JSON.stringify(before), JSON.stringify(after), a.id]); await journal(db, { id: event, module: 'client_purchase_rules', type: 'client_purchase_rule', entity: key, version: after.version, action, actor: a.id, project: d.project_id, at: after.updated_at }); return after; }); }
