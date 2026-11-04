import { AppError } from "../../errors";
import { trim } from "./text";
export class MasterValidationError extends AppError {
    constructor(public field: string, public source: "body" | "query" | "path" = "body", message = "Invalid or missing value") { super(422, message); }
}
export function bad(field: string, message?: string): never { throw new MasterValidationError(field, "body", message); }
export function canonicalID(raw: unknown, field = "id", source: "body" | "query" | "path" = "body"): string {
    if (typeof raw !== "string")
        throw new MasterValidationError(field, source);
    let value = raw.replace(/^\{/u, "").replace(/\}$/u, "").replace(/^urn:uuid:/iu, "");
    if (!/^[a-f0-9]{8}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{12}$/iu.test(value))
        throw new MasterValidationError(field, source);
    value = value.replace(/-/g, "").toLowerCase();
    return `${value.slice(0, 8)}-${value.slice(8, 12)}-${value.slice(12, 16)}-${value.slice(16, 20)}-${value.slice(20)}`;
}
export function strictObject(value: unknown, allowed: readonly string[], field = ""): Record<string, unknown> {
    if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).some(key => !allowed.includes(key)))
        bad(field);
    return value as Record<string, unknown>;
}
export function stringField(input: Record<string, unknown>, field: string, min: number, max: number, fallback?: string) {
    const raw = input[field] === undefined ? fallback : input[field];
    if (typeof raw !== "string")
        bad(field);
    const value = trim(raw);
    if ([...value].length < min || [...value].length > max)
        bad(field);
    return value;
}
