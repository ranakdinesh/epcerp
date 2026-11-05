import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { AppError } from '../../errors';
import { id, object, str, integer } from '../tsc/validation';
import { digest } from '../tsc/crypto';
import { access, admit } from './access';
import { writeObject, readObject } from './objects';
export const goJSON = (value: unknown) => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, c => '\\u' + c.charCodeAt(0).toString(16).padStart(4, '0'));
export const lower = (value: string) => [...value].map(c => c === 'İ' ? 'i' : c.toLowerCase()).join('');
type Declaration = {
    ClientKey: string;
    Filename: string;
    Kind: string;
    SHA256: string;
    Size: number;
};
function declarations(value: unknown): Declaration[] { if (!Array.isArray(value) || value.length < 1 || value.length > 100)
    throw new AppError(422, 'Declare between 1 and 100 files'); const keys = new Set<string>(), names = new Set<string>(); let total = 0; return value.map(v => { const r = object(v, ['client_file_key', 'filename', 'kind', 'size_bytes', 'sha256']), ClientKey = id(r.client_file_key), Filename = str(r.filename, 200), Kind = str(r.kind, 8), SHA256 = str(r.sha256, 64), Size = integer(r.size_bytes, 1, 64 << 20); if (!Filename || Filename !== Filename.trim() || ['.', '..'].includes(Filename) || /[\/\\\p{Cc}\p{Cf}\uD800-\uDFFF]/u.test(Filename))
    throw new AppError(422, 'Use a safe leaf filename'); if (!['pdf', 'doc', 'docx', 'xls', 'xlsx', 'zip'].includes(Kind))
    throw new AppError(415, 'Unsupported document kind'); if (!lower(Filename).endsWith('.' + Kind) || !/^[a-f\d]{64}$/.test(SHA256))
    throw new AppError(422, 'Declared kind or checksum does not match the filename'); if (Size > (Kind === 'zip' ? 64 : 32) * 1048576 || (total += Size) > 250 * 1048576)
    throw new AppError(413, 'Receipt exceeds its supported size'); if (keys.has(ClientKey) || names.has(lower(Filename)))
    throw new AppError(422, 'Duplicate file keys or ambiguous filenames'); keys.add(ClientKey); names.add(lower(Filename)); return { ClientKey, Filename, Kind, SHA256, Size }; }); }
export async function ownedReceipt(db: DB, receipt: string, actor: string) { const row = (await db.query('SELECT * FROM document_package_receipts WHERE id=$1 AND owner_id=$2 FOR UPDATE', [id(receipt), actor])).rows[0]; if (!row)
    throw new AppError(404, 'Document receipt not found'); if (row.state !== 'consumed' && new Date(row.expires_at).getTime() <= Date.now()) {
    row.state = 'expired';
    row.error_code = 'receipt_expired';
} return row; }
async function dto(db: DB, row: Record<string, unknown>) { const files = (await db.query('SELECT id AS file_id,client_file_key,filename,version,state,declared_size AS size_bytes,declared_sha256 AS sha256,error_code FROM document_package_files WHERE receipt_id=$1 ORDER BY created_at,id', [row.id])).rows.map(f => ({ ...f, version: Number(f.version), size_bytes: Number(f.size_bytes) })); return { id: row.id, version: Number(row.version), state: row.state, error_code: row.error_code, limit_profile_revision: Number(row.limit_profile_revision), expires_at: row.expires_at, files, member_count: Number((await db.query('SELECT count(*)FROM document_package_members WHERE receipt_id=$1 AND validation_claim_id=$2', [row.id, row.claim_id])).rows[0].count), unresolved_count: ['ready', 'consumed'].includes(String(row.state)) ? 0 : files.length }; }
async function event(db: DB, receipt: string, actor: string, action: string, version: number, file?: string, error?: string) { await db.query('INSERT INTO document_package_events(id,receipt_id,file_id,action,version,error_code,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7)', [randomUUID(), receipt, file ?? null, action, version, error ?? null, actor]); }
async function operation(db: DB, actor: string, command: string, context: string, key: string, fingerprint: string) { id(key); const old = (await db.query('SELECT * FROM document_package_operations WHERE actor_id=$1 AND command=$2 AND context_key=$3 AND idempotency_key=$4', [actor, command, context, key])).rows[0]; if (old && old.fingerprint !== fingerprint)
    throw new AppError(409, 'Operation key was used with different input'); return old; }
