import { AppError } from "../../errors";
import { goJSON } from "../catalogue/serialization";
import { boolean, byteLength, fieldError, goObject, integer, invalid, runeLength, text } from "../catalogue/validation";
import { sourceTimestamp } from "../categories/domain";
import { trim, upper } from "../shared/text";
import { createHash } from "node:crypto";
export type Vendor = {
    id: string;
    code: string;
    name: string;
    data: Record<string, unknown>;
    is_active: boolean;
    version: number;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
    is_imported: boolean;
};
export type VendorSource = {
    id: string;
    version: number;
    created_at: string;
    updated_at: string;
};
export type StoredVendor = Vendor & {
    source_id: string | null;
    source_version: number | null;
    source_created_at: string | null;
    source_updated_at: string | null;
};
export const vendorID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const fields: Record<string, number> = { registration_reference: 120, contact: 255, location: 255, contact_person_name: 255, email: 255, phone: 50, aadhar_number: 50, pan_number: 50, gst_number: 50, cancelled_cheque_ref: 120, notes: 2000, indent_notes: 4000, indent_terms: 12000, delivery_terms: 2000 };
function object(value: unknown, message: string): Record<string, unknown> { if (!value || typeof value !== "object" || Array.isArray(value) || (JSON as typeof JSON & {
    isRawJSON: (value: unknown) => boolean;
}).isRawJSON(value))
    invalid(message); return value as Record<string, unknown>; }
function numberToken(value: unknown) { if (typeof value === "number" && Number.isFinite(value))
    return String(value); if ((JSON as typeof JSON & {
    isRawJSON: (value: unknown) => boolean;
}).isRawJSON(value))
    return (value as {
        rawJSON: string;
    }).rawJSON; return null; }
function decimal(value: unknown): {
    n: bigint;
    scale: number;
} | null {
    const token = numberToken(value);
    if (token === null)
        return null;
    const m = /^(-?)(\d+)(?:\.(\d+))?(?:[eE]([+-]?\d+))?$/.exec(token);
    if (!m)
        return null;
    const scale = BigInt(m[3]?.length ?? 0) - BigInt(m[4] ?? 0);
    // Match math/big.Rat's decimal exponent bound without allocating unbounded powers.
    if (scale < -BigInt("1000000") || scale > BigInt("1000000"))
        return null;
    let n = BigInt(m[2] + (m[3] ?? ""));
    if (m[1])
        n = -n;
    if (scale < BigInt("0"))
        return { n: n * BigInt("10") ** (-scale), scale: 0 };
    return { n, scale: Number(scale) };
}
export function validatePaymentTerms(value: unknown) {
    const terms = object(value, "Invalid payment terms");
    let total = BigInt("0"), scale = 0;
    for (const [key, value] of Object.entries(terms)) {
        if (["advance_pct", "qc_pct", "delivery_pct", "retention_pct"].includes(key)) {
            if (typeof value === "string")
                invalid("Payment values must be numbers");
            const d = decimal(value);
            if (!d || d.n < BigInt("0") || d.n > BigInt("100") * BigInt("10") ** BigInt(d.scale))
                invalid("Invalid payment percentage");
            const next = Math.max(scale, d!.scale);
            total = total * BigInt("10") ** BigInt(next - scale) + d!.n * BigInt("10") ** BigInt(next - d!.scale);
            scale = next;
        }
        else if (key === "credit_days") {
            if (typeof value === "string")
                invalid("Payment values must be numbers");
            const d = decimal(value);
            if (!d)
                invalid("Invalid credit days");
            const denominator = BigInt("10") ** BigInt(d!.scale);
            if (d!.n < BigInt("0") || d!.n > BigInt("365") * denominator || d!.n % denominator !== BigInt("0"))
                invalid("Invalid credit days");
        }
        else if (key === "description") {
            if (typeof value !== "string" || runeLength(value) > 500)
                invalid("Invalid payment description");
        }
        else
            invalid("Unknown payment term");
    }
    const denominator = BigInt("10") ** BigInt(scale), delta = total - BigInt("100") * denominator, difference = delta < BigInt("0") ? -delta : delta;
    if (total > BigInt("0") && difference * BigInt("100") > denominator)
        fieldError("advance_pct", "Payment percentages must total 100");
}
export function validateVendorMaster(code: string, name: string, value: unknown) {
    if (!/^[A-Z0-9_-]{1,16}$/.test(code))
        fieldError("code", "Use 1–16 letters, numbers, hyphens or underscores");
    if (!trim(name) || runeLength(name) > 255)
        fieldError("name", "Supplier name is required (maximum 255 characters)");
    const data = object(value, "Supplier data is required");
    for (const [key, value] of Object.entries(data)) {
        if (Object.hasOwn(fields, key)) {
            if (typeof value !== "string" || runeLength(value) > fields[key])
                fieldError(key, "Invalid supplier text field");
            continue;
        }
        switch (key) {
            case "qualification":
                if (typeof value !== "string" || !["unverified", "approved", "suspended"].includes(value))
                    invalid("Invalid qualification");
                break;
            case "onboarding_verified":
                if (typeof value !== "boolean")
                    invalid("Invalid onboarding status");
                break;
            case "category_ids":
            case "item_ids":
                if (!Array.isArray(value) || value.length)
                    invalid("Catalogue links require a reviewed Go identifier mapping");
                break;
            case "payment_terms":
                validatePaymentTerms(value);
                break;
            default: invalid("Unknown supplier field");
        }
    }
    if (data.qualification === "approved" && !trim(typeof data.notes === "string" ? data.notes : ""))
        fieldError("notes", "Approved qualification requires review notes");
    return data;
}
export function validateVendorReason(value: unknown) { const result = text(value); if (!trim(result) || byteLength(result) > 2000)
    fieldError("reason", "A change reason is required (maximum 2000 bytes)"); return result; }
