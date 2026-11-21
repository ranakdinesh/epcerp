import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
export async function journal(db: DB, event: string, module: string, entity: string, id: string, version: number | string, action: string, actor: string, at: unknown) {
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)
      VALUES($1,$2,$3,$4,$5,0,$6,$7,'user',$1,'platform','restricted',$8)`, [event,module,entity,id,version,action,actor,at]);
}
export async function recordEvent(db: DB, row: { id: string; business_version: string; updated_at_exact: string }, actor: string, action: string) {
    const event = randomUUID();
    await db.query('INSERT INTO tender.tender_fms_record_events(id,intake_id,business_version,action,actor_id)VALUES($1,$2,$3,$4,$5)', [event,row.id,row.business_version,action,actor]);
    await journal(db,event,'tender_fms','tender_record',row.id,row.business_version,action,actor,row.updated_at_exact);
}
