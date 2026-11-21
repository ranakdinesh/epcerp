import 'server-only';
import { and, desc, eq, sql } from 'drizzle-orm';
import { drizzle } from 'drizzle-orm/node-postgres';
import { bigint, pgTable, text, timestamp, uuid } from 'drizzle-orm/pg-core';
import type { PoolClient } from 'pg';
import { users } from '../auth/schema';

// Partitioning and constraints are owned by the SQL migration.
const activityEvents = pgTable('activity_events', {
    scopeKind: text('scope_kind').notNull(),
    scopeId: uuid('scope_id').notNull(),
    id: uuid('id').notNull(),
    module: text('module').notNull(),
    action: text('action').notNull(),
    actorId: uuid('actor_id'),
    actorKind: text('actor_kind').notNull(),
    occurredAt: timestamp('occurred_at', { withTimezone: true }).notNull().default(sql`clock_timestamp()`),
    sourceEventId: uuid('source_event_id'),
    entityId: uuid('entity_id'),
    entityVersion: bigint('entity_version', { mode: 'number' }),
    reason: text('reason'),
});

type PurchaseActivity = {
    project: string;
    id: string;
    action: 'purchase_started' | 'requirements_published' | 'item_list_opened';
    actor: string;
    source?: { entity: string; version: number; at: string; reason: string };
};

export async function recordPurchaseActivity(client: PoolClient, event: PurchaseActivity) {
    await drizzle({ client }).insert(activityEvents).values({
        scopeKind: 'project', scopeId: event.project, module: 'purchase',
        id: event.id, action: event.action, actorKind: 'user', actorId: event.actor,
        sourceEventId: event.source ? event.id : undefined,
        entityId: event.source?.entity, entityVersion: event.source?.version,
        occurredAt: event.source ? sql`${event.source.at}::timestamptz` : undefined,
        reason: event.source?.reason,
    }).onConflictDoNothing();
}

export function recentPurchaseActivity(client: PoolClient, project: string) {
    return drizzle({ client }).select({
        id: activityEvents.id, action: activityEvents.action, actorId: activityEvents.actorId,
        actorKind: activityEvents.actorKind, actorUsername: users.username, occurredAt: activityEvents.occurredAt,
        entityId: activityEvents.entityId, entityVersion: activityEvents.entityVersion,
        reason: activityEvents.reason,
    }).from(activityEvents).leftJoin(users, eq(users.id, activityEvents.actorId)).where(and(eq(activityEvents.scopeKind, 'project'),
        eq(activityEvents.scopeId, project), eq(activityEvents.module, 'purchase')))
        .orderBy(desc(activityEvents.occurredAt), desc(activityEvents.id), activityEvents.action).limit(20);
}