async function saveOperation(db: DB, actor: string, command: string, context: string, key: string, fingerprint: string, receipt: string) { await db.query('INSERT INTO document_package_operations(actor_id,command,context_key,idempotency_key,fingerprint,receipt_id)VALUES($1,$2,$3,$4,$5,$6)', [actor, command, context, key, fingerprint, receipt]); }
export async function create(token: string, key: string, value: unknown) { await admit(token, 'receipt', 'create', true); id(key); const files = declarations(object(value, ['files']).files), fingerprint = digest(goJSON(files)); return transaction(async (db) => { const actor = await access(db, token, 'receipt', true), old = await operation(db, actor.id, 'create', 'tender_capture', key, fingerprint); if (old)
    return { value: await dto(db, await ownedReceipt(db, old.receipt_id, actor.id)), replay: true }; const receipt = randomUUID(), row = (await db.query('INSERT INTO document_package_receipts(id,owner_id,created_by_id,updated_by_id)VALUES($1,$2,$2,$2)RETURNING *', [receipt, actor.id])).rows[0]; for (const f of files)
    await db.query('INSERT INTO document_package_files(id,receipt_id,client_file_key,filename,kind,declared_size,declared_sha256,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)', [randomUUID(), receipt, f.ClientKey, f.Filename, f.Kind, f.Size, f.SHA256, actor.id]); await saveOperation(db, actor.id, 'create', 'tender_capture', key, fingerprint, receipt); await event(db, receipt, actor.id, 'created', 1); return { value: await dto(db, row), replay: false }; }); }
export async function get(token: string, receipt: string) { await admit(token, 'receipt', 'read'); return transaction(async (db) => { const actor = await access(db, token, 'receipt'); return dto(db, await ownedReceipt(db, receipt, actor.id)); }); }
async function file(db: DB, receipt: string, document: string) { const row = (await db.query('SELECT * FROM document_package_files WHERE id=$1 AND receipt_id=$2 FOR UPDATE', [id(document), receipt])).rows[0]; if (!row)
    throw new AppError(404, 'Document receipt file not found'); return row; }
function available(row: {
    state: string;
}) { if (['expired', 'consumed'].includes(row.state))
    throw new AppError(409, 'Document receipt is expired or already consumed'); }
export async function preflight(token: string, receipt: string, document: string) { await transaction(async (db) => { const actor = await access(db, token, 'receipt', true), r = await ownedReceipt(db, receipt, actor.id); available(r); if (!['receiving', 'verifying', 'ready'].includes(r.state))
    throw new AppError(409, 'Receipt changed; refresh before continuing'); await file(db, receipt, document); }); }
