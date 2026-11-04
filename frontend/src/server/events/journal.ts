import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../db";
// Published native writer for Master events with no delivery targets, matching Go.
// Call only within the same transaction as the subject and its audit history.
export async function recordMasterEvent(db: DB, module: "category_catalogue" | "configuration_templates" | "catalogue_items" | "manufacturers", entityType: string, entityID: string, version: number, action: string, actor: string, id = randomUUID(), occurredAt?: string) {
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)
    VALUES($1,$2,$3,$4,$5,0,$6,$7,'user',$1,'platform','restricted',coalesce($8::timestamptz,clock_timestamp()))`, [id, module, entityType, entityID, version, action, actor, occurredAt ?? null]);
}
