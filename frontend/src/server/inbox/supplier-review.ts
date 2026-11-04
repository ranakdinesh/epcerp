import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../db";
// Published Inbox writer. Assignment, addressed notice, history and event share the caller's transaction.
export async function supplierReviewNotice(db: DB, event: string, recipient: string, supplier: string) {
    const source = (await db.query("SELECT *,to_char(occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD\"T\"HH24:MI:SS.US\"Z\"') AS at FROM business_events WHERE id=$1 FOR SHARE", [event])).rows[0];
    if (!source || source.source_module !== 'supplier_review_assignments' || source.entity_type !== 'supplier_review_assignment' || source.scope_kind !== 'platform' || source.scope_id || source.visibility !== 'restricted' || source.action !== 'assigned' || Number(source.entity_version) < 1)
        throw new Error('Invalid supplier review notice source');
    const id = randomUUID(), title = 'Supplier document review assigned', link = '/dashboard?screen=suppliers&supplier_id=' + supplier, at = source.at.replace(/(\.\d*?[1-9])0+Z$/, '$1Z').replace(/\.0+Z$/, 'Z');
    const saved = (await db.query(`INSERT INTO communication_notices(id,source_event_id,recipient_id,director_designation_id,director_version,decision_id,project_id,source_version,decision_action,created_at,updated_at,created_by_id,updated_by_id,notice_kind,title,link,source_action_id)
 VALUES($1,$2,$3,NULL,0,$4,NULL,$5,'assigned',$6,$6,$7,$7,'supplier_review',$8,$9,NULL) ON CONFLICT(source_event_id,recipient_id) DO NOTHING RETURNING id`, [id, event, recipient, source.entity_id, source.entity_version, at, source.actor_id, title, link])).rows[0];
    if (!saved)
        return;
    const snapshot = { notice_kind: 'supplier_review', title, link, id, source_event_id: event, recipient_id: recipient, director_designation_id: '', director_version: 0, decision_id: source.entity_id, project_id: '', source_version: Number(source.entity_version), decision_action: 'assigned', version: 1, is_active: true, read_at: null, acknowledged_at: null, created_at: at, updated_at: at, created_by_id: source.actor_id, updated_by_id: source.actor_id };
    const history = randomUUID();
    await db.query("INSERT INTO communication_notice_events(id,notice_id,action,notice_version,before_snapshot,after_snapshot,created_at,created_by_id) VALUES($1,$2,'delivered',1,'null'::jsonb,$3,$4,$5)", [history, id, snapshot, at, source.actor_id]);
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,causation_id,scope_kind,visibility,occurred_at)
 VALUES($1,'communication_inbox','director_notice',$2,1,0,'delivered',$3,'user',$4,$4,'platform','restricted',$5)`, [history, id, source.actor_id, event, at]);
}
