import 'server-only';
import { randomUUID, createCipheriv, randomBytes } from 'node:crypto';
import { pool, transaction } from '../../db';
import { readObject } from './objects';
import { processor } from '../../platform/processor';
import { digest } from '../tsc/crypto';
import { AppError } from '../../errors';
type Member = {
    path: string;
    normalized_path: string;
    filename: string;
    kind: string;
    byte_size: number;
    expanded_bytes: number;
    pages: number;
    sheets: number;
    cells: number;
    scanner_revision: string;
    sha256: string;
    data: string;
};
function seal(bytes: Buffer) {
    const key = Buffer.from(process.env.EPCERP_DOCUMENT_READING_KEY ?? '', 'hex'), n = randomBytes(12), c = createCipheriv('aes-256-gcm', key, n);
    return Buffer.concat([n, c.update(bytes), c.final(), c.getAuthTag()]);
}
async function lines(r: Response, accept: (v: {
    member?: Member;
    path?: string;
    disposition?: string;
    error_code?: string;
    error?: string;
    done?: boolean;
}) => Promise<void>) {
    const reader = r.body!.getReader(), decode = new TextDecoder('utf-8', { fatal: true });
    let pending = '', total = 0;
    try {
        for (;;) {
            const { value, done } = await reader.read();
            if (done)
                break;
            total += value.length;
            if (total > 400 * 1048576)
                throw Error('payload_too_large');
            pending += decode.decode(value, { stream: true });
            let end;
            while ((end = pending.indexOf('\n')) >= 0) {
                const line = pending.slice(0, end);
                pending = pending.slice(end + 1);
                if (line)
                    await accept(JSON.parse(line));
            }
            if (pending.length > 48 * 1048576)
                throw Error('payload_too_large');
        }
        pending += decode.decode();
        if (pending.trim())
            throw Error('verification_failed');
    }
    finally {
        await reader.cancel().catch(() => {
        });
        reader.releaseLock();
    }
}
export async function verifyOnce() {
    const slot = await pool().connect();
    let acquired = false;
    try {
        acquired = (await slot.query('SELECT pg_try_advisory_lock(735841920001) AS locked')).rows[0].locked;
        if (!acquired)
            return false;
        const claim = randomUUID(), job = await transaction(async (db) => {
            const exhausted = (await db.query("UPDATE document_package_receipts SET state='failed',error_code='verification_attempts_exhausted',lease_until=NULL,version=version+1,updated_at=clock_timestamp(),updated_by_id=owner_id WHERE state='verifying'AND attempts>=3 AND lease_until<clock_timestamp()RETURNING id,version,owner_id")).rows;
            for (const row of exhausted)
                await db.query("INSERT INTO document_package_events(id,receipt_id,action,version,error_code,created_by_id)VALUES($1,$2,'verification_exhausted',$3,'verification_attempts_exhausted',$4)", [randomUUID(), row.id, row.version, row.owner_id]);
            return (await db.query("WITH next AS(SELECT id FROM document_package_receipts WHERE state='verifying'AND attempts<3 AND expires_at>clock_timestamp()AND(lease_until IS NULL OR lease_until<clock_timestamp())ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1)UPDATE document_package_receipts r SET claim_id=$1,lease_until=clock_timestamp()+interval '1 minute',attempts=least(attempts+1,3),updated_at=clock_timestamp(),updated_by_id=NULL FROM next n WHERE r.id=n.id RETURNING r.*", [claim])).rows[0];
        });
        if (!job)
            return false;
        const controller = new AbortController();
        let renewing = false;
        const renew = setInterval(async () => {
            if (renewing)
                return;
            renewing = true;
            try {
                const r = await pool().query("UPDATE document_package_receipts SET lease_until=clock_timestamp()+interval '1 minute',updated_at=clock_timestamp()WHERE id=$1 AND claim_id=$2 AND state='verifying'AND lease_until>clock_timestamp()", [job.id, claim]);
                if (r.rowCount !== 1)
                    controller.abort();
            }
            catch {
                controller.abort();
            }
            finally {
                renewing = false;
            }
        }, 20000);
        renew.unref();
        let code = '';
        try {
            const files = (await pool().query('SELECT * FROM document_package_files WHERE receipt_id=$1 ORDER BY created_at,id', [job.id])).rows;
            for (const file of files) {
                const bytes = await readObject(file.object_key, Number(file.declared_size), file.declared_sha256), response = await processor('/verify', bytes, controller.signal, file.filename);
                let done = false;
                await lines(response, async (item) => {
                    if (item.error)
                        throw Error(item.error);
                    if (item.done) {
                        done = true;
                        return;
                    }
                    await transaction(async (db) => {
                        const current = (await db.query("SELECT id FROM document_package_receipts WHERE id=$1 AND claim_id=$2 AND state='verifying'AND lease_until>clock_timestamp()AND expires_at>clock_timestamp()FOR UPDATE", [job.id, claim])).rows[0];
                        if (!current)
                            throw Error('claim_expired');
                        if (item.member) {
                            const m = item.member, data = Buffer.from(m.data, 'base64');
                            if (data.length !== m.byte_size || digest(data) !== m.sha256)
                                throw Error('document_validation_failed');
                            const totals = (await db.query('SELECT count(*)::int AS count,coalesce(sum(expanded_bytes),0)::bigint AS bytes,coalesce(sum(pages),0)::int AS pages,coalesce(sum(sheets),0)::int AS sheets,coalesce(sum(cells),0)::bigint AS cells FROM document_package_members WHERE receipt_id=$1 AND validation_claim_id=$2', [job.id, claim])).rows[0];
                            if (totals.count >= 100 || Number(totals.bytes) + m.expanded_bytes > 250 * 1048576 || totals.pages + m.pages > 3000 || totals.sheets + m.sheets > 200 || Number(totals.cells) + m.cells > 250000)
                                throw Error('payload_too_large');
                            const payload = seal(data);
                            await db.query('INSERT INTO document_package_members(id,receipt_id,file_id,path,normalized_path,filename,kind,byte_size,expanded_bytes,sha256,reader_payload,reader_payload_sha256,validation_claim_id,scanner_revision,pages,sheets,cells,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)', [randomUUID(), job.id, file.id, m.path, m.normalized_path, m.filename, m.kind, m.byte_size, m.expanded_bytes, m.sha256, payload, digest(payload), claim, m.scanner_revision, m.pages, m.sheets, m.cells, job.owner_id]);
                        }
                        else if (item.path && item.disposition) {
                            await db.query('INSERT INTO document_package_manifest(id,receipt_id,file_id,claim_id,path,disposition,error_code)VALUES($1,$2,$3,$4,$5,$6,$7)', [randomUUID(), job.id, file.id, claim, item.path, item.disposition, item.error_code || '']);
                        }
                    });
                });
                if (!done)
                    throw Error('verification_failed');
            }
        }
        catch (e) {
            const allowed = new Set(['dependency_unavailable', 'archive_depth_limit', 'malware_detected', 'unsafe_archive_path', 'archive_path_collision', 'unsafe_archive_entry', 'archive_expansion_limit', 'member_limit', 'invalid_archive', 'archive_integrity_failed', 'unsupported_media', 'invalid_pdf', 'invalid_office', 'active_office_content', 'external_office_reference', 'office_expansion_limit', 'payload_too_large', 'empty_package', 'document_validation_failed', 'verification_failed']);
            code = e instanceof AppError && e.status === 503 ? 'dependency_unavailable' : e instanceof Error && allowed.has(e.message) ? e.message : 'verification_failed';
        }
        finally {
            clearInterval(renew);
        }
        await transaction(async (db) => {
            const count = Number((await db.query('SELECT count(*)FROM document_package_members WHERE receipt_id=$1 AND validation_claim_id=$2', [job.id, claim])).rows[0].count);
            if (!count && !code)
                code = 'empty_package';
            const state = !code ? 'ready' : code === 'dependency_unavailable' && job.attempts < 3 ? 'verifying' : 'failed';
            const r = (await db.query("UPDATE document_package_receipts SET state=$3,error_code=$4,lease_until=CASE WHEN $3='verifying'THEN clock_timestamp()+interval '30 seconds'ELSE NULL END,version=version+1,updated_at=clock_timestamp(),updated_by_id=owner_id WHERE id=$1 AND claim_id=$2 AND state='verifying'AND lease_until>clock_timestamp()AND expires_at>clock_timestamp()RETURNING version", [job.id, claim, state, code || null])).rows[0];
            if (!r)
                throw Error('Verification claim expired');
            if (state === 'ready')
                await db.query("UPDATE document_package_files SET state='ready',version=version+1,updated_at=clock_timestamp(),updated_by_id=$2 WHERE receipt_id=$1 AND state='received'", [job.id, job.owner_id]);
            await db.query('INSERT INTO document_package_events(id,receipt_id,action,version,error_code,created_by_id)VALUES($1,$2,$3,$4,$5,$6)', [randomUUID(), job.id, state, r.version, code || null, job.owner_id]);
        });
        return true;
    }
    finally {
        if (acquired)
            await slot.query('SELECT pg_advisory_unlock(735841920001)').catch(() => {
            });
        slot.release();
    }
}
const state = globalThis as typeof globalThis & {
    nativeVerificationStarted?: boolean;
};
export function startVerification() {
    if (state.nativeVerificationStarted || process.env.EPCERP_PLATFORM_OWNER !== 'nextjs')
        return;
    state.nativeVerificationStarted = true;
    let running = false;
    const timer = setInterval(async () => {
        if (running)
            return;
        running = true;
        try {
            await verifyOnce();
        }
        catch {
            console.error('Source verification unavailable');
        }
        finally {
            running = false;
        }
    }, 3000);
    timer.unref();
}
