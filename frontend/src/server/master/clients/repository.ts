import "server-only";
import { rows, type DB } from "../../db";
import { AppError } from "../../errors";
import { type Client, type ClientFields, type Contact, type Filter, type Page, missingContacts } from "./domain";
import { fold, normalizedName } from "../shared/text";
type Header = Omit<Client, "contacts" | "missing_contact_types">;
const fields: Array<keyof ClientFields> = ["legal_name", "display_name", "parent_organization", "client_type", "industry", "gstin", "pan", "cin", "website", "address_line_1", "address_line_2", "city", "state", "postal_code", "country"];
const headerColumns = `id,client_code,${fields.join(",")},is_active,
  to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS created_at,
  to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS updated_at`;
const contactColumns = "id,client_id,full_name,contact_type,designation,department,email,phone,is_primary,is_active";
function timestamp(value: string) { return value.replace(/(\.\d*?[1-9])0+Z$/, "$1Z").replace(/\.0+Z$/, "Z"); }
export class ClientRepository {
    constructor(private db: DB) { }
    private async attachContacts(headers: Header[]): Promise<Client[]> {
        if (!headers.length)
            return [];
        const contacts = await rows<Contact & {
            client_id: string;
        }>(this.db, `SELECT ${contactColumns} FROM public.client_contacts WHERE client_id=ANY($1::uuid[]) ORDER BY full_name`, [headers.map(h => h.id)]);
        const grouped = new Map<string, Contact[]>();
        for (const { client_id, ...contact } of contacts) {
            const values = grouped.get(client_id) ?? [];
            values.push(contact);
            grouped.set(client_id, values);
        }
        return headers.map(header => {
            const values = grouped.get(header.id) ?? [];
            return { ...header, created_at: timestamp(header.created_at), updated_at: timestamp(header.updated_at), contacts: values, missing_contact_types: missingContacts(values) };
        });
    }
    async get(id: string, forUpdate = false): Promise<Client | null> {
        const headers = await rows<Header>(this.db, `SELECT ${headerColumns} FROM public.clients WHERE id=$1 ${forUpdate ? "FOR UPDATE" : ""}`, [id]);
        return (await this.attachContacts(headers))[0] ?? null;
    }
    async list(filter: Filter): Promise<Page> {
        const where = `($1::text='' OR lower(legal_name) LIKE $1 OR lower(display_name) LIKE $1 OR lower(client_code) LIKE $1 OR lower(city) LIKE $1)
      AND ($2::boolean IS NULL OR is_active=$2) AND ($3::text='' OR lower(client_type)=$3)`;
        const args = [filter.search ? `%${fold(filter.search)}%` : "", filter.active, fold(filter.clientType)];
        // Only validated, whitelisted sort names can become SQL identifiers.
        const sort = { legal_name: "legal_name", client_code: "client_code", created_at: "created_at" }[filter.sort];
        const headers = await rows<Header>(this.db, `SELECT ${headerColumns} FROM public.clients WHERE ${where} ORDER BY ${sort} ${filter.descending ? "DESC" : "ASC"} LIMIT $4 OFFSET $5`, [...args, filter.size, (filter.page - 1) * filter.size]);
        const [count] = await rows<{
            n: string;
        }>(this.db, `SELECT count(*) AS n FROM public.clients WHERE ${where}`, args);
        const [summary] = await rows<{
            all_count: string;
            active_count: string;
            inactive_count: string;
        }>(this.db, "SELECT count(*) AS all_count,count(*) FILTER(WHERE is_active) AS active_count,count(*) FILTER(WHERE NOT is_active) AS inactive_count FROM public.clients");
        return { items: await this.attachContacts(headers), page: filter.page, page_size: filter.size, total: Number(count.n), total_pages: Math.ceil(Number(count.n) / filter.size), all_count: Number(summary.all_count), active_count: Number(summary.active_count), inactive_count: Number(summary.inactive_count) };
    }
    async save(id: string, code: string, input: ClientFields, active: boolean, by: string, create: boolean) {
        const args = [id, ...fields.map(k => input[k]), normalizedName(input.legal_name), active, by];
        const normalizedIndex = fields.length + 2, activeIndex = normalizedIndex + 1, actorIndex = activeIndex + 1;
        try {
            if (create)
                await this.db.query(`INSERT INTO public.clients(id,${fields.join(",")},normalized_legal_name,is_active,created_by_id,updated_by_id,client_code)
        VALUES($1,${fields.map((_, i) => `$${i + 2}`).join(",")},$${normalizedIndex},$${activeIndex},$${actorIndex},$${actorIndex},$${actorIndex + 1})`, [...args, code]);
            else
                await this.db.query(`UPDATE public.clients SET ${fields.map((key, i) => `${key}=$${i + 2}`).join(",")},normalized_legal_name=$${normalizedIndex},is_active=$${activeIndex},updated_by_id=$${actorIndex},updated_at=now() WHERE id=$1`, args);
        }
        catch (error) {
            if ((error as {
                code?: string;
            }).code === "23505")
                throw new AppError(409, "A client with this legal name already exists");
            throw error;
        }
    }
    async contacts(id: string, contacts: Contact[], by: string) {
        const known = new Set((await rows<{
            id: string;
        }>(this.db, "SELECT id FROM public.client_contacts WHERE client_id=$1", [id])).map(c => c.id));
        await this.db.query("UPDATE public.client_contacts SET is_active=false,updated_by_id=$3,updated_at=now() WHERE client_id=$1 AND NOT(id=ANY($2::uuid[]))", [id, contacts.map(c => c.id), by]);
        for (const c of contacts) {
            const args = [c.id, id, c.full_name, c.contact_type, c.designation, c.department, c.email, c.phone, c.is_primary, c.is_active, by];
            if (known.has(c.id))
                await this.db.query("UPDATE public.client_contacts SET full_name=$3,contact_type=$4,designation=$5,department=$6,email=$7,phone=$8,is_primary=$9,is_active=$10,updated_by_id=$11,updated_at=now() WHERE id=$1 AND client_id=$2", args);
            else
                await this.db.query("INSERT INTO public.client_contacts(id,client_id,full_name,contact_type,designation,department,email,phone,is_primary,is_active,created_by_id,updated_by_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)", args);
        }
    }
}
