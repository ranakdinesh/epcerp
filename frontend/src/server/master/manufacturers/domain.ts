import { fold, trim } from "../shared/text";
import { strictObject, stringField } from "../shared/validation";
import { boolean, byteLength, goObject, integer, invalid, list, queryFields, queryInt, text } from "../catalogue/validation";
import { sourceTimestamp } from "../categories/domain";
export type Manufacturer = {
    id: string;
    code: string;
    name: string;
    data: {
        eligible?: string[];
        notes?: string;
        [key: string]: unknown;
    };
    is_active: boolean;
    version: number;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
};
export type ManufacturerData = {
    eligible: string[];
    notes: string;
};
export const nameKey = (s: string) => fold(trim(s));
export function parseManufacturer(value: unknown, kind: "create" | "profile" | "active") {
    const input = strictObject(value, kind === "active" ? ["is_active", "version", "reason"] : kind === "profile" ? ["name", "data", "version", "reason"] : ["code", "name", "data", "reason"]);
    const reason = stringField(input, "reason", 1, 2000);
    const version = kind === "create" ? 0 : integer(input.version);
    if (kind !== "create" && version < 1)
        invalid("A current record version is required");
    if (kind === "active") {
        if (typeof input.is_active !== "boolean")
            invalid("Invalid or missing is_active");
        return { reason, version, is_active: input.is_active as boolean, name: "", code: "", data: { eligible: [], notes: "" } };
    }
    const name = stringField(input, "name", 1, 255), code = kind === "create" ? stringField(input, "code", 1, 16) : "";
    if (kind === "create" && !/^[A-Z0-9]{1,16}$/.test(code))
        invalid("Code requires 1–16 letters or digits");
    const data = strictObject(input.data, ["eligible", "notes"]), notes = stringField(data, "notes", 0, 2000, "");
    if (!Array.isArray(data.eligible) || !data.eligible.length || data.eligible.length > 200 || data.eligible.some(v => typeof v !== "string"))
        invalid("Select 1–200 eligibility codes");
    return { reason, version, name, code, data: { eligible: (data.eligible as string[]).map(trim), notes }, is_active: true };
}
export function eligibilityQuery(q: URLSearchParams) {
    queryFields(q, ["search", "codes", "skip", "kind", "category_code"]);
    const search = q.get("search") ?? "", codes = q.get("codes") ? q.get("codes")!.split(",") : [], skip = queryInt(q, "skip", 0), kind = q.get("kind") ?? "", category_code = q.get("category_code") ?? "";
    if (byteLength(search) > 128 || codes.length > 200 || skip < 0 || skip > 1000000 || !["", "category", "subcategory"].includes(kind) || byteLength(category_code) > 32 || (category_code && kind !== "subcategory"))
        invalid("Invalid eligibility query");
    return { search, codes, skip, kind, category_code };
}
export function parseManufacturerImport(value: unknown) {
    const r = goObject(value, ["source", "code", "name", "data", "is_active"]), s = goObject(r.source, ["created_by_id", "updated_by_id", "id", "version", "created_at", "updated_at"]), d = goObject(r.data, ["eligible", "notes"]);
    const data = { eligible: list(d.eligible).map(v => text(v)), notes: text(d.notes) }, code = text(r.code), name = text(r.name);
    const source = { created_by_id: s.created_by_id == null ? null : text(s.created_by_id), updated_by_id: s.updated_by_id == null ? null : text(s.updated_by_id), id: text(s.id), version: integer(s.version), created_at: sourceTimestamp(s.created_at), updated_at: sourceTimestamp(s.updated_at) };
    if (!/^[A-Z0-9]{1,16}$/.test(code) || !trim(name) || byteLength(name) > 255 || byteLength(data.notes) > 2000 || !data.eligible.length || data.eligible.length > 200 || r.is_active == null || source.version < 1)
        invalid("Invalid manufacturer source snapshot");
    return { source, code, name, data, is_active: boolean(r.is_active) };
}
