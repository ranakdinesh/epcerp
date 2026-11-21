import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../db';
import type { Notice, History } from './types';
import { InboxError } from './validation';
// Keep PostgreSQL microseconds: JavaScript Date would lose source/history precision.
export const timeSQL = (column: string) => `to_char(${column} AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')`;
const timestampFields = ['read_at','acknowledged_at','created_at','updated_at'];
export const exactColumns = timestampFields.map(k => `${timeSQL(k)} AS ${k}`).join(',');
export function timestamp(value: string | null) { return value?.replace(/(\.\d*?[1-9])0+Z$/,'$1Z').replace(/\.0+Z$/,'Z') ?? null; }
export function notice(row: Record<string, unknown>): Notice {
    const optional = Object.fromEntries(['source_action_id','notice_kind','title','link'].flatMap(k => row[k] ? [[k,row[k]]] : []));
    return { ...optional,id:String(row.id),source_event_id:String(row.source_event_id),recipient_id:String(row.recipient_id),director_designation_id:String(row.director_designation_id ?? ''),director_version:Number(row.director_version),decision_id:String(row.decision_id),project_id:String(row.project_id ?? ''),source_version:Number(row.source_version),decision_action:String(row.decision_action),version:Number(row.version),is_active:row.is_active === true,
        read_at:timestamp(row.read_at as string | null),acknowledged_at:timestamp(row.acknowledged_at as string | null),created_at:timestamp(row.created_at as string)!,updated_at:timestamp(row.updated_at as string)!,created_by_id:row.created_by_id as string | null,updated_by_id:row.updated_by_id as string | null };
}
export async function ownNotice(db: DB, id: string, recipient: string) {
    const row = (await db.query(`SELECT n.*,${exactColumns} FROM communication_notices n WHERE id=$1 AND recipient_id=$2 FOR UPDATE`,[id,recipient])).rows[0];
    if (!row) throw new InboxError(404,'Inbox notice not found');
    return notice(row);
}
export async function history(db: DB, id: string, recipient: string): Promise<History[]> {
    return (await db.query(`SELECT e.*,${timeSQL('e.created_at')} AS at FROM communication_notice_events e JOIN communication_notices n ON n.id=e.notice_id WHERE n.id=$1 AND n.recipient_id=$2 ORDER BY e.notice_version DESC LIMIT 100`,[id,recipient])).rows.map(r => ({id:r.id,action:r.action,before:r.before_snapshot,after:r.after_snapshot,created_by_id:r.created_by_id ?? '',created_at:timestamp(r.at)!}));
}
export async function receiptEvent(db: DB, before: Notice, after: Notice, action: string, actor: string) {
    const id = randomUUID();
    await db.query('INSERT INTO communication_notice_events(id,notice_id,action,notice_version,before_snapshot,after_snapshot,created_at,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8)',[id,after.id,action,after.version,before,after,after.updated_at,actor]);
    const scope = ['supplier_review','tender_manager'].includes(after.notice_kind ?? '') ? 'platform' : 'project';
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,causation_id,scope_kind,scope_id,visibility,occurred_at)
      VALUES($1,'communication_inbox','director_notice',$2,$3,0,$4,$5,'user',$6,$6,$7,$8,'restricted',$9)`,[id,after.id,after.version,action,actor,after.source_event_id,scope,after.project_id || null,after.updated_at]);
}