export function parseVendorCreate(value: unknown) { const r = goObject(value, ["code", "name", "data", "reason"]), code = upper(trim(text(r.code))), name = trim(text(r.name)); return { code, name, data: validateVendorMaster(code, name, r.data), reason: validateVendorReason(r.reason) }; }
export function parseVendorUpdate(value: unknown) { const r = goObject(value, ["name", "data", "version", "is_active", "reason"]); return { name: trim(text(r.name)), data: r.data, version: integer(r.version), is_active: r.is_active == null ? null : boolean(r.is_active), reason: validateVendorReason(r.reason) }; }
export function parseVendorImport(value: unknown) {
    const r = goObject(value, ["source", "code", "name", "data", "is_active"]), s = goObject(r.source, ["id", "version", "created_at", "updated_at"]);
    const source: VendorSource = { id: text(s.id), version: integer(s.version), created_at: vendorSourceTimestamp(s.created_at), updated_at: vendorSourceTimestamp(s.updated_at) };
    if (!vendorID.test(source.id) || source.version < 1 || r.is_active == null)
        invalid("Source identity, revision, timestamps and status are required");
    const code = text(r.code), name = text(r.name), active = boolean(r.is_active);
    return { source, code, name, data: validateVendorMaster(code, name, r.data), is_active: active };
}
function vendorSourceTimestamp(value: unknown) {
    const result = sourceTimestamp(value);
    const [year, month, day, hour, minute, second] = result.slice(0, 19).split(/[-T:]/).map(Number);
    const days = [31, year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0) ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    if (month < 1 || month > 12 || day < 1 || day > days[month - 1] || hour > 23 || minute > 59 || second > 59 || sourceInstant(result) === "0001-01-01T00:00:00.000000000")
        invalid("Source identity, revision, timestamps and status are required");
    return result;
}
function sorted(value: unknown): unknown {
    if ((JSON as typeof JSON & {
        isRawJSON: (value: unknown) => boolean;
    }).isRawJSON(value))
        return value;
    if (Object.is(value, -0))
        return (JSON as typeof JSON & {
            rawJSON: (s: string) => unknown;
        }).rawJSON("-0");
    if (Array.isArray(value))
        return value.map(sorted);
    if (value && typeof value === "object")
        return Object.fromEntries(Object.keys(value).sort().map(k => [k, sorted((value as Record<string, unknown>)[k])]));
    return value;
}
export const dataFingerprint = (data: Record<string, unknown>) => createHash("sha256").update(goJSON(sorted(data))).digest("hex");
export const missingVendor = () => new AppError(404, "Supplier not found");
export const vendorConflict = () => new AppError(409, "Source import conflicts with an existing supplier");
// PostgreSQL/pgx stores microseconds. Compare source instants at nanosecond precision
// like Go Time.Equal, and truncate source storage exactly like the Go pgx codec.
export function sourceInstant(value: string) { const fraction = /\.(\d+)/.exec(value)?.[1] ?? ""; return new Date(value.replace(/\.\d+/, "")).toISOString().replace(/\.000Z$/, '') + "." + fraction.slice(0, 9).padEnd(9, "0"); }
export const sourceStorage = (value: string) => value.replace(/\.(\d{6})\d+/, '.$1');
