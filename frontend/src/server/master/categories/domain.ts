import { lower, normalizedName, spacedName, trim, upper } from "../shared/text";
import { boolean, byteLength, fieldError, goObject, integer, invalid, list, queryFields, queryInt, runeLength, text, uuidPattern } from "../catalogue/validation";
export type CategoryKind = "category" | "subcategory" | "unit";
export type CategoryField = {
    code: string;
    label: string;
    required: boolean;
    options: {
        code: string;
        label: string;
    }[];
};
export type Configuration = {
    fields: CategoryField[];
    model_required: boolean;
    family_code: string;
    revision: number;
};
export type Category = {
    id: string;
    code: string;
    name: string;
    is_active: boolean;
    version: number;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
    category_id?: string;
    unit_id?: string;
    configuration?: Configuration;
};
export type StoredCategory = Category & {
    source_fingerprint: string | null;
};
export type CategorySource = {
    id: string;
    code: string;
    name: string;
    data: {
        kind: string;
        category_code?: string;
        uom?: string;
        fields?: CategoryField[];
        model_required?: boolean;
        family_code?: string;
        revision?: number;
        source?: string;
        legacy_category_code?: string;
    };
    is_active: boolean;
    version: number;
    created_at: string;
    updated_at: string;
};
export const unitNames: Record<string, string> = { Nos: "Number", Meter: "Meter", Kg: "Kilogram", Set: "Set", Pair: "Pair", Lot: "Lot", Sqm: "Square meter", Rmt: "Running meter", Bundle: "Bundle", Box: "Box" };
export const codePattern = /^[A-Z0-9]{1,32}$/;
export const sameName = (a: string, b: string) => normalizedName(a) === normalizedName(b);
export function categoryKind(value: string): CategoryKind { if (!["category", "subcategory", "unit"].includes(value))
    invalid("Unknown catalogue record kind"); return value as CategoryKind; }
