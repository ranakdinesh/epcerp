import { AppError } from "../../errors";
import { MasterValidationError, strictObject } from "../shared/validation";
import { trim } from "../shared/text";
export const invalid = (detail: string): never => { throw new AppError(422, detail); };
export function fieldError(field: string, detail: string): never { throw new MasterValidationError(field, "body", detail); }
export const runeLength = (value: string) => [...value].length;
export const byteLength = (value: string) => Buffer.byteLength(value, "utf8");
export const uuidPattern = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i;
export function strictID(value: string, detail = "Input should be a valid UUID") { if (!uuidPattern.test(value))
    invalid(detail); return value.toLowerCase(); }
// Go's struct decoder accepts field-name casing and null as the zero value.
export function goObject(value: unknown, keys: readonly string[]): Record<string, unknown> {
    if (value == null)
        return {};
    if (typeof value !== "object" || Array.isArray(value))
        invalid("Invalid JSON or unknown field");
    const out: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
        const name = keys.find(k => k.toLowerCase() === key.toLowerCase());
        if (!name)
            invalid("Invalid JSON or unknown field");
        out[name!] = item;
    }
    return out;
}
export function text(value: unknown, fallback = "") { if (value == null)
    return fallback; if (typeof value !== "string")
    invalid("Provide supported fields with the correct value types"); return value as string; }
export function integer(value: unknown, fallback = 0) { if (value == null)
    return fallback; if (typeof value !== "number" || !Number.isInteger(value) || value < -2147483648 || value > 2147483647)
    invalid("Provide an integer within range"); return value as number; }
export function boolean(value: unknown, fallback = false) { if (value == null)
    return fallback; if (typeof value !== "boolean")
    invalid("Provide a boolean value"); return value as boolean; }
export function list(value: unknown): unknown[] { if (value == null)
    return []; if (!Array.isArray(value))
    invalid("Provide an array"); return value as unknown[]; }
export function stringMap(value: unknown, required = false) {
    if (value == null && !required)
        return Object.create(null) as Record<string, string>;
    const input = strictObject(value, Object.keys(value && typeof value === "object" ? value : {}));
    const result: Record<string, string> = Object.create(null);
    for (const [key, item] of Object.entries(input)) {
        if (typeof item !== "string")
            fieldError(key, "Provide an option code as text");
        result[key] = item as string;
    }
    return result;
}
export function reason(value: unknown, max = 2000) { const result = trim(text(value)); if (!result || runeLength(result) > max)
    fieldError("reason", `Provide a reason between 1 and ${max} characters`); return result; }
export function queryFields(q: URLSearchParams, allowed: string[]) { for (const key of q.keys())
    if (!allowed.includes(key) || q.getAll(key).length !== 1)
        invalid("Unknown or repeated query parameter"); }
export function queryInt(q: URLSearchParams, key: string, fallback: number) { const value = q.get(key); if (!value)
    return fallback; if (!/^[+-]?\d+$/.test(value))
    invalid(`Invalid ${key}`); return integer(Number(value)); }
