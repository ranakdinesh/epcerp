import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import type { Actor } from '../core/access';
import { now } from '../core/time';
import { invalid } from '../core/validation';
import { id } from '../sourcing/values';
import { accessGuard, guard, missing, stale, sameSource, clientCleared, fiscalYear, number, apply } from './policy';
import { input } from './validation';
import { validateFile } from './evidence';
import * as repo from './repository';
import * as context from './context';
export async function workspaceBound(db: DB, a: Actor, p: string, id: string) { const caps = await context.access(db, a, p, id); accessGuard(a, caps, 'read'); const source = await context.source(db, p, id); if (source.project_id !== p || source.package_id !== id)
    invalid('source', 'Order source does not belong to this project/package'); const order = await repo.get(db, p, id); if (order && !order.is_active)
    throw missing(); const out = { order, source, actions: [] as string[], history: order && caps.scoped ? await repo.history(db, order.id) : [], task_only: !caps.scoped }; try {
    guard(source);
}
catch {
    return out;
} if (order?.state === 'released')
    return out; const can = (action: string) => { try {
    accessGuard(a, caps, action);
    return true;
}
catch {
    return false;
} }; if (can('prepare'))
    out.actions.push('prepare'); if (order && sameSource(order.source_snapshot, source)) {
    if (order.draft_revision > 0 && ['not_submitted', 'rejected', ''].includes(order.client_approval.status) && can('submit_client'))
        out.actions.push('submit_client');
    if (order.client_approval.status === 'submitted' && can('client_decision'))
        out.actions.push('client_decision');
    if (clientCleared(order) && can('release'))
        out.actions.push('release');
} return out; }
export async function workspace(token: string, p: string, record: string) { return transaction(async (db) => { const a = await context.actor(db, token); id(a.id); id(p); id(record); return workspaceBound(db, a, p, record); }); }
export async function execute(token: string, p: string, record: string, raw: Record<string, unknown>) { return transaction(async (db) => { const a = await context.actor(db, token); id(a.id); id(p); id(record); const q = input(raw), caps = await context.access(db, a, p, record); accessGuard(a, caps, q.action); const before = await repo.get(db, p, record); if (q.version !== (before?.version ?? 0))
    throw stale(); const source = await context.source(db, p, record); if (source.project_id !== p || source.package_id !== record)
    invalid('source', 'Order source does not belong to this project/package'); guard(source); let poNumber = ''; if (q.action === 'release') {
    if (!before)
        invalid('order', 'Prepare the complete order before release');
    if (!sameSource(before.source_snapshot, source))
        throw stale();
    poNumber = number(before.details.date, await repo.nextNumber(db, fiscalYear(before.details.date)));
} if (q.client_decision?.file_id)
    await validateFile(db, before, q.client_decision.file_id); const next = apply(before, a, caps, q, source, now(), randomUUID(), poNumber), saved = await repo.save(db, next); await repo.event(db, before, saved, q.action, q.reason, a.id); return workspaceBound(db, a, p, record); }); }
export async function document(token: string, p: string, record: string) { return transaction(async (db) => { const a = await context.actor(db, token); id(p); id(record); accessGuard(a, await context.access(db, a, p, record), 'read'); const o = await repo.get(db, p, record); if (!o?.is_active)
    throw missing(); await repo.documentEvent(db, o, a.id, now()); return o; }); }
