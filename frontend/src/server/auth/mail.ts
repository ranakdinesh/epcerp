import "server-only";
import { createCipheriv, createDecipheriv, createHmac, randomBytes, randomUUID } from "node:crypto";
import { rows, transaction, type DB } from "../db";
import { AuthError } from "./domain";
import { sha256 } from "./security";

export function publicURL() {
  const value = process.env.EPCERP_PUBLIC_URL ?? "https://erp.vishaltechnopower.in";
  const u = new URL(value);
  const local = ["development", "test"].includes(process.env.EPCERP_ENVIRONMENT ?? "") && u.protocol === "http:" && ["localhost", "127.0.0.1"].includes(u.hostname);
  if ((!local && u.protocol !== "https:") || u.username || u.password || u.search || u.hash || !["", "/"].includes(u.pathname)) throw new Error("Invalid EPCERP_PUBLIC_URL");
  return u.origin;
}
export function mailReady() {
  return process.env.EPCERP_EMAIL_DELIVERY_ENABLED === "true" && !!process.env.EPCERP_BREVO_API_KEY && !!(process.env.EPCERP_EMAIL_SENDER_EMAIL || process.env.EPCERP_RECOVERY_SENDER_EMAIL);
}
export function requireMail() {
  if (!mailReady()) throw new AuthError(503, "Email delivery is not enabled or configured. Contact your administrator.");
}
function key() {
  const raw = process.env.EPCERP_IMAGE_ENCRYPTION_KEY ?? "";
  if (!/^[a-f0-9]{64}$/i.test(raw)) throw new Error("Invalid credential email encryption key");
  return createHmac("sha256", Buffer.from(raw, "hex")).update("epcERP/credential-email/v1").digest();
}
// Go AES-GCM envelope: 12-byte nonce || ciphertext || 16-byte authentication tag.
export function sealMail(plain: Buffer) {
  const nonce = randomBytes(12), cipher = createCipheriv("aes-256-gcm", key(), nonce);
  return { payload: Buffer.concat([nonce, cipher.update(plain), cipher.final(), cipher.getAuthTag()]), hash: sha256(plain) };
}
export function openMail(payload: Buffer, hash: string) {
  if (payload.length < 28) throw new Error("Invalid mail envelope");
  const cipher = createDecipheriv("aes-256-gcm", key(), payload.subarray(0,12));
  cipher.setAuthTag(payload.subarray(-16));
  const plain = Buffer.concat([cipher.update(payload.subarray(12,-16)), cipher.final()]);
  if (sha256(plain) !== hash) throw new Error("Invalid mail integrity");
  return plain;
}
const escapeHTML = (s: string) => s.replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);
export async function sendEmail(recipient: string, subject: string, message: string) {
  requireMail();
  const sender = process.env.EPCERP_EMAIL_SENDER_EMAIL || process.env.EPCERP_RECOVERY_SENDER_EMAIL!;
  const response = await fetch("https://api.brevo.com/v3/smtp/email", {
    method: "POST", redirect: "error", signal: AbortSignal.timeout(8000),
    headers: { "api-key": process.env.EPCERP_BREVO_API_KEY!, "Content-Type": "application/json" },
    body: JSON.stringify({ sender: { email: sender, name: process.env.EPCERP_EMAIL_SENDER_NAME || process.env.EPCERP_RECOVERY_SENDER_NAME || "Vishal Technopower | epcERP" },
      to: [{ email: recipient }], subject, textContent: message,
      htmlContent: `<!doctype html><html><body style="font-family:Arial,sans-serif;color:#0f172a"><h2>Vishal Technopower · epcERP</h2><h3>${escapeHTML(subject)}</h3><div style="white-space:pre-wrap;line-height:1.7">${escapeHTML(message)}</div><p>Keep your credentials private.</p></body></html>` }),
  });
  await response.body?.cancel();
  if (!response.ok) throw new AuthError(503, "Email delivery failed. Please retry.");
}
export async function cancelStaleMail(db: DB, userID?: string) {
  await db.query(`UPDATE auth.auth_credential_emails e SET state=CASE WHEN e.attempts>=5 THEN 'failed' ELSE 'cancelled' END,payload=NULL,updated_at=clock_timestamp(),updated_by_id=NULL
    WHERE ($1::uuid IS NULL OR e.user_id=$1) AND e.state IN ('pending','sending') AND (e.expires_at<=clock_timestamp() OR (e.attempts>=5 AND e.state='sending' AND e.lease_until<clock_timestamp()) OR NOT EXISTS
      (SELECT 1 FROM auth.users u WHERE u.id=e.user_id AND u.is_active AND u.must_change_password AND u.credential_version=e.credential_version))`, [userID ?? null]);
}
export type CredentialMail = { ID: string; UserID: string; Recipient: string; Subject: string; Message: string };
export async function queueCredentialMail(db: DB, mail: CredentialMail, version: number, expiry: Date, by: string) {
  const sealed = sealMail(Buffer.from(JSON.stringify(mail)));
  await cancelStaleMail(db);
  await db.query(`INSERT INTO auth.auth_credential_emails(id,user_id,credential_version,payload,payload_hash,expires_at,created_by_id,updated_by_id)
    VALUES($1,$2,$3,$4,$5,$6,$7,$7)`, [mail.ID, mail.UserID, version, sealed.payload, sealed.hash, expiry, by]);
}
export async function deliverCredentialMail() {
  if (!mailReady()) return false;
  const lease = randomUUID();
  const job = await transaction(async db => {
    await cancelStaleMail(db);
    const [job] = await rows<{ id: string; user_id: string; credential_version: string; payload: Buffer; payload_hash: string }>(db, `UPDATE auth.auth_credential_emails SET state='sending',attempts=attempts+1,lease_id=$1,lease_until=clock_timestamp()+interval '60 seconds',updated_at=clock_timestamp(),updated_by_id=NULL
      WHERE id=(SELECT e.id FROM auth.auth_credential_emails e JOIN auth.users u ON u.id=e.user_id WHERE u.is_active AND u.must_change_password AND u.credential_version=e.credential_version
        AND e.expires_at>clock_timestamp() AND e.attempts<5 AND ((e.state='pending' AND e.next_attempt_at<=clock_timestamp()) OR (e.state='sending' AND e.lease_until<clock_timestamp()))
        ORDER BY e.created_at LIMIT 1 FOR UPDATE OF e SKIP LOCKED) RETURNING *`, [lease]);
    return job;
  });
  if (!job) return false;
  let delivered = false;
  try {
    const mail: CredentialMail = JSON.parse(openMail(job.payload, job.payload_hash).toString());
    const valid = await transaction(async db => {
      const [u] = await rows(db, "SELECT email FROM auth.users WHERE id=$1 AND is_active AND must_change_password AND credential_version=$2", [job.user_id, job.credential_version]);
      return mail.ID === job.id && mail.UserID === job.user_id && u?.email === mail.Recipient;
    });
    if (!valid) {
      await transaction(db => db.query("UPDATE auth.auth_credential_emails SET state='cancelled',payload=NULL,updated_at=clock_timestamp(),updated_by_id=NULL WHERE id=$1 AND lease_id=$2 AND state='sending'", [job.id, lease]));
      return true;
    }
    await sendEmail(mail.Recipient, mail.Subject, mail.Message);
    delivered = true;
  } catch { console.error("Credential email delivery was not accepted"); }
  await transaction(db => db.query(`UPDATE auth.auth_credential_emails SET state=CASE WHEN $3 THEN 'delivered' WHEN attempts>=5 OR expires_at<=clock_timestamp() THEN 'failed' ELSE 'pending' END,
    payload=CASE WHEN $3 OR attempts>=5 OR expires_at<=clock_timestamp() THEN NULL ELSE payload END,lease_until=NULL,next_attempt_at=clock_timestamp()+interval '1 minute'*attempts,updated_at=clock_timestamp(),updated_by_id=NULL
    WHERE id=$1 AND lease_id=$2 AND state='sending'`, [job.id, lease, delivered]));
  return true;
}
const runtime = globalThis as typeof globalThis & { authMailTimer?: ReturnType<typeof setInterval> };
export function startCredentialMail() {
  if (runtime.authMailTimer || !mailReady()) return;
  let busy = false;
  runtime.authMailTimer = setInterval(async () => {
    if (busy) return;
    busy = true;
    try { await deliverCredentialMail(); } catch { console.error("Credential email worker unavailable"); } finally { busy = false; }
  }, 2000);
  runtime.authMailTimer.unref();
}
