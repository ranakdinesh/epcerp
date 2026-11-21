import { boolean, numeric, pgSchema, timestamp, uuid, varchar } from "drizzle-orm/pg-core";

// SQL migrations own the database schema. These mappings cover summary reads.
const tender = pgSchema("tender");

export const documentDrafts = tender.table("document_drafts", {
  id: uuid("id").primaryKey(),
  isActive: boolean("is_active").notNull(),
  ownerId: uuid("owner_id").notNull(),
  createdById: uuid("created_by_id"),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull(),
});

export const biddingAuthorizations = tender.table("bidding_authorizations", {
  draftId: uuid("draft_id").notNull().unique(),
  decision: varchar("decision", { length: 32 }).notNull(),
  estimatedValue: numeric("estimated_value", { precision: 18, scale: 2 }),
});

export const bidCostings = tender.table("bid_costings", {
  draftId: uuid("draft_id").notNull().unique(),
  status: varchar("status", { length: 32 }).notNull(),
  totalBidValue: numeric("total_bid_value", { precision: 18, scale: 2 }).notNull(),
});
