import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../db';
// Published Inbox writer: the caller owns the shared business transaction.
export async function tenderAssignmentNotice(db: DB, action: string) {
    const source = (await db.query(`SELECT a.*,e.source_module,e.entity_type,e.entity_id,e.entity_version,e.scope_kind,e.scope_id,e.visibility,e.action AS event_action,e.actor_id,
      to_char(e.occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS at,
      e.occurred_at=a.created_at AS same_time FROM tender.tender_source_actions a JOIN business_events e ON e.id=a.source_event_id WHERE a.id=$1`, [action])).rows[0];
    if (!source || source.state !== 'pending' || !source.is_active || source.source_module !== 'tender_fms' || source.entity_type !== 'tender_assignment' || source.entity_id !== source.assignment_id || source.event_action !== 'manager_assigned' || source.scope_kind !== 'platform' || source.scope_id || source.visibility !== 'restricted' || source.entity_version !== source.assignment_version || source.actor_id !== source.created_by_id || !source.same_time) throw new Error('Invalid Tender assignment notice source');
    const id = randomUUID(), event = source.source_event_id, recipient = source.recipient_id, title = 'Tender assigned for evaluation', link = '/dashboard?screen=tender-fms&tender_id=' + source.tender_id;
    const at = source.at.replace(/(\.\d*?[1-9])0+Z$/, '$1Z').replace(/\.0+Z$/, 'Z');
    const saved = (await db.query(`INSERT INTO communication_notices(id,source_event_id,recipient_id,director_designation_id,director_version,decision_id,project_id,source_version,decision_action,created_at,updated_at,created_by_id,updated_by_id,notice_kind,title,link,source_action_id)
      VALUES($1,$2,$3,NULL,0,$4,NULL,$5,'assigned',$6,$6,$7,$7,'tender_manager',$8,$9,$10) ON CONFLICT(source_event_id,recipient_id) DO NOTHING RETURNING id`, [id,event,recipient,source.assignment_id,source.assignment_version,at,source.actor_id,title,link,action])).rows[0];
    if (!saved) return (await db.query('SELECT id FROM communication_notices WHERE source_event_id=$1 AND recipient_id=$2', [event,recipient])).rows[0].id as string;
    const snapshot = { notice_kind:'tender_manager', title, link, source_action_id:action, id, source_event_id:event, recipient_id:recipient, director_designation_id:'', director_version:0, decision_id:source.assignment_id, project_id:'', source_version:Number(source.assignment_version), decision_action:'assigned', version:1, is_active:true, read_at:null, acknowledged_at:null, created_at:at, updated_at:at, created_by_id:source.actor_id, updated_by_id:source.actor_id };
    const history = randomUUID();
    await db.query("INSERT INTO communication_notice_events(id,notice_id,action,notice_version,before_snapshot,after_snapshot,created_at,created_by_id)VALUES($1,$2,'delivered',1,'null'::jsonb,$3,$4,$5)", [history,id,snapshot,at,source.actor_id]);
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,causation_id,scope_kind,visibility,occurred_at)
      VALUES($1,'communication_inbox','director_notice',$2,1,0,'delivered',$3,'user',$4,$4,'platform','restricted',$5)`, [history,id,source.actor_id,event,at]);
    return id;
}
