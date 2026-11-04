import fullFold from "../catalogue/full-fold.json";
import { itemHash, sortedMap } from "../catalogue/serialization";
import { byteLength, fieldError, goObject, integer, invalid, runeLength, stringMap, text, uuidPattern } from "../catalogue/validation";
import { lower, spacedName, trim, upper } from "../shared/text";
export type ItemField = {
    code: string;
    label: string;
    required: boolean;
    options: {
        code: string;
        label: string;
    }[];
};
export type ItemDefinition = {
    kind: string;
    category_code: string;
    family_code: string;
    uom: string;
    revision: number;
    model_required: boolean;
    fields: ItemField[];
};
export type ItemConfiguration = {
    id: string;
    code: string;
    name: string;
    is_active: boolean;
    version: number;
    data: ItemDefinition;
    category_id?: string;
};
export type ItemMake = {
    id: string;
    code: string;
    name: string;
    is_active: boolean;
    data: {
        eligible: string[] | null;
    };
};
export type ItemSnapshot = {
    configuration: ItemDefinition & {
        name: string;
        code: string;
    };
    manufacturer: ItemMake;
    sku_prefix: string;
};
export type Item = {
    id: string;
    configuration_id: string;
    manufacturer_id: string;
    model: string;
    specifications: Record<string, string>;
    snapshot: ItemSnapshot;
    fingerprint: string;
    engineering_key: string;
    description: string;
    sku: string;
    serial: number;
    status: string;
    is_active: boolean;
    version: number;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
};
export type StoredItem = Item & {
    category_id: string;
};
const modelFold = new Map<number, string>((fullFold as [
    number,
    string
][]).map(([key, value]) => [key, value]));
export function normalizeModel(value: string) { return spacedName([...value.normalize("NFKC")].map(c => modelFold.get(c.codePointAt(0)!) ?? c).join("").replace(/[\u001c-\u001f]/g, " ")); }
export function eligible(c: ItemConfiguration, m: ItemMake) { return c.is_active && m.is_active && (m.data.eligible ?? []).some(code => code === c.code || code === c.data.family_code || code === c.data.category_code); }
export function deriveItem(c: ItemConfiguration, m: ItemMake, spec: Record<string, string>, rawModel: string) {
    if (!c.is_active || c.data.kind !== "type")
        fieldError("configuration_id", "Choose an active configuration with an active category and unit");
    if (!eligible(c, m))
        fieldError("manufacturer_id", "Manufacturer is inactive or not eligible for this configuration");
    if (Object.keys(spec).length > 64)
        fieldError("specifications", "Too many specification fields");
    const known = new Set(c.data.fields.map(f => f.code));
    for (const key of Object.keys(spec))
        if (!known.has(key))
            fieldError("specifications", "Unknown or inactive specification field");
    const values: Record<string, string> = Object.create(null), tokens = [c.data.category_code, c.data.family_code], labels = [c.name];
    for (const f of c.data.fields) {
        const v = upper(trim(spec[f.code] ?? ""));
        if (!v) {
            if (f.required)
                fieldError(f.code, "This specification is required");
            continue;
        }
        const option = f.options.find(o => o.code === v);
        if (!option)
            fieldError(f.code, "Choose a valid active option");
        labels.push(f.label + ": " + option!.label);
        values[f.code] = v;
        tokens.push(f.code + v);
    }
    const model = lower(spacedName(normalizeModel(rawModel)));
    if (runeLength(model) > 120)
        fieldError("model", "Model must be at most 120 characters");
    if (c.data.model_required && !model)
        fieldError("model", "Model or part number is required");
    tokens.push(m.code);
    labels.push(m.name);
    if (model) {
        const mt = upper(model).replace(/[^A-Z0-9]/g, "").slice(0, 24);
        if (mt)
            tokens.push(mt);
        labels.push(model);
    }
    const prefix = tokens.join("-");
    if (byteLength(prefix) > 900)
        fieldError("specifications", "Specifications produce a SKU prefix longer than 900 characters");
    const base = { family: c.data.family_code, uom: c.data.uom, specifications: sortedMap(values) }, engineering_key = itemHash(base), fingerprint = itemHash({ ...base, manufacturer: m.id, model });
    return { engineering_key, fingerprint, model, prefix, description: labels.join(" · "), specifications: values };
}
export function parseItem(value: unknown, change = false) {
    if (!value || typeof value !== "object" || Array.isArray(value))
        invalid("Provide a JSON object");
    for (const [key, v] of Object.entries(value as Record<string, unknown>))
        if (v === null)
            fieldError(key, "Null is not accepted");
    const r = goObject(value, change ? ["status", "version", "reason"] : ["configuration_id", "manufacturer_id", "specifications", "model"]);
    if (change) {
        const version = integer(r.version);
        if (version < 1)
            fieldError("version", "Provide a positive version");
        return { status: text(r.status), version, reason: text(r.reason), configuration_id: "", manufacturer_id: "", model: "", specifications: {} };
    }
    const configuration_id = text(r.configuration_id), manufacturer_id = text(r.manufacturer_id);
    if (!uuidPattern.test(configuration_id))
        fieldError("configuration_id", "Choose a valid configuration");
    if (!uuidPattern.test(manufacturer_id))
        fieldError("manufacturer_id", "Choose a valid manufacturer");
    return { configuration_id: configuration_id.toLowerCase(), manufacturer_id: manufacturer_id.toLowerCase(), model: text(r.model), specifications: stringMap(r.specifications, true), status: "", version: 0, reason: "" };
}
