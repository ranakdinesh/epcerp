import "server-only";
import { access } from "./intake/access";
import { and, eq, gte, lt, sql } from "drizzle-orm";
import { drizzle } from "drizzle-orm/node-postgres";
import { transaction } from "../db";
import { biddingAuthorizations, bidCostings, documentDrafts } from "./schema";

export type TenderSummary = {
  year: number;
  processed: number;
  cleared: number;
  won: number;
  lost: number;
  estimated: number;
  processed_value: string | null;
  won_value: string | null;
  lost_value: string | null;
};

export function tenderSummary(token: string): Promise<TenderSummary> {
  return transaction(async db => {
    const actor = await access(db, token, "draft");
    const portfolio = actor.superuser || actor.allows("system:admin");
    const start = sql`date_trunc('year', now() AT TIME ZONE 'Asia/Kolkata')`;
    const [summary] = await drizzle({ client: db }).select({
      year: sql<number>`extract(year FROM ${start})::int`,
      processed: sql<number>`count(*)::int`,
      cleared: sql<number>`count(*) FILTER (WHERE ${biddingAuthorizations.decision} = 'authorized')::int`,
      won: sql<number>`count(*) FILTER (WHERE ${bidCostings.status} = 'awarded')::int`,
      lost: sql<number>`count(*) FILTER (WHERE ${bidCostings.status} = 'lost')::int`,
      estimated: sql<number>`count(${biddingAuthorizations.estimatedValue})::int`,
      processed_value: sql<string | null>`sum(${biddingAuthorizations.estimatedValue})::text`,
      won_value: sql<string | null>`(sum(${bidCostings.totalBidValue}) FILTER (WHERE ${bidCostings.status} = 'awarded'))::text`,
      lost_value: sql<string | null>`(sum(${bidCostings.totalBidValue}) FILTER (WHERE ${bidCostings.status} = 'lost'))::text`,
    }).from(documentDrafts)
      .leftJoin(biddingAuthorizations, eq(biddingAuthorizations.draftId, documentDrafts.id))
      .leftJoin(bidCostings, eq(bidCostings.draftId, documentDrafts.id))
      .where(and(
        eq(documentDrafts.isActive, true),
        portfolio ? undefined : sql`(${documentDrafts.ownerId} = ${actor.id} OR ${documentDrafts.createdById} = ${actor.id})`,
        gte(documentDrafts.createdAt, sql`(${start} AT TIME ZONE 'Asia/Kolkata')`),
        lt(documentDrafts.createdAt, sql`((${start} + interval '1 year') AT TIME ZONE 'Asia/Kolkata')`),
      ));
    return summary;
  });
}
