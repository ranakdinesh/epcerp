import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction } from '../../db';
import { seal, open, putObject, readStoredObject, documentBucket } from '../../master/documents/files';
import { object, string, isoDate, invalid } from '../core/validation';
import { text, missing } from '../workflow/policy';
import { id } from '../approvals/validation';
import { now } from '../core/time';
import { indiaDate } from '../../personal-tasks/validation';
import { authorize, access, candidates } from '../awards/context';
import { get as handoff } from '../awards/repository';
import { fingerprint } from '../approvals/fingerprint';
import { content, digest } from './content';
import { render } from './pdf';
import * as repo from './repository';
import type { Command, Document } from './types';
export function input(raw: Record<string, unknown>): Command { const r = object(raw, ['reference', 'date', 'format', 'notes']), q = { reference: text('reference', string(r.reference, 'reference', ''), 120), date: string(r.date, 'date', ''), format: string(r.format, 'format', ''), notes: text('notes', string(r.notes, 'notes', ''), 12000, false) }; if (!isoDate(q.date) || q.date > indiaDate(new Date()))
    invalid('date', 'Use a valid LOI date that is not in the future'); if (!['general', 'cable', 'panel', 'pole'].includes(q.format))
    invalid('format', 'Choose an LOI format'); return q; }
export async function workspace(token: string, p: string, attempt: string) { return transaction(async (db) => { id(p); id(attempt); const a = await authorize(db, token, p, false), h = await handoff(db, p, attempt); if (!h)
    invalid('award', 'Prepare the approved award before generating its LOI'); let can_generate = true; try {
    await access(db, a, p, true);
}
catch {
    can_generate = false;
} return { default_reference: 'LOI/VTPL/' + h.id.slice(0, 8), today: indiaDate(new Date()), can_generate, documents: (await repo.list(db, p, attempt)).map(repo.view) }; }); }
export async function generate(token: string, p: string, attempt: string, raw: Record<string, unknown>) { const q = input(raw); return transaction(async (db) => { id(p); id(attempt); const a = await authorize(db, token, p, true); await db.query("SELECT pg_advisory_xact_lock(hashtextextended('purchase-award:'||$1::text,0))", [p]); const h = await handoff(db, p, attempt); if (!h)
    invalid('award', 'Prepare the approved award before generating its LOI'); const snapshot = await content(db, p, attempt, q), hash = digest(snapshot), previous = await repo.list(db, p, attempt), existing = previous.find(d => d.request_hash === hash); if (existing)
    return repo.view(existing); const version = previous.reduce((v, d) => Math.max(v, d.version + 1), 1), data = await render(snapshot, version), encrypted = seal(data), key = randomUUID(), bucket = documentBucket(), objectKey = 'vendor-lois/' + p + '/' + h.id + '/' + key; await putObject(bucket, objectKey, encrypted.encrypted); const d: Document = { id: key, project_id: p, handoff_id: h.id, attempt_id: attempt, ...q, version, created_at: now(), created_by_id: a.id, request_hash: hash, sha256: encrypted.sha256, bucket, object_key: objectKey, size: data.length, content: snapshot, review: null }; await repo.save(db, d); await repo.event(db, d, 'generated', a.id); return repo.view(d); }); }
export async function download(token: string, p: string, attempt: string, file: string) { return transaction(async (db) => { id(p); id(attempt); id(file); const a = await authorize(db, token, p, false), d = (await repo.list(db, p, attempt)).find(d => d.id === file); if (!d)
    throw missing(); const data = open(await readStoredObject(d.bucket, d.object_key), d.sha256); if (data.length !== d.size)
    throw new Error('LOI file integrity check failed'); await repo.event(db, d, 'downloaded', a.id); return { data, name: 'LOI-' + d.id.slice(0, 8) + '-v' + d.version + '.pdf' }; }); }
export async function completeReview(token: string, p: string, attempt: string, file: string, raw: Record<string, unknown>) { const q = object(raw, ['details_correct', 'prices_correct', 'format_correct']); for (const [k, msg] of [['details_correct', 'Confirm the items, quantities and technical details'], ['prices_correct', 'Confirm the approved prices and commercial terms'], ['format_correct', 'Confirm the PDF format is correct']] as const)
    if (q[k] !== true)
        invalid(k, msg); return transaction(async (db) => { id(p); id(attempt); id(file); const a = await authorize(db, token, p, true); await db.query("SELECT pg_advisory_xact_lock(hashtextextended('purchase-award:'||$1::text,0))", [p]); const docs = await repo.list(db, p, attempt), d = docs[0]; if (!d || d.id !== file)
    invalid('document', 'A newer PDF is available. Review the latest version before completing'); const c = (await candidates(db, p)).find(c => c.attempt.id === attempt); if (!c || fingerprint(c.attempt) !== d.content.SourceHash)
    invalid('approval', 'The approved source changed. Refresh the award and generate its current LOI'); return d.review ?? await repo.completeReview(db, p, file, a.id); }); }
