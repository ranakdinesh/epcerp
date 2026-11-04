import "server-only";
import type { DB } from "../db";
import { authorize } from "./repository";

// Published contract for other server modules. Credential data stays inside Auth.
export async function requirePermission(db: DB, token: string, permission: string, write = false) {
  const user = await authorize(db,token,permission,write);
  return {id:user.id,is_superuser:user.is_superuser};
}
