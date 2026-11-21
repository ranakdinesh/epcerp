import 'server-only';
import { readStageEvidence, saveStageEvidence, type StageUpload } from '../core/stage-evidence';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import type { Actor } from '../core/access';
import { now } from '../core/time';
import { object, string, version, invalid } from '../core/validation';
import { id } from '../sourcing/values';
import { accessGuard, missing, stale, can, validate, apply, cleared } from './policy';
import type { Command, Clearance } from './types';
import * as context from './context';
import { store } from './repository';
export function input(raw: Record<string, unknown>): Command { const r = object(raw, ['version', 'action', 'reason', 'clearance']), c: Command = { version: version(r.version ?? 0), action: string(r.action, 'action', ''), reason: string(r.reason, 'reason', ''), clearance: null }; if (r.clearance != null) {
    const fields = ['status', 'mdcc_ref', 'clearance_date', 'cleared_by_authority', 'remarks', 'file_name'], q = object(r.clearance, fields), out: Record<string, string> = {};
    for (const k of fields)
        out[k] = string(q[k], k, '');
    c.clearance = out as Clearance;
} return validate(c); }
export async function workspaceBound(db: DB, a: Actor, p: string, id: string) { const caps = await context.access(db, a, p, id); accessGuard(a, caps, false); const source = await context.source(db, p, id), record = await store.get(db, p, id); if (record && !record.is_active)
    throw missing(); const actions = []; for (const action of ['record', 'revise'])
    try {
        can(record, a, caps, action, source);
        actions.push(action);
    }
    catch { } return { record, source, actions, history: record && caps.scoped ? (await store.history(db, record.id)).map(({ subject_id, ...rest }) => ({ ...rest, clearance_id: subject_id })) : [], task_only: !caps.scoped }; }
export async function workspace(token: string, p: string, record: string) { return transaction(async (db) => { const a = await context.actor(db, token); id(p); id(record); return workspaceBound(db, a, p, record); }); }
export async function execute(token: string, p: string, record: string, raw: Record<string, unknown>, evidence?: StageUpload) { return transaction(async (db) => { const a = await context.actor(db, token); id(p); id(record); const q = input(raw), caps = await context.access(db, a, p, record); accessGuard(a, caps, true); const before = await store.get(db, p, record); if (q.version !== (before?.version ?? 0))
    throw stale(); const source = await context.source(db, p, record);
    if (q.clearance?.status === 'cleared' && !evidence)
        invalid('file', 'Upload the client-issued clearance certificate before marking cleared');
    if (evidence && q.clearance?.status !== 'cleared')
        invalid('file', 'Attach the issued certificate when recording client clearance');
    if (evidence && q.clearance) q.clearance.file_name = evidence.name;
    const next = apply(before, a, caps, q, source, now(), randomUUID());
    if (evidence) next.clearance_evidence = await saveStageEvidence(p, record, 'client-clearance', evidence);
    else delete next.clearance_evidence;
    const saved = await store.save(db, next); await store.event(db, before, saved, q.action, q.reason, a.id); return workspaceBound(db, a, p, record); }); }
export async function clearedFact(db: DB, p: string, id: string) { return cleared(await store.get(db, p, id), await context.source(db, p, id)); }

export async function downloadEvidence(token: string, p: string, pkg: string) {
    return transaction(async db => {
        const a = await context.actor(db, token);
        id(p); id(pkg);
        accessGuard(a, await context.access(db, a, p, pkg), false);
        const r = await store.get(db, p, pkg);
        if (!r?.is_active || !r.clearance_evidence) throw missing();
        return readStageEvidence(p, pkg, 'client-clearance', r.clearance_evidence);
    });
}
