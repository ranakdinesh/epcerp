import { actor, lock, rows, transaction, type DB } from "../db";
import { AuthError, forbidden, invalid, strongPassword, username, type User } from "./domain";
import { accountEvent, authorize, loadUser, resolve, revokeUser, setPassword } from "./repository";
import { hashPassword, opaqueToken, sha256, verifyPassword } from "./security";
import { publicURL, requireMail, sendEmail } from "./mail";

export async function stageEmail(db: DB, u: User, recipient: string, by: string) {
  requireMail();
  if (recipient === u.email) invalid("Enter a different email address.");
  await db.query("UPDATE auth.email_changes SET consumed_at=clock_timestamp() WHERE user_id=$1 AND consumed_at IS NULL", [u.id]);
  await db.query("UPDATE auth.auth_credential_emails SET state='cancelled',payload=NULL,updated_at=clock_timestamp() WHERE user_id=$1 AND state IN ('pending','sending')", [u.id]);
  const raw = opaqueToken();
  await db.query(`INSERT INTO auth.email_changes(token_hash,user_id,email,previous_email,credential_version,created_by_id,expires_at)
    VALUES($1,$2,$3,$4,$5,$6,clock_timestamp()+interval '30 minutes')`, [sha256(raw), u.id, recipient, u.email, u.credential_version, by]);
  return raw;
}
export function sendVerification(recipient: string, raw: string) {
  return sendEmail(recipient, "Verify your new epcERP email address", `An email address change was requested for your epcERP account. Confirm this address using the link below. Your current sign-in email remains unchanged until you confirm.\n\n${publicURL()}/verify-email#token=${raw}\n\nThis link expires in 30 minutes and can be used once. Ignore it if you did not request the change.`);
}
export async function changeAccount(token: string, current: string, change: { password: string } | { email: string }) {
  if ("password" in change && !strongPassword(change.password)) invalid("Use 8 to 128 characters including uppercase, lowercase, number and symbol.");
  const verification = await transaction(async db => {
    // Match administrative lock order before locking the account row.
    await lock(db, "auth.administration");
    const u = await resolve(db, token, true, true);
    if (u.must_change_password && !("password" in change)) throw forbidden();
    if (!await verifyPassword(current, u.password_hash)) invalid("Current password is incorrect");
    await actor(db, u.id);
    if ("password" in change) {
      if (change.password === current) invalid("Choose a different password");
      await setPassword(db, u.id, await hashPassword(change.password), u.id);
      await accountEvent(db, u.id, u.id, "password_changed");
      return null;
    }
    const raw = await stageEmail(db, u, change.email, u.id);
    await accountEvent(db, u.id, u.id, "email_verification_requested");
    return { recipient: change.email, raw };
  });
  if (verification) await sendVerification(verification.recipient, verification.raw);
}
export function verifyEmail(raw: string) {
  if (raw.length !== 64) invalid("This verification link is invalid or expired. Request a new link.");
  return transaction(async db => {
    await lock(db, "auth.administration");
    const [c] = await rows<{ user_id: string; created_by_id: string; email: string; previous_email: string; credential_version: string; consumed_at: Date | null; expires_at: Date }>(db,
      "SELECT * FROM auth.email_changes WHERE token_hash=$1 FOR UPDATE", [sha256(raw)]);
    if (!c || c.consumed_at || c.expires_at.getTime() <= Date.now()) invalid("This verification link is invalid or expired. Request a new link.");
    const u = await loadUser(db, c.user_id, true);
    if (!u?.is_active || u.credential_version !== Number(c.credential_version) || u.email !== c.previous_email) invalid("This verification link is invalid or expired. Request a new link.");
    await actor(db, c.created_by_id);
    await db.query("UPDATE auth.users SET email=$2,updated_at=now(),updated_by_id=$3 WHERE id=$1", [u.id, c.email, c.created_by_id]);
    await db.query("UPDATE auth.email_changes SET consumed_at=clock_timestamp() WHERE user_id=$1 AND consumed_at IS NULL", [u.id]);
    await revokeUser(db, u.id);
    await accountEvent(db, u.id, u.id, "email_verified");
  });
}
export function usernameAvailable(token: string, name: string) {
  name = username(name);
  return transaction(async db => {
    const u = await authorize(db, token, "account.profile.write");
    const [result] = await rows<{ available: boolean }>(db, "SELECT NOT EXISTS(SELECT 1 FROM auth.users WHERE username=$1 AND id<>$2) AS available", [name, u.id]);
    return result;
  });
}
export function changeUsername(token: string, name: string, expected: string) {
  name = username(name);
  return transaction(async db => {
    const by = await authorize(db, token, "account.profile.write", true), u = await loadUser(db, by.id, true);
    if (!u) throw new AuthError(401, "Invalid or expired credentials");
    if (u.username !== expected) throw new AuthError(409, "Your username changed. Close and reopen your profile before saving.");
    if (u.username === name) return;
    await db.query("UPDATE auth.users SET username=$2,updated_at=clock_timestamp(),updated_by_id=$1 WHERE id=$1", [u.id, name]);
    const e = await accountEvent(db, u.id, u.id, "username_changed");
    await db.query("INSERT INTO auth.auth_username_changes(id,user_id,version,previous_username,username,created_by_id) VALUES($1,$2,$3,$4,$5,$2)", [e.eventID, u.id, e.version, expected, name]);
  });
}
export async function requestRecovery(address: string) {
  requireMail();
  const raw = opaqueToken();
  const recipient = await transaction(async db => {
    const matches = await rows<{ id: string }>(db, "SELECT id FROM auth.users WHERE lower(email)=lower($1) LIMIT 2", [address]);
    if (matches.length !== 1) return null;
    const u = await loadUser(db, matches[0].id, true);
    if (!u?.is_active || u.email.toLowerCase() !== address.toLowerCase()) return null;
    const [recent] = await rows<{ found: boolean }>(db, "SELECT EXISTS(SELECT 1 FROM auth.auth_password_recovery WHERE user_id=$1 AND created_at>clock_timestamp()-interval '1 minute') AS found", [u.id]);
    if (recent.found) return null;
    await db.query("UPDATE auth.auth_password_recovery SET consumed_at=clock_timestamp() WHERE user_id=$1 AND consumed_at IS NULL", [u.id]);
    await db.query("INSERT INTO auth.auth_password_recovery(token_hash,user_id,expires_at) VALUES($1,$2,clock_timestamp()+interval '15 minutes')", [sha256(raw), u.id]);
    return u.email;
  });
  if (recipient) await sendEmail(recipient, "Reset your epcERP password", `A password reset was requested for your epcERP account. Open the secure link below to choose a new password.\n\n${publicURL()}/reset-password#token=${raw}\n\nThis link expires in 15 minutes and can be used once. If you did not request this, ignore this email. Your password has not changed.`);
}
export function completeRecovery(raw: string, password: string) {
  const bad = () => invalid("This reset link is invalid or expired. Request a new link.");
  if (raw.length !== 64) bad();
  if (!strongPassword(password)) invalid("Use 8 to 128 characters including uppercase, lowercase, number and symbol.");
  return transaction(async db => {
    const [initial] = await rows<{ user_id: string }>(db, "SELECT user_id FROM auth.auth_password_recovery WHERE token_hash=$1", [sha256(raw)]);
    if (!initial) bad();
    const u = await loadUser(db, initial.user_id, true);
    if (!u?.is_active) return bad();
    const [valid] = await rows(db, `SELECT t.token_hash FROM auth.auth_password_recovery t JOIN auth.users u ON u.id=t.user_id
      WHERE t.token_hash=$1 AND t.consumed_at IS NULL AND t.expires_at>clock_timestamp() AND u.is_active AND u.updated_at<=t.created_at FOR UPDATE OF t`, [sha256(raw)]);
    if (!valid) bad();
    if (await verifyPassword(password, u.password_hash)) invalid("Choose a different password.");
    await setPassword(db, u.id, await hashPassword(password), u.id);
    await db.query("UPDATE auth.auth_password_recovery SET consumed_at=clock_timestamp() WHERE user_id=$1 AND consumed_at IS NULL", [u.id]);
    await accountEvent(db, u.id, u.id, "password_reset");
  });
}
