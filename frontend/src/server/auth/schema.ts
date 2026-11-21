import { bigint, boolean, pgSchema, text, timestamp, uuid, varchar } from "drizzle-orm/pg-core";

// SQL migrations own the schema. This mapping covers the columns read by Drizzle.
const auth = pgSchema("auth");

export const users = auth.table("users", {
  id: uuid("id").primaryKey(),
  username: varchar("username", { length: 64 }).notNull(),
  email: varchar("email", { length: 255 }).notNull(),
  password_hash: varchar("password_hash", { length: 255 }).notNull(),
  is_active: boolean("is_active").notNull(),
  must_change_password: boolean("must_change_password").notNull(),
  temporary_password_expires_at: timestamp("temporary_password_expires_at", { withTimezone: true }),
  credential_version: bigint("credential_version", { mode: "number" }).notNull(),
  updated_at: timestamp("updated_at", { withTimezone: true }).notNull(),
});

export const passwordRecovery = auth.table("auth_password_recovery", {
  tokenHash: text("token_hash").primaryKey(),
  userId: uuid("user_id").notNull(),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull(),
  expiresAt: timestamp("expires_at", { withTimezone: true }).notNull(),
  consumedAt: timestamp("consumed_at", { withTimezone: true }),
});

export const userRoles = auth.table("user_roles", {
  userId: uuid("user_id").notNull(),
  roleId: uuid("role_id").notNull(),
});
