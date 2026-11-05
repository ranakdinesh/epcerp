import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction } from '../../db';
import { access } from './access';
import { AppError } from '../../errors';
import { uuid } from '../../purchasing/core/validation';
import { decryptBytes } from '../tsc/crypto';
import { json, binary } from '../../purchasing/http-input';
const sources = `id,filename,format,status,size_bytes,sha256,version,attempts,error_message AS error,segment_count,warnings`;
export const ownsHistorical = (path: string) => path === '/tender-fms' || /^\/tender-fms\/(?:upload$|[^/]+\/(?:history$|documents\/[^/]+\/[^/]+$))/.test(path);
// Read retained evidence in place. Historical local model/OCR queues are retired.
export async function handleHistorical(r: Request) {
    try {
        const p = new URL(r.url).pathname.replace(/^\/api\/v1\/tender-fms\/?/, '').split('/').filter(Boolean), token = r.headers.get('authorization')?.replace(/^Bearer /i, '') ?? '';
        return await transaction(async (db) => {
            const a = await access(db, token, 'draft', r.method !== 'GET');
            if (r.method !== 'GET')
                throw new AppError(409, 'Local reading is retired. Upload documents into a tender draft for cloud or manual TSC.');
            if (!p.length) {
                const rows = (await db.query("SELECT id,title,owner_id,version,is_active,created_at,updated_at,created_by_id,updated_by_id FROM tender.tender_fms_intakes WHERE is_active AND($1 OR owner_id=$2)ORDER BY updated_at DESC,id", [a.allows('tenders.portfolio.read'), a.id])).rows;
                for (const row of rows)
                    row.documents = (await db.query(`SELECT ${sources} FROM document_readings WHERE module='tender_fms'AND entity_id=$1 AND is_active ORDER BY created_at,id`, [row.id])).rows.map(s => ({ ...s, size_bytes: Number(s.size_bytes) }));
                return json(rows);
            }
            const id = uuid(p[0]), intake = (await db.query('SELECT * FROM tender.tender_fms_intakes WHERE id=$1 AND is_active', [id])).rows[0];
            if (!intake)
                throw new AppError(404, 'Tender not found');
            if (intake.owner_id !== a.id && !a.allows('tenders.portfolio.read'))
                throw new AppError(403, 'Tender permission required');
            if (p.length === 2 && p[1] === 'history')
                return json((await db.query("SELECT id,action,actor_id,created_at,document_id FROM tender.tender_fms_events WHERE intake_id=$1 UNION ALL SELECT e.id,e.action,e.actor_id,e.created_at,e.document_id FROM document_reading_events e JOIN document_readings d ON d.id=e.document_id WHERE d.module='tender_fms'AND d.entity_id=$1 ORDER BY created_at DESC,id", [id])).rows);
            if (p.length === 4 && p[1] === 'documents' && ['original', 'text', 'result'].includes(p[3])) {
                const doc = (await db.query("SELECT * FROM document_readings WHERE id=$1 AND module='tender_fms'AND entity_id=$2 AND is_active", [uuid(p[2]), id])).rows[0];
                if (!doc)
                    throw new AppError(404, 'Document not found');
                let data: Buffer;
                if (p[3] === 'original')
                    data = decryptBytes(doc.original, doc.sha256);
                else {
                    if (doc.status !== 'ready')
                        throw new AppError(409, 'Document result is not ready');
                    const result = (await db.query('SELECT * FROM document_reading_results WHERE document_id=$1', [doc.id])).rows[0];
                    if (!result)
                        throw new AppError(409, 'Document result is not ready');
                    data = decryptBytes(result.result, result.result_sha256);
                    if (p[3] === 'text') {
                        const retained = JSON.parse(data.toString()) as {
                            segments?: {
                                label?: string;
                                text?: string;
                                native_text?: string;
                                ocr_text?: string;
                            }[];
                        };
                        data = Buffer.from((retained.segments ?? []).map(s => '## ' + (s.label ?? 'Source') + '\n\n' + (s.text ?? s.native_text ?? s.ocr_text ?? '')).join('\n\n'));
                    }
                }
                await db.query('INSERT INTO tender.tender_fms_events(id,intake_id,action,document_id,actor_id)VALUES($1,$2,$3,$4,$5)', [randomUUID(), id, 'source_' + p[3] + '_accessed', doc.id, a.id]);
                return binary(data, p[3] === 'original' ? doc.filename : doc.filename + '.' + (p[3] === 'text' ? 'md' : 'json'), p[3] === 'original' ? 'application/octet-stream' : p[3] === 'text' ? 'text/plain; charset=utf-8' : 'application/json');
            }
            return json({ detail: 'Not found' }, 404);
        });
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status);
        console.error('Historical tender evidence unavailable');
        return json({ detail: 'Tender evidence unavailable' }, 500);
    }
}
