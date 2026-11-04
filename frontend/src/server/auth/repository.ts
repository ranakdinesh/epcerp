import { randomUUID } from "node:crypto";
import { actor, lock, rows, type DB } from "../db";
import { type User, type Role, denied, forbidden, allows, temporaryValid, AuthError } from "./domain";
import { verifyAccess, sha256 } from "./security";

type UserRow = Omit<User, "roles" | "email_status">;
export async function loadRoles(db: DB, userID?: string): Promise<Role[]> {
  const result = await rows<Role>(db, `SELECT r.id,r.name,r.description,r.is_active,r.edit_version,
    COALESCE((SELECT jsonb_agg(p.code ORDER BY p.code) FROM auth.permissions p JOIN auth.role_permissions rp ON rp.permission_id=p.id WHERE rp.role_id=r.id),'[]'::jsonb) AS permissions
    FROM auth.roles r ${userID ? "JOIN auth.user_roles ur ON ur.role_id=r.id WHERE ur.user_id=$1" : ""} ORDER BY r.name`, userID ? [userID] : []);
  return result.map(r => ({ ...r, edit_version: Number(r.edit_version) }));
}
export async function loadUser(db: DB, id: string, forUpdate = false): Promise<User | null> {
  const [u] = await rows<UserRow>(db, `SELECT * FROM auth.users WHERE id=$1 ${forUpdate ? "FOR UPDATE" : ""}`, [id]);
  if (!u) return null;
  const [status] = await rows<{ state: string }>(db, "SELECT state FROM auth.auth_credential_emails WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1", [id]);
  return { ...u, credential_version: Number(u.credential_version), edit_version: Number(u.edit_version), roles: await loadRoles(db, id), email_status: status?.state ?? "" };
}
export async function resolve(db: DB, token: string, self = false, forUpdate = false): Promise<User> {
  const claims = verifyAccess(token);
  const [access] = await rows<{ revoked: boolean }>(db, "SELECT EXISTS(SELECT 1 FROM auth.access_sessions WHERE token_hash=$1 AND revoked_at IS NOT NULL) AS revoked", [sha256(token)]);
  if (access.revoked) throw denied();
  const u = await loadUser(db, claims.sub, forUpdate);
  if (!u || !u.is_active || u.credential_version !== claims.cv || !temporaryValid(u)) throw denied();
  if (!self && u.must_change_password) throw forbidden();
  return u;
}
export async function authorize(db: DB, token: string, permission: string, write = false) {
  if (write) await lock(db, "auth.administration");
  const u = await resolve(db, token);
  if (!allows(u, permission)) throw forbidden();
  if (write) await actor(db, u.id);
  return u;
}
export async function revokeUser(db: DB, id: string) {
  await db.query("UPDATE auth.refresh_tokens SET revoked_at=clock_timestamp() WHERE user_id=$1 AND revoked_at IS NULL", [id]);
  await db.query("UPDATE auth.access_sessions SET revoked_at=clock_timestamp() WHERE user_id=$1 AND revoked_at IS NULL", [id]);
}
export async function revokeFamily(db: DB, id: string) {
  await db.query("UPDATE auth.refresh_tokens SET revoked_at=clock_timestamp() WHERE family_id=$1 AND revoked_at IS NULL", [id]);
  await db.query("UPDATE auth.access_sessions SET revoked_at=clock_timestamp() WHERE family_id=$1 AND revoked_at IS NULL", [id]);
}
export async function setPassword(db: DB, id: string, hash: string, by: string, temporary = false) {
  await actor(db, by);
  await db.query(`UPDATE auth.users SET password_hash=$2,must_change_password=$4,
    temporary_password_expires_at=CASE WHEN $4 THEN clock_timestamp()+interval '24 hours' ELSE NULL END,
    credential_version=credential_version+1,updated_by_id=$3,updated_at=clock_timestamp() WHERE id=$1`, [id, hash, by, temporary]);
  await revokeUser(db, id);
}
export async function accountEvent(db: DB, id: string, by: string, action: string) {
  const [v] = await rows<{ account_version: string; updated_at: Date }>(db, "UPDATE auth.users SET account_version=account_version+1 WHERE id=$1 RETURNING account_version,updated_at", [id]);
  const eventID = randomUUID();
  await db.query(`INSERT INTO public.business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)
    VALUES($1,'auth','account',$2,$3,0,$4,$5,'user',$1,'platform','restricted',$6)`, [eventID, id, v.account_version, action, by, v.updated_at]);
  return { eventID, version: v.account_version };
}
export async function assignRoles(db: DB, id: string, names: string[], user: User, previous: string[] = []) {
  if (!user.is_superuser && names.includes("superadmin")) throw forbidden();
  const available = await loadRoles(db);
  for (const name of names) {
    const r = available.find(r => r.name === name);
    if (!r) throw new AuthError(422, "Unknown role");
    if (!r.is_active && !previous.includes(name)) throw new AuthError(422, "Inactive roles cannot be assigned");
  }
  await db.query("DELETE FROM auth.user_roles WHERE user_id=$1", [id]);
  for (const name of names) await db.query("INSERT INTO auth.user_roles(user_id,role_id) SELECT $1,id FROM auth.roles WHERE name=$2 ON CONFLICT DO NOTHING", [id, name]);
}
