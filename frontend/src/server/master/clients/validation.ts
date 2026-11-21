import { type ClientInput, type Filter, contactTypes, validGST, validPAN } from "./domain";
import { bad, canonicalID, MasterValidationError, strictObject, stringField } from "../shared/validation";
import { goSpace, lower, trim, upper } from "../shared/text";
export const clientKeys = ["legal_name", "display_name", "parent_organization", "client_type", "industry", "client_type_id", "industry_id", "gstin", "pan", "cin", "website", "address_line_1", "address_line_2", "city", "state", "city_id", "state_id", "postal_code", "country", "contacts"];
function website(raw: unknown) {
    if (raw === undefined || raw === null || raw === "")
        return "";
    if (typeof raw !== "string")
        bad("website");
    let value = trim(raw);
    if (!value)
        return "";
    if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(value))
        value = `https://${value}`;
    try {
        const url = new URL(value);
        if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || !url.hostname.includes(".") || goSpace.test(value))
            bad("website");
    }
    catch {
        bad("website");
    }
    if ([...value].length > 255)
        bad("website");
    return value;
}
export function parseClient(value: unknown): ClientInput {
    const input = strictObject(value, clientKeys);
    const result = {} as ClientInput;
    for (const [field, min, max, fallback] of [
        ["legal_name", 2, 255, undefined], ["display_name", 0, 255, ""], ["parent_organization", 0, 255, ""],
        ["client_type", 0, 64, ""], ["industry", 0, 128, ""], ["gstin", 1, 15, undefined], ["pan", 1, 10, undefined],
        ["cin", 0, 21, ""], ["address_line_1", 1, 255, undefined], ["address_line_2", 0, 255, ""],
        ["city", 0, 128, ""], ["state", 0, 128, ""], ["postal_code", 6, 6, undefined], ["country", 2, 128, "India"],
    ] as const)
        result[field] = stringField(input, field, min, max, fallback);
    result.website = website(input.website);
    result.gstin = upper(result.gstin);
    result.pan = upper(result.pan);
    result.cin = upper(result.cin);
    if (!validGST(result.gstin))
        bad("gstin");
    if (!validPAN(result.pan) || result.gstin.slice(2, 12) !== result.pan)
        bad("pan");
    if (result.cin && !/^[A-Z0-9]{21}$/.test(result.cin))
        bad("cin");
    if (!/^[1-9][0-9]{5}$/.test(result.postal_code))
        bad("postal_code");
    for (const key of ["client_type_id", "industry_id", "state_id", "city_id"] as const)
        result[key] = canonicalID(input[key], key);
    const contacts = input.contacts;
    if (!Array.isArray(contacts) || contacts.length > 20)
        bad("contacts");
    result.contacts = contacts.map((raw: unknown) => {
        try {
            const c = strictObject(raw, ["id", "full_name", "designation", "department", "email", "phone", "is_primary", "contact_type"], "contacts");
            const full_name = stringField(c, "full_name", 2, 128), email = stringField(c, "email", 1, 255), phone = stringField(c, "phone", 1, 32);
            const contact_type = stringField(c, "contact_type", 0, 16, ""), designation = stringField(c, "designation", 0, 128, ""), department = stringField(c, "department", 0, 128, "");
            if (!contactTypes.includes(contact_type) || !/^[^\x09-\x0d\x20@]+@[^\x09-\x0d\x20@.]+(?:\.[^\x09-\x0d\x20@.]+)+$/.test(email) || goSpace.test(email))
                bad("contacts");
            const compact = phone.replace(/[ ()-]/g, "");
            if (!/^(?:\+91|0)?[1-9][0-9]{9}$/.test(compact) && (!/^\+[1-9][0-9]{7,14}$/.test(compact) || compact.startsWith("+91")))
                bad("contacts");
            if (c.is_primary !== undefined && typeof c.is_primary !== "boolean")
                bad("contacts");
            return { id: c.id == null ? "" : canonicalID(c.id, "contacts"), full_name, email: lower(email), phone,
                contact_type, designation, department, is_primary: c.is_primary === true, is_active: true };
        }
        catch (error) {
            if (error instanceof MasterValidationError)
                throw new MasterValidationError("contacts");
            throw error;
        }
    });
    if (result.contacts.filter(c => c.is_primary).length > 1)
        bad("contacts", "Only one primary contact can be assigned to a client");
    const ids = result.contacts.filter(c => c.id).map(c => c.id);
    if (new Set(ids).size !== ids.length)
        bad("contacts", "A contact may appear only once");
    result.display_name ||= result.legal_name;
    return result;
}
export function parseFilter(query: URLSearchParams): Filter {
    function invalid(field: string): never { throw new MasterValidationError(field, "query"); }
    function integer(field: string, fallback: number, max: number) {
        const raw = query.get(field), v = raw === null ? fallback : /^[+-]?\d+$/.test(trim(raw)) ? Number(trim(raw)) : NaN;
        if (!Number.isInteger(v) || v < 1 || v > max)
            invalid(field);
        return v;
    }
    const page = integer("page", 1, 2147483647), size = integer("page_size", 20, 100);
    if ((page - 1) * size > 2147483647)
        invalid("page");
    const search = query.get("search") ?? "", clientType = query.get("client_type") ?? "";
    if (query.has("search") && ([...search].length < 1 || [...search].length > 128))
        invalid("search");
    if (query.has("client_type") && ([...clientType].length < 1 || [...clientType].length > 64))
        invalid("client_type");
    const status = query.get("status"), sort = query.get("sort") ?? "legal_name_asc";
    if (status !== null && !["active", "inactive"].includes(status))
        invalid("status");
    const match = /^(legal_name|client_code|created_at)_(asc|desc)$/.exec(sort);
    if (!match)
        invalid("sort");
    return { page, size, search, clientType, active: status === null ? null : status === "active", sort: match[1] as Filter["sort"], descending: match[2] === "desc" };
}
