import "server-only";
import { Pool, type PoolClient, type QueryResultRow } from "pg";
import { AppError } from "./errors";

const globalDB = globalThis as typeof globalThis & { epcerpPool?: Pool };
export function pool() {
  if (!globalDB.epcerpPool) {
    const connectionString = process.env.EPCERP_DATABASE_URL;
    if (!connectionString || !/^postgres(ql)?:\/\//.test(connectionString)) throw new Error("Configure EPCERP_DATABASE_URL for Next.js");
    // Deliberately no schema creation, ORM synchronization or automatic migrations.
    globalDB.epcerpPool = new Pool({ connectionString, max: 10, connectionTimeoutMillis: 5000,
      idleTimeoutMillis: 30000, statement_timeout: 15000, application_name: "epcerp-next" });
    globalDB.epcerpPool.on("error", () => console.error("Database connection unavailable"));
  }
  return globalDB.epcerpPool;
}
export type DB = Pick<PoolClient, "query">;
export async function rows<T extends QueryResultRow>(db: DB, sql: string, values: unknown[] = []): Promise<T[]> {
  return (await db.query<T>(sql, values)).rows;
}
export async function transaction<T>(work: (db: DB) => Promise<T>): Promise<T> {
  const db = await pool().connect();
  try {
    await db.query("BEGIN");
    const result = await work(db);
    await db.query("COMMIT");
    return result;
  } catch (error) {
    await db.query("ROLLBACK").catch(() => {});
    if ((error as { code?: string }).code === "23505") throw new AppError(409, "A record with these details already exists");
    throw error;
  } finally { db.release(); }
}
export async function lock(db: DB, key: string) { await db.query("SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))", [key]); }
export async function actor(db: DB, id: string) { await db.query("SELECT set_config('epcerp.auth_actor',$1,true)", [id]); }
