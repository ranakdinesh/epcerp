import 'server-only';
import { randomUUID, createHash, createCipheriv, createDecipheriv, randomBytes } from 'node:crypto';
import { S3Client, GetObjectCommand, PutObjectCommand, HeadBucketCommand, CreateBucketCommand } from '@aws-sdk/client-s3';
import { transaction, type DB } from '../db';
import { requireAnyPermission } from '../auth/access';
import { AppError } from '../errors';
import { bytes, json, binary } from '../purchasing/http-input';
import { timeSQL, timestamp } from '../inbox/repository';
import { lower } from '../tenders/intake/receipts';
const maximum = 50 * 1048576;
const fail = (s: string) => new AppError(400, s);
export const digest = (b: Buffer) => createHash('sha256').update(b).digest('hex');
function key() {
    const raw = process.env.VTPL_DOCUMENT_ENCRYPTION_KEY || ((process.env.VTPL_JWT_SECRET_KEY ?? '') + ':vtpl_secure_doc_salt');
    if (!process.env.VTPL_DOCUMENT_ENCRYPTION_KEY && (process.env.VTPL_JWT_SECRET_KEY ?? '').length < 32)
        throw new Error('Document key unavailable');
    return createHash('sha256').update(raw).digest();
}
export function encrypt(data: Buffer) {
    const n = randomBytes(12), c = createCipheriv('aes-256-gcm', key(), n);
    return Buffer.concat([n, c.update(data), c.final(), c.getAuthTag()]);
}
export function decrypt(data: Buffer) {
    try {
        const c = createDecipheriv('aes-256-gcm', key(), data.subarray(0, 12));
        c.setAuthTag(data.subarray(-16));
        return Buffer.concat([c.update(data.subarray(12, -16)), c.final()]);
    }
    catch {
        throw fail('Document encryption authentication failed');
    }
}
export function filename(s: string) {
    let v = [...s.trim().split('/').pop()!].map(c => /[\p{L}\p{N}_.()\-]/u.test(c) ? c : '_').join('');
    if (!v || v.startsWith('.'))
        v = 'document_' + v.replace(/^\.+/, '');
    return [...v].slice(0, 200).join('');
}
function id(s: string) {
    const v = s.replaceAll('-', '');
    if (!/^[a-f\d]{32}$/i.test(v))
        throw fail('Invalid UUID');
    return `${v.slice(0, 8)}-${v.slice(8, 12)}-${v.slice(12, 16)}-${v.slice(16, 20)}-${v.slice(20)}`.toLowerCase();
}
function storage() {
    return new S3Client({
        endpoint: `${['true', '1', 'yes', 'on'].includes(process.env.VTPL_STORAGE_SECURE ?? '') ? 'https' : 'http'}://${process.env.VTPL_STORAGE_ENDPOINT}`, region: 'us-east-1', forcePathStyle: true, maxAttempts: 2, credentials: { accessKeyId: process.env.VTPL_STORAGE_ACCESS_KEY ?? '', secretAccessKey: process.env.VTPL_STORAGE_SECRET_KEY ?? '' }, requestHandler: { connectionTimeout: 5000, requestTimeout: 30000 }
    });
}
const fields = `id,entity_type,entity_id,doc_type,title,original_filename,content_type,byte_size,storage_bucket,storage_key,sha256_hash,is_encrypted,encryption_algorithm,is_active,${timeSQL('created_at')} AS created_at,created_by_id,${timeSQL('updated_at')} AS updated_at,updated_by_id`;
const view = (r: Record<string, unknown>) => ({ ...r, created_at: timestamp(r.created_at as string), updated_at: timestamp(r.updated_at as string) });
async function event(db: DB, document: string, action: string, actor: string, metadata: Record<string, unknown>) {
    await db.query('INSERT INTO stored_document_events(id,document_id,action,actor_id,metadata_snapshot)VALUES($1,$2,$3,$4,$5)', [randomUUID(), document, action, actor, metadata]);
}
const types = new Set('aadhar pan gst cancelled_cheque msme_certificate vendor_profile bank_mandate po_signed_copy advance_request call_notice stage_inspection irn_note tc_test_certificate tr_test_report mtc_mill_test_certificate rtc_routine_test_certificate warranty_certificate mdcc_clearance transporter_lr eway_bill tax_invoice site_verification grn_voucher mbook_entry jv_settlement contract_evidence project_document technical_datasheet drawing other'.split(' '));
const media = new Set(['application/pdf', 'image/png', 'image/jpeg', 'image/webp', 'text/csv', 'text/plain', 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', 'application/vnd.ms-excel', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'application/zip', 'application/x-zip-compressed', 'application/octet-stream']);
export async function handleStorage(r: Request) {
    try {
        const u = new URL(r.url), p = u.pathname.replace(/^\/api\/v1/, '').split('/').filter(Boolean), token = r.headers.get('authorization')?.replace(/^Bearer /i, '') ?? '', write = r.method !== 'GET';
        return await transaction(async (db) => {
            const a = await requireAnyPermission(db, token, [write ? 'documents.storage.write' : 'documents.storage.read'], write);
            if (r.method === 'HEAD' && p[1] === 'upload')
                return new Response(null, { status: 200 });
            if (r.method === 'POST' && p[1] === 'upload') {
                const raw = await bytes(r, maximum + 65536);
                let form: FormData;
                try {
                    form = await new Request(r.url, { method: 'POST', headers: { 'Content-Type': r.headers.get('content-type') ?? '' }, body: new Uint8Array(raw) }).formData();
                }
                catch {
                    throw new AppError(422, 'Multipart form required');
                }
                const seen = new Set<string>();
                for (const [k, v] of form) {
                    if (seen.has(k) || !['file', 'entity_type', 'entity_id', 'doc_type', 'title'].includes(k))
                        throw new AppError(422, 'Unknown or repeated multipart field');
                    seen.add(k);
                    if (k !== 'file' && (typeof v !== 'string' || Buffer.byteLength(v) > 4096))
                        throw new AppError(413, 'Request body is too large');
                }
                const file = form.get('file');
                if (!file || typeof file === 'string')
                    throw new AppError(422, 'File required');
                for (const k of ['entity_type', 'entity_id', 'doc_type'])
                    if (typeof form.get(k) !== 'string' || !String(form.get(k)).trim())
                        throw new AppError(422, 'Field required');
                if (!file.size)
                    throw fail('Document cannot be empty.');
                if (file.size > maximum)
                    throw new AppError(413, 'Request body is too large');
                const data = Buffer.from(await file.arrayBuffer()), entity = String(form.get('entity_type')), entityID = id(String(form.get('entity_id'))), docType = String(form.get('doc_type')), title = String(form.get('title') ?? '').trim(), type = file.type || 'application/octet-stream';
                if (!types.has(lower(docType.trim())))
                    throw fail('Invalid document type');
                if (!media.has(type.toLowerCase().trim()))
                    throw fail('Disallowed content type');
                if ([...entity].length > 60 || [...title].length > 255)
                    throw fail('entity_type or title exceeds maximum length');
                const name = filename(file.name || 'unnamed_document'), document = randomUUID(), bucket = process.env.VTPL_STORAGE_BUCKET!, object = entity + '/' + entityID + '/' + document + '_' + name + '.enc';
                if ([...object].length > 500)
                    throw fail('Storage key exceeds maximum length');
                const hash = digest(data), encrypted = encrypt(data), row = (await db.query(`INSERT INTO stored_documents(id,entity_type,entity_id,doc_type,title,original_filename,content_type,byte_size,storage_bucket,storage_key,sha256_hash,is_encrypted,encryption_algorithm,is_active,created_at,created_by_id,updated_at,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,true,'AES-256-GCM',true,now(),$12,now(),$12)RETURNING ${fields}`, [document, lower(entity.trim()), entityID, lower(docType.trim()), title || name, name, lower(type.trim()), data.length, bucket, object, hash, a.id])).rows[0];
                await event(db, document, 'UPLOAD', a.id, {
                    entity_type: entity, entity_id: entityID, doc_type: docType, byte_size: data.length, sha256: hash, algorithm: 'AES-256-GCM'
                });
                const client = storage();
                try {
                    try {
                        await client.send(new HeadBucketCommand({ Bucket: bucket }));
                    }
                    catch (e) {
                        if ((e as {
                            $metadata?: {
                                httpStatusCode: number;
                            };
                        }).$metadata?.httpStatusCode !== 404)
                            throw e;
                        await client.send(new CreateBucketCommand({ Bucket: bucket }));
                    }
                    await client.send(new PutObjectCommand({
                        Bucket: bucket, Key: object, Body: encrypted, ContentType: 'application/octet-stream'
                    }));
                }
                finally {
                    client.destroy();
                }
                return json(view(row), 201);
            }
            if (r.method === 'GET' && p.length === 1) {
                const entity = u.searchParams.get('entity_type'), target = u.searchParams.get('entity_id');
                if (entity === null || target === null)
                    throw new AppError(422, 'entity_type and entity_id required');
                const items = (await db.query(`SELECT ${fields} FROM stored_documents WHERE entity_type=$1 AND entity_id=$2 AND is_active ORDER BY created_at DESC`, [lower(entity.trim()), id(target)])).rows.map(view);
                return json({ items, total: items.length });
            }
            const target = id(p[1] ?? ''), row = (await db.query(`SELECT ${fields} FROM stored_documents WHERE id=$1${write || p[2] === 'download' ? ' FOR UPDATE' : ''}`, [target])).rows[0];
            if (!row)
                throw new AppError(404, 'Document not found');
            if (r.method === 'DELETE' && p.length === 2) {
                await db.query('UPDATE stored_documents SET is_active=false,updated_at=now(),updated_by_id=$2 WHERE id=$1', [target, a.id]);
                await event(db, target, 'DEACTIVATE', a.id, {});
                return json({ status: 'deactivated', id: target });
            }
            if (!row.is_active)
                throw new AppError(404, 'Document not found or inactive');
            if (r.method === 'GET' && p[2] === 'download') {
                const client = storage();
                let data: Buffer;
                try {
                    const response = await client.send(new GetObjectCommand({ Bucket: row.storage_bucket, Key: row.storage_key }));
                    if (!response.Body)
                        throw fail('Missing document payload');
                    const chunks: Buffer[] = [];
                    let size = 0;
                    for await (const chunk of response.Body as AsyncIterable<Uint8Array>) {
                        size += chunk.length;
                        if (size > maximum + 28)
                            throw fail('Document exceeds size bounds');
                        chunks.push(Buffer.from(chunk));
                    }
                    data = Buffer.concat(chunks);
                }
                finally {
                    client.destroy();
                }
                if (row.is_encrypted)
                    data = decrypt(data);
                if (digest(data) !== row.sha256_hash)
                    throw fail('Document integrity verification failed (SHA256 mismatch). Document payload may have been tampered with or corrupted.');
                await event(db, target, 'DOWNLOAD', a.id, { byte_size: data.length });
                const response = binary(data, filename(row.original_filename), row.content_type);
                response.headers.set('X-Document-SHA256', row.sha256_hash);
                return response;
            }
            if (r.method === 'GET' && p.length === 2)
                return json(view(row));
            return json({ detail: 'Not found' }, 404);
        });
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status);
        console.error('Document storage operation unavailable');
        return json({ detail: 'Internal server error' }, 500);
    }
}
