import type { DB } from '../../db';
export async function journal(db: DB, event: { id: string; module: string; type: string; entity: string; version: number; action: string; actor: string; project: string; at: string }) {
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,scope_id,visibility,occurred_at)VALUES($1,$2,$3,$4,$5,0,$6,$7,'user',$1,'project',$8,'restricted',$9)`, [event.id,event.module,event.type,event.entity,event.version,event.action,event.actor,event.project,event.at]);
}
