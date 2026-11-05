import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import type { Actor } from '../core/access';
import { now } from '../core/time';
import { object, string, version, invalid } from '../core/validation';
import { id } from '../sourcing/values';
import { accessGuard, missing, stale, can, validate, apply, cleared } from './policy';
import type { Command, InspectionInput } from './types';
import * as context from './context';
import { store } from './repository';
const bool = (v: unknown) => { if (v == null)
    return null; if (typeof v !== 'boolean')
    invalid('witness_points', 'Declare mandatory and cleared flags explicitly'); return v; };
export function input(raw: Record<string, unknown>): Command { const r = object(raw, ['version', 'action', 'reason', 'inspection']), c: Command = { version: version(r.version ?? 0), action: string(r.action, 'action', ''), reason: string(r.reason, 'reason', ''), inspection: null }; if (r.inspection != null) {
    const fields = ['status', 'call_notice_ref', 'call_date', 'scheduled_date', 'actual_date', 'inspection_agency', 'inspector_name', 'factory_location', 'no_witness_points_reason', 'findings'], q = object(r.inspection, [...fields, 'witness_points']), out: Record<string, unknown> = {};
    for (const k of fields)
        out[k] = string(q[k], k, '');
    if (q.witness_points != null && !Array.isArray(q.witness_points))
        invalid('witness_points', 'Keep an inspection checklist within 100 points');
    out.witness_points = ((q.witness_points ?? []) as unknown[]).map(p => { const r = object(p, ['code', 'title', 'is_witness', 'is_cleared', 'remarks']); return { code: string(r.code, 'code', ''), title: string(r.title, 'title', ''), is_witness: bool(r.is_witness), is_cleared: bool(r.is_cleared), remarks: string(r.remarks, 'remarks', '') }; });
    c.inspection = out as InspectionInput;
} return validate(c); }
export async function workspaceBound(db: DB, a: Actor, p: string, id: string) { const caps = await context.access(db, a, p, id); accessGuard(a, caps, false); const source = await context.source(db, p, id), record = await store.get(db, p, id); if (record && !record.is_active)
    throw missing(); const actions = []; for (const action of ['record', 'revise'])
    try {
        can(record, a, caps, action, source);
        actions.push(action);
    }
    catch { } return { record, source, actions, history: record && caps.scoped ? (await store.history(db, record.id)).map(({ subject_id, ...rest }) => ({ ...rest, inspection_id: subject_id })) : [], task_only: !caps.scoped }; }
export async function workspace(token: string, p: string, record: string) { return transaction(async (db) => { const a = await context.actor(db, token); id(p); id(record); return workspaceBound(db, a, p, record); }); }
export async function execute(token: string, p: string, record: string, raw: Record<string, unknown>) { return transaction(async (db) => { const a = await context.actor(db, token); id(p); id(record); const q = input(raw), caps = await context.access(db, a, p, record); accessGuard(a, caps, true); const before = await store.get(db, p, record); if (q.version !== (before?.version ?? 0))
    throw stale(); const source = await context.source(db, p, record), next = apply(before, a, caps, q, source, now(), randomUUID()), saved = await store.save(db, next); await store.event(db, before, saved, q.action, q.reason, a.id); return workspaceBound(db, a, p, record); }); }
export async function clearedFact(db: DB, p: string, id: string) { return cleared(await store.get(db, p, id), await context.source(db, p, id)); }
