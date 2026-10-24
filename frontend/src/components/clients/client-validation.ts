/** Client Master input checks; these do not verify live tax registration. */
const panPattern = /^[A-Z]{3}[ABCFGHJLPT][A-Z][0-9]{4}[A-Z]$/;
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ";
export function panError(value: string): string {
  const pan = value.trim().toUpperCase();
  if (!pan) return "PAN is required.";
  return panPattern.test(pan) && pan.slice(5, 9) !== "0000" ? "" : "PAN must have 5 letters, 4 digits and 1 letter (for example, AAACW4202F).";
}
export function gstinError(value: string): string {
  const gstin = value.trim().toUpperCase();
  if (!gstin) return "GSTIN is required.";
  if (gstin.length !== 15) return `GSTIN has ${gstin.length} characters; enter all 15 characters from your registration certificate.`;
  if (!/^(0[1-9]|[12][0-9]|3[0-8]|97|99)[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$/.test(gstin) || panError(gstin.slice(2, 12))) return "Enter a valid 15-character Indian GSTIN.";
  let total = 0;
  for (let index = 0; index < 14; index++) {
    const product = alphabet.indexOf(gstin[index]) * (index % 2 === 0 ? 1 : 2);
    total += Math.floor(product / 36) + product % 36;
  }
  if (alphabet[(36 - total % 36) % 36] !== gstin[14]) return "GSTIN check digit is invalid. Check the number.";
  return "";
}
export function websiteError(value: string): string {
  if (!value.trim()) return "";
  try {
    const candidate = /^[a-z][a-z0-9+.-]*:\/\//i.test(value.trim()) ? value.trim() : `https://${value.trim()}`;
    const url = new URL(candidate);
    if (!["http:", "https:"].includes(url.protocol) || !url.hostname.includes(".") || /\s/.test(value.trim()) || url.username || url.password) throw new Error();
    return "";
  } catch { return "Enter a valid website address, such as example.com."; }
}
export function emailError(value: string): string {
  if (!value.trim()) return "Email is required.";
  return /^[^\s@]+@[^\s@.]+(?:\.[^\s@.]+)+$/.test(value.trim()) ? "" : "Enter a valid email address.";
}
export function phoneError(value: string): string {
  if (!value.trim()) return "Phone is required.";
  const compact = value.replace(/[ ()-]/g, "");
  return /^(?:\+91|0)?[1-9][0-9]{9}$/.test(compact) || (/^\+(?!91)[1-9][0-9]{7,14}$/.test(compact)) ? "" : "Enter a 10-digit Indian phone number, or an international number with +country code.";
}
const gstStateNames: Record<string, string> = {"01":"Jammu and Kashmir","02":"Himachal Pradesh","03":"Punjab","04":"Chandigarh","05":"Uttarakhand","06":"Haryana","07":"Delhi","08":"Rajasthan","09":"Uttar Pradesh","10":"Bihar","11":"Sikkim","12":"Arunachal Pradesh","13":"Nagaland","14":"Manipur","15":"Mizoram","16":"Tripura","17":"Meghalaya","18":"Assam","19":"West Bengal","20":"Jharkhand","21":"Odisha","22":"Chhattisgarh","23":"Madhya Pradesh","24":"Gujarat","26":"Dadra and Nagar Haveli and Daman and Diu","27":"Maharashtra","29":"Karnataka","30":"Goa","31":"Lakshadweep","32":"Kerala","33":"Tamil Nadu","34":"Puducherry","35":"Andaman and Nicobar Islands","36":"Telangana","37":"Andhra Pradesh","38":"Ladakh"};
type Form = { client_type_id?: string; industry_id?: string; state_id?: string; city_id?: string; postal_code?: string; legal_name: string; address_line_1: string; website: string; gstin: string; pan: string; state?: string; contacts: { full_name: string; email: string; phone: string; contact_type?: string }[] };
export function clientErrors(form: Form): Record<string, string> {
  const errors: Record<string, string> = {};
  const add = (key: string, error: string) => { if (error) errors[key] = error; };
  add("legal_name", form.legal_name.trim().length >= 2 ? "" : "Legal name must contain at least 2 characters.");
  add("address_line_1", form.address_line_1.trim() ? "" : "Address line 1 is required.");
  for (const [key, label] of [["client_type_id", "client type"], ["industry_id", "industry"], ["state_id", "state / union territory"], ["city_id", "city"]] as const) add(key, form[key] ? "" : `Select a valid ${label} from the list.`);
  add("postal_code", !form.postal_code?.trim() ? "PIN code is required." : /^[1-9][0-9]{5}$/.test(form.postal_code.trim()) ? "" : "Enter a valid 6-digit Indian PIN code.");
  add("website", websiteError(form.website));
  add("pan", panError(form.pan));
  add("gstin", gstinError(form.gstin));
  const gstState = gstStateNames[form.gstin.trim().slice(0, 2)];
  if (!errors.gstin && gstState && form.state && gstState.toLocaleLowerCase("en-IN") !== form.state.trim().toLocaleLowerCase("en-IN")) {
    add("gstin", `GSTIN state code ${form.gstin.trim().slice(0, 2)} belongs to ${gstState}; select the matching state.`);
  }
  if (!panError(form.pan) && !gstinError(form.gstin) && form.gstin.trim().toUpperCase().slice(2, 12) !== form.pan.trim().toUpperCase()) {
    add("pan", "PAN does not match the PAN within GSTIN. Check both against the registration certificate.");
  }
  add("contacts", form.contacts.length <= 20 ? "" : "At most 20 contacts are permitted.");
  form.contacts.forEach((contact, index) => {
    add(`contacts.${index}.contact_type`, !contact.contact_type || ["EIC", "HOD", "SITE_CONTACT"].includes(contact.contact_type) ? "" : "Select a valid contact type.");
    add(`contacts.${index}.full_name`, contact.full_name.trim().length >= 2 ? "" : "Full name must contain at least 2 characters.");
    add(`contacts.${index}.email`, emailError(contact.email));
    add(`contacts.${index}.phone`, phoneError(contact.phone));
  });
  return errors;
}
