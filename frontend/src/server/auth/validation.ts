import { invalid, email, username } from "./domain";
export type Input = Record<string, unknown>;
export async function body(request: Request, allowed: string[], limit = 16384): Promise<Input> {
  const type = request.headers.get("content-type")?.split(";")[0].trim();
  if (type && type !== "application/json" && !/^application\/.+\+json$/.test(type)) invalid("JSON object required");
  const reader = request.body?.getReader();
  if (!reader) invalid("JSON object required");
  const chunks: Uint8Array[] = []; let size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read(); if (done) break;
      size += value.length;
      if (size > limit) { await reader.cancel(); const { AuthError } = await import("./domain"); throw new AuthError(413, "Request body is too large"); }
      chunks.push(value);
    }
  } finally { reader.releaseLock(); }
  let v: unknown;
  try { v = JSON.parse(Buffer.concat(chunks).toString("utf8")); } catch { invalid("Invalid JSON body"); }
  if (!v || typeof v !== "object" || Array.isArray(v) || Object.keys(v).some(k => !allowed.includes(k))) invalid("Invalid request fields");
  return v as Input;
}
export function text(v: Input, name: string, min: number, max: number, trim = true): string {
  const raw = v[name];
  if (typeof raw !== "string") invalid(`${name} is required`);
  const value = trim ? raw.trim() : raw;
  if ([...value].length < min || [...value].length > max) invalid(`Invalid ${name} length`);
  return value;
}
export function boolean(v: Input, name: string, fallback?: boolean): boolean {
  if (v[name] === undefined && fallback !== undefined) return fallback;
  if (typeof v[name] !== "boolean") invalid(`Use true or false for ${name}`);
  return v[name] as boolean;
}
export function version(v: Input): number | undefined {
  if (v.expected_version === undefined) return;
  if (!Number.isSafeInteger(v.expected_version) || (v.expected_version as number) < 0) invalid("Invalid expected version");
  return v.expected_version as number;
}
export function list(v: Input, name: string, min: number, max: number): string[] {
  const value = v[name];
  if (!Array.isArray(value) || value.length < min || value.length > max || value.some(v => typeof v !== "string" || v.length < 1 || v.length > 255)) invalid(`Invalid ${name}`);
  return [...new Set(value as string[])];
}
export function names(v: Input, min: number) { return [...new Set(list(v, "role_names", min, 10).map(v => v.trim().toLowerCase()))].sort(); }
export function contact(v: Input, partial = false) {
  const result: Partial<{ first_name: string; last_name: string; mobile: string }> = {};
  for (const field of ["first_name", "last_name", "mobile"] as const) {
    if (partial && v[field] === undefined) continue;
    const value = text(v, field, 1, field === "mobile" ? 24 : 100);
    if (field === "mobile") {
      const digits = value.replace(/\D/g, "");
      if (!/^\+?[0-9 ()-]+$/.test(value) || digits.length < 8 || digits.length > 15) invalid("Use 8 to 15 mobile digits, optionally with country code and spaces");
    } else if (/\p{Cc}/u.test(value)) invalid("Invalid employee name");
    result[field] = value;
  }
  return result;
}
export const address = (v: Input) => email(text(v, "email", 5, 255));
export const userName = (v: Input) => username(text(v, "username", 3, 64));