export async function upload(token: string, receipt: string, document: string, key: string, version: number, request: Request) {
    await admit(token, 'receipt', 'upload', true);
    id(key);
    integer(version, 1);
    const claim = randomUUID(), prepared = await transaction(async (db) => { const actor = await access(db, token, 'receipt', true), r = await ownedReceipt(db, receipt, actor.id); available(r); const f = await file(db, receipt, document); if (f.upload_key === key && ['received', 'ready'].includes(f.state)) {
        if (Number(f.upload_input_version) !== version)
            throw new AppError(409, 'Upload key was used with different input');
        return { replay: true, value: await dto(db, r), f, actor: actor.id };
    } if (r.state !== 'receiving' || Number(f.version) !== version || !['declared', 'uploading'].includes(f.state) || (f.state === 'uploading' && new Date(f.lease_until).getTime() > Date.now()))
        throw new AppError(409, 'Receipt or file changed; refresh before continuing'); const active = Number((await db.query("SELECT count(*)FROM document_package_files f JOIN document_package_receipts r ON r.id=f.receipt_id WHERE r.owner_id=$1 AND f.state='uploading' AND f.lease_until>clock_timestamp()", [actor.id])).rows[0].count); if (active >= 2)
        throw new AppError(429, 'At most two uploads may be active'); await db.query("UPDATE document_package_files SET state='uploading',version=version+1,claim_id=$2,object_key=$2,lease_until=clock_timestamp()+interval '10 minutes',upload_key=$3,upload_input_version=version,updated_at=clock_timestamp(),updated_by_id=$4 WHERE id=$1", [document, claim, key, actor.id]); await db.query('INSERT INTO document_package_objects(id,receipt_id,file_id)VALUES($1,$2,$3)', [claim, receipt, document]); await event(db, receipt, actor.id, 'upload_claimed', version + 1, document); return { replay: false, value: null, f, actor: actor.id }; });
    if (prepared.replay)
        return { value: prepared.value!, replay: true };
    try {
        await writeObject(claim, request, Number(prepared.f.declared_size), prepared.f.declared_sha256);
        return await transaction(async (db) => { const actor = await access(db, token, 'receipt', true), r = await ownedReceipt(db, receipt, actor.id); available(r); const changed = (await db.query("UPDATE document_package_files SET state='received',claim_id=NULL,lease_until=NULL,error_code=NULL,version=version+1,updated_at=clock_timestamp(),updated_by_id=$4 WHERE id=$1 AND state='uploading' AND claim_id=$2 AND upload_key=$3 AND lease_until>clock_timestamp() RETURNING id", [document, claim, key, actor.id])).rows.length; if (!changed)
            throw new AppError(409, 'Upload claim expired'); await db.query('UPDATE document_package_receipts SET version=version+1,updated_at=clock_timestamp(),updated_by_id=$2 WHERE id=$1', [receipt, actor.id]); await event(db, receipt, actor.id, 'uploaded', version + 2, document); return { value: await dto(db, await ownedReceipt(db, receipt, actor.id)), replay: false }; });
    }
    catch (e) {
        await transaction(async (db) => { const row = (await db.query("UPDATE document_package_files SET state='declared',claim_id=NULL,lease_until=NULL,version=version+1,error_code='upload_failed',updated_at=clock_timestamp(),updated_by_id=$3 WHERE id=$1 AND state='uploading' AND claim_id=$2 RETURNING version", [document, claim, prepared.actor])).rows[0]; if (row)
            await event(db, receipt, prepared.actor, 'upload_aborted', Number(row.version), document, 'upload_failed'); }).catch(() => { });
        throw e;
    }
}
export async function seal(token: string, receipt: string, key: string, value: unknown) { await admit(token, 'receipt', 'seal', true); const version = integer(object(value, ['expected_version']).expected_version, 1), fp = digest(goJSON(version)); return transaction(async (db) => { const actor = await access(db, token, 'receipt', true), r = await ownedReceipt(db, receipt, actor.id), old = await operation(db, actor.id, 'seal', receipt, key, fp); if (old)
    return { value: await dto(db, r), replay: true }; available(r); if (Number(r.version) !== version || !['receiving', 'failed'].includes(r.state) || r.attempts >= 3)
    throw new AppError(409, 'Receipt changed; refresh before continuing'); if ((await db.query("SELECT id FROM document_package_files WHERE receipt_id=$1 AND state NOT IN('received','ready')LIMIT 1", [receipt])).rows.length)
    throw new AppError(409, 'Upload every declared file before verification'); await db.query("UPDATE document_package_receipts SET state='verifying',version=version+1,updated_at=clock_timestamp(),updated_by_id=$2 WHERE id=$1", [receipt, actor.id]); await saveOperation(db, actor.id, 'seal', receipt, key, fp, receipt); await event(db, receipt, actor.id, 'verification_queued', version + 1); return { value: await dto(db, await ownedReceipt(db, receipt, actor.id)), replay: false }; }); }
export async function original(token: string, receipt: string, document: string) { await admit(token, 'receipt', 'read'); const source = await transaction(async (db) => { const actor = await access(db, token, 'receipt'); const r = await ownedReceipt(db, receipt, actor.id); const f = await file(db, receipt, document); if (f.kind === 'zip' && r.draft_id && ['ready','consumed'].includes(r.state)) throw new AppError(410, 'ZIP extracted. Download its individual documents from the tender.'); if (!f.object_key || ['declared', 'uploading'].includes(f.state))
    throw new AppError(404, 'Original upload is not available'); return f; }); const bytes = await readObject(source.object_key, Number(source.declared_size), source.declared_sha256); await transaction(async (db) => { const actor = await access(db, token, 'receipt'); await ownedReceipt(db, receipt, actor.id); const current = await file(db, receipt, document); if (current.object_key !== source.object_key)
    throw new AppError(409, 'Original upload changed'); }); return { bytes, filename: source.filename as string }; }
