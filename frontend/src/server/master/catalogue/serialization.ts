import { createHash } from "node:crypto";
// Structs must be constructed in DTO field order; map values must be sorted.
export function goJSON(value: unknown, escapeHTML = true): string {
    return JSON.stringify(value).replace(/[\u2028\u2029]/g, c => `\\u${c.charCodeAt(0).toString(16)}`).replace(/[<>&]/g, c => escapeHTML ? `\\u${c.charCodeAt(0).toString(16).padStart(4, "0")}` : c);
}
export const hash = (value: unknown) => createHash("sha256").update(goJSON(value)).digest("hex");
export function sortedMap<T>(value: Record<string, T>): Record<string, T> { return Object.fromEntries(Object.entries(value).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0)); }
export function itemHash(value: Record<string, unknown>) {
    const json = goJSON(sortedMap(value), false).replace(/[\u007f-\uffff]/g, c => `\\u${c.charCodeAt(0).toString(16).padStart(4, "0")}`);
    return createHash("sha256").update(json).digest("hex");
}
