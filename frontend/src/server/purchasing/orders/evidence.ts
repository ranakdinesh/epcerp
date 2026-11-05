import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { seal, open, putObject, readStoredObject, documentBucket } from '../../master/documents/files';
import { timeSQL, timestamp } from '../../inbox/repository';
import { inspectTechnical } from '../core/evidence-inspector';
import { journal } from '../core/journal';
import { now } from '../core/time';
import { invalid } from '../core/validation';
import { id } from '../sourcing/values';
import type { Order } from './types';
import { accessGuard, missing, stale, guard, sameSource } from './policy';
import * as context from './context';
import * as repo from './repository';
export type File = {
    id: string;
    order_id: string;
    draft_revision: number;
    name: string;
    media_type: string;
    size: number;
    sha256: string;
    bucket: string;
    object_key: string;
    created_at: string;
    created_by_id: string;
};
export const fileView = (f: File) => ({ id: f.id, order_id: f.order_id, draft_revision: f.draft_revision, name: f.name, media_type: f.media_type, size: f.size, created_at: f.created_at, created_by_id: f.created_by_id });
export async function file(db: DB, order: string, id: string): Promise<File | null> { const r = (await db.query(`SELECT id,order_id,draft_revision,name,media_type,byte_size AS size,sha256,object_bucket AS bucket,object_key,${timeSQL('created_at')} AS created_at,created_by_id FROM purchase_order_client_files WHERE order_id=$1 AND id=$2 AND is_active`, [order, id])).rows[0]; return r ? { ...r, size: Number(r.size), created_at: timestamp(r.created_at)!, created_by_id: r.created_by_id ?? '00000000-0000-0000-0000-000000000000' } : null; }
async function event(db: DB, p: string, order: string, file: string, action: string, actor: string, at: string) { const id = randomUUID(); await db.query('INSERT INTO purchase_order_client_file_events(id,file_id,order_id,action,created_at,created_by_id)VALUES($1,$2,$3,$4,$5,$6)', [id, file, order, action, at, actor]); await journal(db, { id, module: 'purchase_orders', type: 'purchase_order_client_file', entity: file, version: Number((await db.query('SELECT count(*)::text AS n FROM purchase_order_client_file_events WHERE file_id=$1', [file])).rows[0].n), action, actor, project: p, at }); }
export async function validateFile(db: DB, o: Order | null, id: string) { if (!o)
    throw missing(); const f = await file(db, o.id, id); if (!f || f.order_id !== o.id || f.draft_revision !== o.draft_revision)
    invalid('file_id', 'Use signed PO evidence uploaded for this exact draft revision'); const data = open(await readStoredObject(f.bucket, f.object_key), f.sha256); if (data.length !== f.size)
    invalid('file_id', 'Signed file integrity check failed'); await inspectTechnical(f.name, f.media_type, data); }
async function scope(db: DB, token: string, p: string, record: string, version: number) { const a = await context.actor(db, token); id(p); id(record); accessGuard(a, await context.access(db, a, p, record), 'client_decision'); const o = await repo.get(db, p, record); if (!o?.is_active)
    throw missing(); if (o.version !== version)
    throw stale(); if (o.state !== 'draft' || o.client_approval.status !== 'submitted')
    invalid('state', 'Submit the draft to the client before attaching their signed response'); const source = await context.source(db, p, record); guard(source); if (!sameSource(o.source_snapshot, source))
    throw stale(); return { a, o }; }
export async function authorizeUpload(token: string, p: string, record: string, version: number) { await transaction(db => scope(db, token, p, record, version)); }
export async function upload(token: string, p: string, record: string, version: number, name: string, mime: string, data: Buffer) { await authorizeUpload(token, p, record, version); await inspectTechnical(name, mime, data); return transaction(async (db) => { const { a, o } = await scope(db, token, p, record, version), encrypted = seal(data), f: File = { id: randomUUID(), order_id: o.id, draft_revision: o.draft_revision, name, media_type: mime, size: data.length, sha256: encrypted.sha256, bucket: documentBucket(), object_key: '', created_at: now(), created_by_id: a.id }; f.object_key = 'purchase-orders/' + p + '/' + o.id + '/' + f.id; await putObject(f.bucket, f.object_key, encrypted.encrypted); await db.query('INSERT INTO purchase_order_client_files(id,order_id,draft_revision,name,media_type,byte_size,sha256,object_bucket,object_key,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,true,$10,$10,$11,$11)', [f.id, o.id, f.draft_revision, name, mime, f.size, f.sha256, f.bucket, f.object_key, f.created_at, a.id]); await event(db, p, o.id, f.id, 'uploaded', a.id, f.created_at); return fileView(f); }); }
export async function download(token: string, p: string, record: string, fileID: string) { return transaction(async (db) => { const a = await context.actor(db, token); id(p); id(record); id(fileID); accessGuard(a, await context.access(db, a, p, record), 'read'); const o = await repo.get(db, p, record); if (!o)
    throw missing(); const f = await file(db, o.id, fileID); if (!f)
    throw missing(); const data = open(await readStoredObject(f.bucket, f.object_key), f.sha256); await event(db, p, o.id, fileID, 'downloaded', a.id, now()); return { file: fileView(f), data }; }); }
