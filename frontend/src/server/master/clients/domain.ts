import { normalizedName } from "../shared/text";
export type Contact = {
    id: string;
    full_name: string;
    contact_type: string;
    designation: string;
    department: string;
    email: string;
    phone: string;
    is_primary: boolean;
    is_active: boolean;
};
export type ClientFields = {
    legal_name: string;
    display_name: string;
    parent_organization: string;
    client_type: string;
    industry: string;
    gstin: string;
    pan: string;
    cin: string;
    website: string;
    address_line_1: string;
    address_line_2: string;
    city: string;
    state: string;
    postal_code: string;
    country: string;
};
export type ClientInput = ClientFields & {
    client_type_id: string;
    industry_id: string;
    state_id: string;
    city_id: string;
    contacts: Contact[];
};
export type Client = ClientFields & {
    id: string;
    client_code: string;
    is_active: boolean;
    created_at: string;
    updated_at: string;
    contacts: Contact[];
    missing_contact_types: string[];
};
export type Filter = {
    page: number;
    size: number;
    search: string;
    clientType: string;
    active: boolean | null;
    sort: "legal_name" | "client_code" | "created_at";
    descending: boolean;
};
export type Page = {
    items: Client[];
    page: number;
    page_size: number;
    total: number;
    total_pages: number;
    all_count: number;
    active_count: number;
    inactive_count: number;
};
export const contactTypes = ["", "EIC", "HOD", "SITE_CONTACT"];
export function missingContacts(contacts: Contact[]) {
    return ["EIC", "HOD", "SITE_CONTACT"].filter(kind => !contacts.some(c => c.is_active && c.contact_type === kind));
}
export function validPAN(value: string) {
    return /^[A-Z]{3}[ABCFGHJLPT][A-Z][0-9]{4}[A-Z]$/.test(value) && value.slice(5, 9) !== "0000";
}
export function validGST(value: string) {
    if (!/^(0[1-9]|[12][0-9]|3[0-8]|97|99)[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$/.test(value) || !validPAN(value.slice(2, 12)))
        return false;
    const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ";
    let sum = 0;
    for (let i = 0; i < 14; i++) {
        const n = alphabet.indexOf(value[i]) * (1 + i % 2);
        sum += Math.floor(n / 36) + n % 36;
    }
    return value[14] === alphabet[(36 - sum % 36) % 36];
}
const states: Record<string, string> = {
    "01": "Jammu and Kashmir", "02": "Himachal Pradesh", "03": "Punjab", "04": "Chandigarh", "05": "Uttarakhand", "06": "Haryana", "07": "Delhi", "08": "Rajasthan", "09": "Uttar Pradesh", "10": "Bihar", "11": "Sikkim", "12": "Arunachal Pradesh", "13": "Nagaland", "14": "Manipur", "15": "Mizoram", "16": "Tripura", "17": "Meghalaya", "18": "Assam", "19": "West Bengal", "20": "Jharkhand", "21": "Odisha", "22": "Chhattisgarh", "23": "Madhya Pradesh", "24": "Gujarat", "26": "Dadra and Nagar Haveli and Daman and Diu", "27": "Maharashtra", "29": "Karnataka", "30": "Goa", "31": "Lakshadweep", "32": "Kerala", "33": "Tamil Nadu", "34": "Puducherry", "35": "Andaman and Nicobar Islands", "36": "Telangana", "37": "Andhra Pradesh", "38": "Ladakh",
};
export function stateMismatch(gstin: string, state: string) {
    const expected = states[gstin.slice(0, 2)];
    return expected && normalizedName(expected) !== normalizedName(state)
        ? `GSTIN state code ${gstin.slice(0, 2)} belongs to ${expected}, but the selected state is ${state}` : null;
}
