import { randomUUID } from "node:crypto";
import { actor, lock, rows, transaction, type DB } from "../db";
import { denied, forbidden, identity, temporaryValid, uuid, type User, AuthError } from "./domain";
import { config, issueAccess, loginAttemptKey, opaqueToken, sha256, verifyAbsent, verifyPassword } from "./security";
import { loadUser, resolve, revokeFamily, revokeUser } from "./repository";

type Refresh = { user_id: string; family_id: string; expires_at: Date; revoked_at: Date | null };
export type TokenPair = { access_token: string; refresh_token: string; token_type: string; expires_in: number; refresh_expires_in: number; must_change_password: boolean };
async function pair(db: DB, user: User, raw: string, expiry: Date, family: string): Promise<TokenPair> {
  const now = Math.floor(Date.now()/1000), sessionExpiry = Math.floor(expiry.getTime()/1000);
  const access = issueAccess(user.id, sessionExpiry, user.credential_version, now);
  await db.query("INSERT INTO auth.access_sessions(token_hash,user_id,family_id,expires_at) VALUES($1,$2,$3,$4)", [sha256(access.token), user.id, family, new Date((now + access.seconds)*1000)]);
  return { access_token: access.token, refresh_token: raw, token_type: "bearer", expires_in: access.seconds,
    refresh_expires_in: Math.max(0, sessionExpiry-now), must_change_password: user.must_change_password };
}
async function insertRefresh(db: DB, user: string, raw: string, family: string, expiry: Date) {
  await db.query("INSERT INTO auth.refresh_tokens(id,user_id,token_hash,family_id,expires_at) VALUES($1,$2,$3,$4,$5)", [randomUUID(), user, sha256(raw), family, expiry]);
}
export async function login(identifier: string, password: string) {
  const result = await transaction(async db => {
    const users = await rows<{ id: string }>(db, "SELECT id FROM auth.users WHERE username=$1 OR email=lower($1) LIMIT 2", [identifier]);
    const found = users.length === 1 ? await loadUser(db, users[0].id) : null;
    const key = loginAttemptKey(found?.id ?? identifier.trim().toLowerCase());
    const [attempt] = await rows<{ blocked: boolean }>(db, `WITH stale AS (DELETE FROM auth.login_attempts WHERE identifier_hash IN (SELECT identifier_hash FROM auth.login_attempts WHERE window_started_at<clock_timestamp()-interval '1 day' LIMIT 100))
      INSERT INTO auth.login_attempts(identifier_hash) VALUES($1) ON CONFLICT(identifier_hash) DO UPDATE SET identifier_hash=EXCLUDED.identifier_hash
      RETURNING COALESCE(blocked_until>clock_timestamp(),false) AS blocked`, [key]);
    if (attempt.blocked) return null;
    const valid = found ? await verifyPassword(password, found.password_hash) : (await verifyAbsent(password), false);
    if (!found || !valid || !found.is_active || !temporaryValid(found)) {
      await db.query(`UPDATE auth.login_attempts SET failures=CASE WHEN window_started_at<clock_timestamp()-interval '15 minutes' THEN 1 ELSE failures+1 END,
        blocked_until=CASE WHEN window_started_at>=clock_timestamp()-interval '15 minutes' AND failures>=4 THEN clock_timestamp()+interval '15 minutes' ELSE NULL END,
        window_started_at=CASE WHEN window_started_at<clock_timestamp()-interval '15 minutes' THEN clock_timestamp() ELSE window_started_at END WHERE identifier_hash=$1`, [key]);
      return null;
    }
    const user = await loadUser(db, found.id, true);
    if (!user || !user.is_active || user.password_hash !== found.password_hash || !temporaryValid(user)) throw denied();
    const raw = opaqueToken(), family = randomUUID();
    const expiry = new Date(Date.now() + Math.min(config().sessionSeconds, user.must_change_password ? 900 : Infinity)*1000);
    await insertRefresh(db, user.id, raw, family, expiry);
    await db.query("DELETE FROM auth.login_attempts WHERE identifier_hash=$1", [key]);
    return pair(db, user, raw, expiry, family);
  });
  // Failed-attempt and replay revocation writes must commit before rejecting.
  if (!result) throw denied();
  return result;
}
export async function refresh(raw: string) {
  const result = await transaction(async db => {
    const [initial] = await rows<Refresh>(db, "SELECT * FROM auth.refresh_tokens WHERE token_hash=$1", [sha256(raw)]);
    if (!initial) return null;
    const user = await loadUser(db, initial.user_id, true);
    await lock(db, `auth.refresh.family:${initial.family_id}`);
    const [token] = await rows<Refresh>(db, "SELECT * FROM auth.refresh_tokens WHERE token_hash=$1 FOR UPDATE", [sha256(raw)]);
    if (!token || token.expires_at.getTime() <= Date.now() || !user?.is_active || !temporaryValid(user)) return null;
    if (token.revoked_at) { await revokeFamily(db, token.family_id); return null; }
    const replacement = opaqueToken();
    await db.query("UPDATE auth.refresh_tokens SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL", [sha256(raw)]);
    await insertRefresh(db, user.id, replacement, token.family_id, token.expires_at);
    return pair(db, user, replacement, token.expires_at, token.family_id);
  });
  if (!result) throw denied();
  return result;
}
export async function logout(raw: string) {
  return transaction(async db => {
    const [token] = await rows<Refresh>(db, "SELECT * FROM auth.refresh_tokens WHERE token_hash=$1", [sha256(raw)]);
    if (!token) return;
    await loadUser(db, token.user_id, true);
    await lock(db, `auth.refresh.family:${token.family_id}`);
    await revokeFamily(db, token.family_id);
  });
}
export function me(token: string) { return transaction(async db => identity(await resolve(db, token, true))); }
export function activity(token: string, page: string) {
  return transaction(async db => {
    const u = await resolve(db, token);
    await db.query("INSERT INTO auth.user_activity(user_id,current_page,last_seen_at) VALUES($1,$2,now()) ON CONFLICT(user_id) DO UPDATE SET current_page=excluded.current_page,last_seen_at=excluded.last_seen_at", [u.id, page]);
  });
}
export function sessions(token: string) {
  return transaction(async db => {
    const u = await resolve(db, token);
    return rows(db, `SELECT family_id AS id,min(created_at) AS created_at,max(expires_at) AS expires_at,
      family_id=(SELECT family_id FROM auth.access_sessions WHERE token_hash=$2) AS current
      FROM auth.refresh_tokens WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp() GROUP BY family_id ORDER BY min(created_at) DESC`, [u.id, sha256(token)]);
  });
}
export function revokeSessions(token: string, family?: string) {
  return transaction(async db => {
    const u = await resolve(db, token, false, true);
    await actor(db, u.id);
    if (!family) return revokeUser(db, u.id);
    uuid(family);
    const [owner] = await rows<{ user_id: string }>(db, "SELECT user_id FROM auth.refresh_tokens WHERE family_id=$1 LIMIT 1", [family]);
    if (!owner) throw new AuthError(404, "User not found");
    if (owner.user_id !== u.id) throw forbidden();
    await lock(db, `auth.refresh.family:${family}`);
    await revokeFamily(db, family);
  });
}
