import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { AppError } from '../../errors';
import { access, admit } from './access';
import { id, object, str } from '../tsc/validation';
import { decryptBytes } from '../tsc/crypto';
function view(row: Record<string, unknown>) { return { engine: 'nextjs', id: row.id, name: row.name, version: Number(row.version), created_at: row.created_at, updated_at: row.updated_at, batches: [] as unknown[] }; }
export async function ownedDraft(db: DB, draft: string, actor: string) { const row = (await db.query('SELECT * FROM tender.document_drafts WHERE id=$1 AND owner_id=$2 AND is_active FOR UPDATE', [id(draft), actor])).rows[0]; if (!row)
    throw new AppError(404, 'Tender draft not found'); return row; }
export async function create(token: string, key: string, value: unknown) { await admit(token, 'draft', 'create', true); id(key); const r = object(value, ['name']), name = str(r.name, 2048).trim(); if ([...name].length < 2 || [...name].length > 255 || /\p{Cc}/u.test(name))
    throw new AppError(422, 'Tender name must contain 2–255 characters without control characters'); return transaction(async (db) => { const actor = await access(db, token, 'draft', true), old = (await db.query('SELECT * FROM tender.document_drafts WHERE owner_id=$1 AND operation_key=$2 FOR UPDATE', [actor.id, key])).rows[0]; if (old) {
    if (old.name !== name)
        throw new AppError(409, 'Operation key was used with another name');
    return view(old);
} const row = (await db.query('INSERT INTO tender.document_drafts(id,name,owner_id,operation_key,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$3,$3)RETURNING *', [randomUUID(), name, actor.id, key])).rows[0]; await db.query("INSERT INTO tender.draft_events(id,draft_id,action,created_by_id)VALUES($1,$2,'created',$3)", [randomUUID(), row.id, actor.id]); return view(row); }); }
export async function list(token: string, query: URLSearchParams) { await admit(token, 'draft', 'read'); const number = (name: string, fallback: number, max: number) => { const text = query.get(name); if (text != null && !/^\d+$/.test(text))
    throw new AppError(422, 'Invalid page'); const n = Number(text ?? fallback); if (!Number.isSafeInteger(n) || n < 1 || n > max)
    throw new AppError(422, 'Invalid page'); return n; }, page = number('page', 1, 100000), size = number('page_size', 20, 100), search = query.get('search') ?? '', sort = query.get('sort') || 'created', direction = query.get('direction') || 'desc'; if (Buffer.byteLength(search) > 200 || !['created', 'name'].includes(sort) || !['asc', 'desc'].includes(direction))
    throw new AppError(422, 'Invalid draft query'); return transaction(async (db) => { const actor = await access(db, token, 'draft'), filter = 'owner_id=$1 AND is_active AND strpos(lower(name),lower($2))>0', rows = (await db.query(`SELECT * FROM tender.document_drafts WHERE ${filter} ORDER BY ${sort === 'name' ? 'name' : 'created_at'} ${direction === 'asc' ? 'ASC' : 'DESC'},id LIMIT $3 OFFSET $4`, [actor.id, search, size, (page - 1) * size])).rows, total = Number((await db.query(`SELECT count(*) AS count FROM tender.document_drafts WHERE ${filter}`, [actor.id, search])).rows[0].count); return { items: rows.map(view), total }; }); }
