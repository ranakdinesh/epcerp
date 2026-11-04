import { lower, trim } from "../shared/text";
import { boolean, byteLength, fieldError, goObject, integer, invalid, list, stringMap, text } from "../catalogue/validation";
import { hash, sortedMap } from "../catalogue/serialization";
export type TemplateOption = {
    id?: string;
    code: string;
    label: string;
    is_active: boolean;
    sort_order: number;
};
export type TemplateField = {
    id?: string;
    field_code: string;
    field_name: string;
    input_type: string;
    mandatory: boolean;
    allow_na: boolean;
    use_in_description: boolean;
    description_order: number;
    description_prefix: string;
    description_suffix: string;
    use_in_duplicate_check: boolean;
    sort_order: number;
    is_active: boolean;
    options: TemplateOption[];
};
export type TemplateDefinition = {
    model_required: boolean;
    subcategory_code: string;
    template_name: string;
    fields: TemplateField[];
};
export type Template = {
    model_required: boolean;
    id: string;
    subcategory_id: string;
    template_name: string;
    revision: number;
    is_active: boolean;
    fields: TemplateField[];
};
export type StoredTemplate = Template & {
    source_fingerprint: string;
};
export const templateKeys = ["model_required", "subcategory_code", "template_name", "fields"];
const fieldKeys = ["id", "field_code", "field_name", "input_type", "mandatory", "allow_na", "use_in_description", "description_order", "description_prefix", "description_suffix", "use_in_duplicate_check", "sort_order", "is_active", "options"];
const optionKeys = ["id", "code", "label", "is_active", "sort_order"];
export function parseDefinition(value: unknown): TemplateDefinition {
    const d = goObject(value, templateKeys);
    return { model_required: boolean(d.model_required), subcategory_code: text(d.subcategory_code), template_name: text(d.template_name), fields: list(d.fields).map(raw => {
            const f = goObject(raw, fieldKeys), id = text(f.id);
            return { ...(id ? { id } : {}), field_code: text(f.field_code), field_name: text(f.field_name), input_type: text(f.input_type), mandatory: boolean(f.mandatory), allow_na: boolean(f.allow_na), use_in_description: boolean(f.use_in_description), description_order: integer(f.description_order), description_prefix: text(f.description_prefix), description_suffix: text(f.description_suffix), use_in_duplicate_check: boolean(f.use_in_duplicate_check), sort_order: integer(f.sort_order), is_active: boolean(f.is_active), options: list(f.options).map(raw => { const o = goObject(raw, optionKeys), id = text(o.id); return { ...(id ? { id } : {}), code: text(o.code), label: text(o.label), is_active: boolean(o.is_active), sort_order: integer(o.sort_order) }; }) };
        }) };
}
export function validateDefinitions(defs: TemplateDefinition[]) {
    if (!defs.length || defs.length > 20)
        invalid("Provide 1–20 templates");
    const parents = new Set<string>(), names = new Set<string>();
    const code = (s: string) => /^[A-Z0-9]{1,32}$/.test(s), name = (s: string, max = 255) => !!trim(s) && byteLength(s) <= max;
    for (const d of defs) {
        if (!code(d.subcategory_code) || parents.has(d.subcategory_code) || !name(d.template_name) || names.has(lower(d.template_name)))
            invalid("Invalid or duplicate template name/subcategory");
        parents.add(d.subcategory_code);
        names.add(lower(d.template_name));
        if (!d.fields.length || d.fields.length > 64)
            invalid("Provide 1–64 fields");
        const codes = new Set<string>(), fieldNames = new Set<string>(), orders = new Set<number>(), descriptionOrders = new Set<number>();
        let total = 0;
        for (const f of d.fields) {
            if (f.id || !code(f.field_code) || codes.has(f.field_code) || !name(f.field_name) || fieldNames.has(lower(f.field_name)) || f.input_type !== "dropdown" || f.sort_order < 1 || f.sort_order > 10000 || orders.has(f.sort_order) || byteLength(f.description_prefix) > 128 || byteLength(f.description_suffix) > 128)
                invalid("Invalid or duplicate field");
            codes.add(f.field_code);
            fieldNames.add(lower(f.field_name));
            orders.add(f.sort_order);
            if (f.use_in_description) {
                if (f.description_order < 1 || f.description_order > 10000 || descriptionOrders.has(f.description_order))
                    invalid("Description orders must be positive and unique");
                descriptionOrders.add(f.description_order);
            }
            else if (f.description_order !== 0)
                invalid("Excluded descriptions must have order zero");
            if (!f.options.length || f.options.length > 128)
                invalid("Provide 1–128 controlled options");
            total += f.options.length;
            if (total > 2048)
                invalid("Too many template options");
            const options = new Set<string>(), labels = new Set<string>(), optionOrders = new Set<number>();
            for (const o of f.options) {
                if (o.id || !code(o.code) || options.has(o.code) || !name(o.label) || labels.has(lower(o.label)) || o.sort_order < 1 || o.sort_order > 10000 || optionOrders.has(o.sort_order))
                    invalid("Invalid or duplicate controlled option");
                options.add(o.code);
                labels.add(lower(o.label));
                optionOrders.add(o.sort_order);
            }
            if (f.is_active && !f.options.some(o => o.is_active))
                invalid("Active fields require active controlled options");
        }
    }
    return defs;
}
export function evaluateTemplate(t: Template, raw: unknown) {
    const input = goObject(raw, ["values"]), values = stringMap(input.values), known = new Set<string>(), identity: Record<string, string> = Object.create(null), parts: {
        order: number;
        text: string;
    }[] = [];
    if (!t.is_active)
        invalid("Template is inactive");
    for (const f of t.fields) {
        if (!f.is_active)
            continue;
        known.add(f.field_code);
        const value = values[f.field_code] ?? "";
        let label = "";
        if (!value) {
            if (f.mandatory)
                fieldError(f.field_code, "Select a value or allowed N/A");
        }
        else if (value === "__NA__") {
            if (!f.allow_na)
                fieldError(f.field_code, "N/A is not allowed");
        }
        else {
            const option = f.options.find(o => o.code === value && o.is_active);
            if (!option)
                fieldError(f.field_code, "Select an active controlled option");
            label = option!.label;
        }
        if (f.use_in_duplicate_check)
            identity[f.field_code] = value;
        if (f.use_in_description && label)
            parts.push({ order: f.description_order, text: f.description_prefix + label + f.description_suffix });
    }
    for (const code of Object.keys(values))
        if (!known.has(code))
            invalid("Unknown or inactive specification field");
    return { description: parts.sort((a, b) => a.order - b.order).map(p => p.text).join(" "), engineering_key: hash({ Subcategory: t.subcategory_id, Values: sortedMap(identity) }) };
}
