import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import { AppError } from '../../errors';
import { digest } from '../tsc/crypto';
import { ownedReceipt } from '../intake/receipts';
import { journal } from './events';
// Compatibility with retained named-record originals. Draft cloud/manual sources do not use this path.
export async function consumeSources(db: DB, actor: string, receiptID: string, version: number, tender: string) {
    const r = await ownedReceipt(db,receiptID,actor);
    if (r.state !== 'ready' || r.draft_id || Number(r.version) !== version) throw new AppError(409,'Use a current, verified, unconsumed receipt');
    const members = (await db.query('SELECT * FROM document_package_members WHERE receipt_id=$1 AND validation_claim_id=$2 ORDER BY normalized_path,id',[r.id,r.claim_id])).rows;
    if (!members.length || members.length > 100) throw new AppError(409,'Verified package contains no supported sources');
    await db.query("UPDATE document_package_receipts SET state='consumed',consumed_by_tender_id=$2,lease_until=NULL,version=version+1,updated_at=clock_timestamp(),updated_by_id=$3 WHERE id=$1",[r.id,tender,actor]);
    await db.query("INSERT INTO document_package_events(id,receipt_id,action,version,created_by_id)VALUES($1,$2,'consumed',$3,$4)",[randomUUID(),r.id,version+1,actor]);
    for (const m of members) {
        if (!m.reader_payload || digest(m.reader_payload) !== m.reader_payload_sha256) throw new Error('Verified source integrity failed');
        const inserted = (await db.query(`INSERT INTO document_readings(id,module,entity_id,filename,format,size_bytes,sha256,original,created_by_id,updated_by_id)
          VALUES($1,'tender_fms',$2,$3,$4,$5,$6,$7,$8,$8) ON CONFLICT(module,entity_id,sha256) DO NOTHING RETURNING *,updated_at::text AS at`,[randomUUID(),tender,m.filename,m.kind,m.byte_size,m.sha256,m.reader_payload,actor])).rows[0];
        const reading = inserted ?? (await db.query("SELECT id FROM document_readings WHERE module='tender_fms' AND entity_id=$1 AND sha256=$2",[tender,m.sha256])).rows[0];
        if (inserted) {
            const event = randomUUID();
            await db.query("INSERT INTO document_reading_events(id,document_id,action,version,actor_id)VALUES($1,$2,'queued',1,$3)",[event,reading.id,actor]);
            await journal(db,event,'document_reading','document_reading',reading.id,1,'queued',actor,inserted.at);
        }
        await db.query('INSERT INTO tender.tender_source_links(member_id,tender_id,document_id,created_by_id)VALUES($1,$2,$3,$4)',[m.id,tender,reading.id,actor]);
    }
}
