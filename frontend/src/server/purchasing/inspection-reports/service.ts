import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import type { Actor } from '../core/access';
import { now } from '../core/time';
import { object, string, version } from '../core/validation';
import { id } from '../sourcing/values';
import { accessGuard, missing, stale, can, validate, apply, issued } from './policy';
import type { Command, Report } from './types';
import * as context from './context';
import { store } from './repository';
export function input(raw: Record<string, unknown>): Command { const r = object(raw, ['version', 'action', 'reason', 'report']), c: Command = { version: version(r.version ?? 0), action: string(r.action, 'action', ''), reason: string(r.reason, 'reason', ''), report: null }; if (r.report != null) {
    const fields = ['status', 'ir_number', 'irn_number', 'issue_date', 'issuing_authority', 'cleared_quantity', 'file_name'], q = object(r.report, fields), out: Record<string, string> = {};
    for (const k of fields)
        out[k] = string(q[k], k, '');
    c.report = out as Omit<Report, 'uom'>;
} return validate(c); }
export async function workspaceBound(db: DB, a: Actor, p: string, id: string) { const caps = await context.access(db, a, p, id); accessGuard(a, caps, false); const source = await context.source(db, p, id), record = await store.get(db, p, id); if (record && !record.is_active)
    throw missing(); const actions = []; for (const action of ['record', 'revise'])
    try {
        can(record, a, caps, action, source);
        actions.push(action);
    }
    catch { } return { record, source, actions, history: record && caps.scoped ? (await store.history(db, record.id)).map(({ subject_id, ...rest }) => ({ ...rest, report_id: subject_id })) : [], task_only: !caps.scoped }; }
export async function workspace(token: string, p: string, record: string) { return transaction(async (db) => { const a = await context.actor(db, token); id(p); id(record); return workspaceBound(db, a, p, record); }); }
export async function execute(token: string, p: string, record: string, raw: Record<string, unknown>) { return transaction(async (db) => { const a = await context.actor(db, token); id(p); id(record); const q = input(raw), caps = await context.access(db, a, p, record); accessGuard(a, caps, true); const before = await store.get(db, p, record); if (q.version !== (before?.version ?? 0))
    throw stale(); const source = await context.source(db, p, record), next = apply(before, a, caps, q, source, now(), randomUUID()), saved = await store.save(db, next); await store.event(db, before, saved, q.action, q.reason, a.id); return workspaceBound(db, a, p, record); }); }
export async function issuedFact(db: DB, p: string, id: string) { return issued(await store.get(db, p, id), await context.source(db, p, id)); }
