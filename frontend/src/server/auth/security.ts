import "server-only";
import { createHash, createHmac, randomBytes, randomUUID, timingSafeEqual } from "node:crypto";
import argon2 from "argon2";
import { denied, uuid } from "./domain";

export const sha256 = (value: string | Buffer) => createHash("sha256").update(value).digest("hex");
export const loginAttemptKey = (identifier: string) => sha256(`login:${identifier}`);
export const opaqueToken = () => randomBytes(48).toString("base64url");
function setting(name: string, fallback: number, min: number, max: number) {
  const raw = process.env[name];
  const n = raw === undefined ? fallback : /^\d+$/.test(raw) ? Number(raw) : NaN;
  if (!Number.isInteger(n) || n < min || n > max) throw new Error(`Invalid ${name}`);
  return n;
}
export function config() {
  const secret = process.env.VTPL_JWT_SECRET_KEY ?? "";
  const issuer = process.env.VTPL_JWT_ISSUER ?? "epcERP", audience = process.env.VTPL_JWT_AUDIENCE ?? "epcERP-api";
  if (Buffer.byteLength(secret) < 32 || issuer.length < 3 || issuer.length > 255 || audience.length < 3 || audience.length > 255) throw new Error("Invalid auth signing configuration");
  return { secret, issuer, audience, accessSeconds: setting("VTPL_ACCESS_TOKEN_MINUTES", 15, 1, 60) * 60,
    sessionSeconds: setting("VTPL_SESSION_HOURS", 12, 1, 12) * 3600,
    presenceSeconds: setting("VTPL_USER_PRESENCE_TIMEOUT_SECONDS", 300, 30, 3600) };
}
export type Claims = { sub: string; iss: string; aud: string | string[]; iat: number; nbf: number; exp: number; jti: string; typ: string; cv: number };
export function issueAccess(id: string, sessionExpiry: number, version: number, now = Math.floor(Date.now()/1000)) {
  const c = config(), expiry = Math.min(sessionExpiry, now + c.accessSeconds);
  if (expiry <= now) throw denied();
  const head = Buffer.from(JSON.stringify({ alg: "HS256", typ: "JWT" })).toString("base64url");
  const body = Buffer.from(JSON.stringify({ sub: id, iss: c.issuer, aud: c.audience, iat: now, nbf: now, exp: expiry, jti: randomUUID(), typ: "access", cv: version })).toString("base64url");
  const data = `${head}.${body}`;
  return { token: `${data}.${createHmac("sha256", c.secret).update(data).digest("base64url")}`, seconds: expiry - now };
}
export function verifyAccess(raw: string, now = Math.floor(Date.now()/1000)): Claims {
  try {
    if (raw.length > 8192) throw denied();
    const parts = raw.split(".");
    if (parts.length !== 3 || parts.some(p => !/^[A-Za-z0-9_-]+$/.test(p))) throw denied();
    const c = config(), header = JSON.parse(Buffer.from(parts[0], "base64url").toString());
    if (header.alg !== "HS256" || header.crit !== undefined) throw denied();
    const signature = Buffer.from(parts[2], "base64url"), expected = createHmac("sha256", c.secret).update(`${parts[0]}.${parts[1]}`).digest();
    if (signature.length !== expected.length || !timingSafeEqual(signature, expected)) throw denied();
    const v = JSON.parse(Buffer.from(parts[1], "base64url").toString());
    if (v.iss !== c.issuer || !(v.aud === c.audience || Array.isArray(v.aud) && v.aud.includes(c.audience)) || v.typ !== "access" ||
      typeof v.sub !== "string" || typeof v.jti !== "string" || !v.jti ||
      ![v.exp,v.iat,v.nbf].every(n => typeof n === "number" && Number.isFinite(n)) || v.exp <= now || v.iat > now || v.nbf > now ||
      !Number.isSafeInteger(v.cv ?? 0) || (v.cv ?? 0) < 0) throw denied();
    return { ...v, sub: uuid(v.sub), cv: v.cv ?? 0 };
  } catch { throw denied(); }
}

// Match Go's bounded PHC verifier. Untrusted hashes cannot request unlimited RAM.
export function validPHC(encoded: string) {
  if (encoded.length > 255) return false;
  const m = /^\$argon2(id|i)\$v=19\$m=(\d+),t=(\d+),p=(\d+)\$([A-Za-z0-9+/]+)\$([A-Za-z0-9+/]+)$/.exec(encoded);
  if (!m) return false;
  const memory = Number(m[2]), time = Number(m[3]), parallel = Number(m[4]);
  const salt = Buffer.from(m[5], "base64"), hash = Buffer.from(m[6], "base64");
  return parallel >= 1 && parallel <= 16 && memory >= 8 * parallel && memory <= 262144 && time >= 1 && time <= 10 &&
    salt.length >= 8 && salt.length <= 64 && hash.length >= 16 && hash.length <= 64 &&
    salt.toString("base64").replace(/=+$/, "") === m[5] && hash.toString("base64").replace(/=+$/, "") === m[6];
}
let running = 0;
const waiting: Array<() => void> = [];
async function bounded<T>(work: () => Promise<T>) {
  if (running >= 4) {
    if (waiting.length >= 32) throw new Error("Password verification busy");
    await new Promise<void>(resolve => waiting.push(resolve));
  } else running++;
  try { return await work(); }
  finally { const next = waiting.shift(); if (next) next(); else running--; }
}
export function hashPassword(password: string) {
  return bounded(() => argon2.hash(password, { type: argon2.argon2id, version: 0x13, memoryCost: 65536, timeCost: 3, parallelism: 4, hashLength: 32, salt: randomBytes(16) }));
}
export async function verifyPassword(password: string, encoded: string) {
  if (!validPHC(encoded)) return false;
  return bounded(() => argon2.verify(encoded, password).catch(() => false));
}
let dummy: Promise<string> | undefined;
export async function verifyAbsent(password: string) { dummy ??= hashPassword(opaqueToken()); await verifyPassword(password, await dummy); }