export async function get(token: string, draft: string) { await admit(token, 'draft', 'read'); return transaction(async (db) => { const actor = await access(db, token, 'draft'), d = await ownedDraft(db, draft, actor.id), batches = (await db.query('SELECT * FROM document_package_receipts WHERE draft_id=$1 ORDER BY created_at,id', [draft])).rows; const result = view(d); for (const receipt of batches) {
    const files = (await db.query('SELECT id,filename AS name,state,coalesce(error_code,\'\') AS error,declared_sha256 AS sha256,version FROM document_package_files WHERE receipt_id=$1 ORDER BY created_at,id', [receipt.id])).rows.map(f => ({ ...f, version: Number(f.version) })), manifest = (await db.query('SELECT path,disposition AS state,error_code AS code FROM document_package_manifest WHERE receipt_id=$1 AND claim_id=$2 ORDER BY created_at,id LIMIT 1000', [receipt.id, receipt.claim_id])).rows, sources = receipt.state === 'consumed' ? (await db.query(`SELECT d.id,m.id AS member_id,m.path,m.filename,m.sha256,d.status,d.error_message AS error,d.version,d.attempts,d.segment_count AS segments,d.warnings,d.updated_at,m.pages AS expected_pages,m.sheets AS expected_sheets FROM document_package_members m JOIN tender.draft_sources s ON s.member_id=m.id AND s.draft_id=$1 JOIN document_readings d ON d.id=s.document_id WHERE m.receipt_id=$2 AND m.validation_claim_id=$3 ORDER BY m.normalized_path,m.id`, [draft, receipt.id, receipt.claim_id])).rows : [];
    result.batches.push({ id: receipt.id, revision: receipt.id, state: receipt.state, error: receipt.error_code ?? '', version: Number(receipt.version), attempts: receipt.attempts, updated_at: receipt.updated_at, files, manifest, sources });
} return result; }); }
export async function attach(token: string, draft: string, value: unknown) { await admit(token, 'draft', 'create', true); const r = object(value, ['receipt_id']), receipt = id(r.receipt_id); await transaction(async (db) => { const actor = await access(db, token, 'draft', true); await ownedDraft(db, draft, actor.id); const row = (await db.query('SELECT * FROM document_package_receipts WHERE id=$1 AND owner_id=$2 FOR UPDATE', [receipt, actor.id])).rows[0]; if (!row)
    throw new AppError(404, 'Document receipt not found'); if (row.draft_id === draft)
    return; const count = Number((await db.query('SELECT count(*)FROM document_package_receipts WHERE draft_id=$1', [draft])).rows[0].count); if (count >= 100)
    throw new AppError(422, 'Draft exceeds 100 upload batches'); if (row.draft_id || row.consumed_by_tender_id || !['receiving', 'verifying', 'ready', 'failed'].includes(row.state) || new Date(row.expires_at).getTime() <= Date.now())
    throw new AppError(409, 'Document receipt is unavailable for attachment'); await db.query("UPDATE document_package_receipts SET draft_id=$2,expires_at='9999-12-31'::timestamptz,updated_at=clock_timestamp(),updated_by_id=$3 WHERE id=$1", [receipt, draft, actor.id]); await db.query("INSERT INTO document_package_events(id,receipt_id,action,version,created_by_id)VALUES($1,$2,'draft_retained',1,$3)", [randomUUID(), receipt, actor.id]); await db.query('UPDATE tender.document_drafts SET version=version+1,updated_at=clock_timestamp(),updated_by_id=$2 WHERE id=$1', [draft, actor.id]); await db.query("INSERT INTO tender.draft_events(id,draft_id,action,receipt_id,created_by_id)VALUES($1,$2,'source_batch_retained',$3,$4)", [randomUUID(), draft, receipt, actor.id]); }); }
export async function artifact(token: string, draft: string, document: string, kind: string) { await admit(token, 'draft', 'read'); return transaction(async (db) => { const actor = await access(db, token, 'draft'); await ownedDraft(db, draft, actor.id); const row = (await db.query("SELECT * FROM document_readings WHERE id=$1 AND module='tender_draft' AND entity_id=$2", [id(document), draft])).rows[0]; if (!row)
    throw new AppError(404, 'Document not found'); if (kind === 'history')
    return JSON.stringify((await db.query('SELECT id,document_id,action,version,actor_id,created_at FROM document_reading_events WHERE document_id=$1 ORDER BY created_at DESC,id', [document])).rows, null, 2); if (kind === 'original')
    return decryptBytes(row.original, row.sha256); if (!['text', 'result'].includes(kind))
    throw new AppError(404, 'Artifact not found'); if (row.status !== 'ready')
    throw new AppError(409, 'Document result is not ready'); const result = (await db.query('SELECT * FROM document_reading_results WHERE document_id=$1', [document])).rows[0]; if (!result)
    throw new AppError(409, 'Document result is not ready'); const bytes = decryptBytes(result.result, result.result_sha256); if (kind === 'result')
    return bytes; const retained = JSON.parse(bytes.toString()) as {
    segments?: {
        label?: string;
        text?: string;
        native_text?: string;
        ocr_text?: string;
    }[];
}; return '# Retained legacy reading\n\nOriginal documents and result JSON remain authoritative. Local extraction is retired for cloud TSC.\n\n' + (retained.segments ?? []).map(s => '## ' + (s.label ?? 'Source') + '\n\n' + (s.text ?? s.native_text ?? s.ocr_text ?? '')).join('\n\n'); }); }