export function cleanCategory(kind: CategoryKind, value: unknown, edit = false) {
    const input = goObject(value, edit ? ["code", "name", "reason", "version"] : ["code", "name", "unit_id", "category_id", "reason"]);
    const originalCode = text(input.code), code = upper(trim(originalCode)), name = spacedName(text(input.name)), reason = trim(text(input.reason));
    if (!codePattern.test(code) || (kind === "unit" && code.length > 16))
        fieldError("code", "Use letters and digits, up to 32 characters (16 for units)");
    if (!name || runeLength(name) > (kind === "unit" ? 128 : 255) || /[\0\r\n]/.test(name))
        fieldError("name", "A valid name is required");
    if (!reason || runeLength(reason) > 2000 || reason.includes("\0"))
        fieldError("reason", "A reason of up to 2000 characters is required");
    const unit_id = text(input.unit_id), category_id = text(input.category_id), version = integer(input.version);
    if (edit) {
        if (version < 1 || version === 2147483647)
            fieldError("version", "A current record version is required");
    }
    else if (kind === "category" && (!uuidPattern.test(unit_id) || category_id))
        fieldError("unit_id", "Select an active base unit");
    else if (kind === "subcategory" && (!uuidPattern.test(category_id) || unit_id))
        fieldError("category_id", "Select an active category; its unit is inherited");
    else if (kind === "unit" && (unit_id || category_id))
        invalid("Units do not have parent references");
    return { code, name, reason, version, unit_id, category_id, originalCode };
}
export function browseQuery(q: URLSearchParams) {
    queryFields(q, ["kind", "category_id", "search", "status", "sort", "direction", "page", "size"]);
    const kind = categoryKind(q.get("kind") || "category"), category_id = q.get("category_id") || null, search = q.get("search") ?? "", status = q.get("status") ?? "", sort = q.get("sort") || "name", direction = q.get("direction") || "asc", page = queryInt(q, "page", 1), size = queryInt(q, "size", 20);
    if (kind === "subcategory" && !uuidPattern.test(category_id ?? ""))
        invalid("A category is required");
    if (kind !== "subcategory" && category_id)
        invalid("Unexpected category filter");
    if (page < 1 || page > 1000000 || ![20, 50, 100].includes(size))
        invalid("Invalid page or size");
    if (byteLength(search) > 128 || !["", "Active", "Inactive"].includes(status))
        invalid("Invalid search or status");
    if (!["code", "name", "unit", "status", "types"].includes(sort))
        invalid("Invalid sort");
    if (!["asc", "desc"].includes(direction))
        invalid("Invalid direction");
    return { kind, category_id, search, status, sort, direction, page, size, page_size: size, page_offset: (page - 1) * size };
}
export function sourceTimestamp(value: unknown) {
    const s = text(value);
    if (!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?(?:Z|[+-]\d\d:\d\d)$/.test(s) || !Number.isFinite(Date.parse(s)) || s.startsWith("0001-01-01T00:00:00Z"))
        invalid("Invalid source identifier, code, name or audit timestamps");
    return s.replace(/\+00:00$/, "Z").replace(/(\.\d*?[1-9])0+(?=Z|[+-])/, '$1').replace(/\.0+(?=Z|[+-])/, '');
}
function sourceFields(value: unknown): CategoryField[] {
    return list(value).map(raw => {
        const f = goObject(raw, ["code", "label", "required", "options"]);
        return { code: text(f.code), label: text(f.label), required: boolean(f.required), options: list(f.options).map(raw => { const o = goObject(raw, ["code", "label"]); return { code: text(o.code), label: text(o.label) }; }) };
    });
}
export function parseCategorySources(value: unknown): CategorySource[] {
    const input = goObject(value, ["records"]);
    const result = list(input.records).map(raw => {
        const r = goObject(raw, ["id", "code", "name", "data", "is_active", "version", "created_at", "updated_at"]), d = goObject(r.data, ["kind", "category_code", "uom", "fields", "model_required", "family_code", "revision", "source", "legacy_category_code"]);
        if (r.is_active == null)
            invalid("Source is_active is required");
        const data: CategorySource["data"] = { kind: text(d.kind) };
        // Match DTO field ordering and omitempty for stable legacy fingerprints.
        for (const key of ["category_code", "uom"] as const) {
            const v = text(d[key]);
            if (v)
                data[key] = v;
        }
        const fields = sourceFields(d.fields);
        if (fields.length)
            data.fields = fields;
        if (boolean(d.model_required))
            data.model_required = true;
        const family = text(d.family_code);
        if (family)
            data.family_code = family;
        const rev = integer(d.revision);
        if (rev)
            data.revision = rev;
        for (const key of ["source", "legacy_category_code"] as const) {
            const v = text(d[key]);
            if (v)
                data[key] = v;
        }
        return { id: text(r.id), code: text(r.code), name: text(r.name), data, is_active: boolean(r.is_active), version: integer(r.version), created_at: sourceTimestamp(r.created_at), updated_at: sourceTimestamp(r.updated_at) };
    });
    if (!result.length || result.length > 1000)
        invalid("Import requires 1–1000 definitions");
    const codes = new Set<string>(), ids = new Set<string>(), names = new Set<string>(), parents = new Set<string>();
    const validText = (s: string, max = 255) => !!s && trim(s) === s && runeLength(s) <= max && !/[\0\r\n]/.test(s);
    for (const r of result) {
        if (!uuidPattern.test(r.id) || !codePattern.test(r.code) || !validText(r.name) || r.version < 1)
            invalid("Invalid source identifier, code, name or audit timestamps");
        if (codes.has(r.code) || ids.has(lower(r.id)))
            invalid("Duplicate source code or identifier");
        codes.add(r.code);
        ids.add(lower(r.id));
        if (r.data.kind === "category") {
            if (names.has(lower(r.name)))
                invalid("Duplicate category name");
            names.add(lower(r.name));
            parents.add(r.code);
        }
        else if (r.data.kind !== "type")
            invalid("Unknown source definition kind");
    }
    for (const r of result) {
        const d = r.data;
        if (d.kind !== "type")
            continue;
        if (!parents.has(d.category_code ?? "") || !Object.hasOwn(unitNames, d.uom ?? "") || !codePattern.test(d.family_code ?? "") || !d.revision || d.revision < 1 || !d.fields?.length || d.fields.length > 64)
            invalid("Subcategory needs an imported parent, controlled unit and valid configuration");
        const fields = new Set<string>();
        for (const f of d.fields!) {
            if (!codePattern.test(f.code) || !validText(f.label) || fields.has(f.code) || !f.options.length || f.options.length > 500)
                invalid("Invalid or duplicate configuration field");
            fields.add(f.code);
            const options = new Set<string>(), labels = new Set<string>();
            for (const o of f.options) {
                if (!codePattern.test(o.code) || !validText(o.label) || options.has(o.code) || labels.has(lower(o.label)))
                    invalid("Invalid or duplicate configuration option");
                options.add(o.code);
                labels.add(lower(o.label));
            }
        }
    }
    const units = new Map<string, string>();
    for (const r of result)
        if (r.data.kind === "type") {
            const code = r.data.category_code!, uom = r.data.uom!;
            if (units.has(code) && units.get(code) !== uom)
                invalid("All subcategories must share their category base unit");
            units.set(code, uom);
        }
    for (const r of result)
        if (r.data.kind === "category" && !units.has(r.code))
            invalid("Imported category needs a base unit from its subcategories");
    return result;
}
