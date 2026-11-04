import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../../db";
import { AppError } from "../../errors";
import { recordMasterEvent } from "../../events/journal";
import { query } from "../catalogue/database";
import { fieldError } from "../catalogue/validation";
import { TemplateRepository } from "../templates/repository";
import type { StoredTemplate } from "../templates/domain";
import { ManufacturerRepository } from "../manufacturers/repository";
import type { Item, ItemConfiguration, ItemDefinition, ItemMake, StoredItem } from "./domain";
export const itemView = (r: Item): Item => ({ id: r.id, configuration_id: r.configuration_id, manufacturer_id: r.manufacturer_id, model: r.model, specifications: r.specifications, snapshot: r.snapshot, fingerprint: r.fingerprint, engineering_key: r.engineering_key, description: r.description, sku: r.sku, serial: r.serial, status: r.status, is_active: r.is_active, version: r.version, created_at: r.created_at, updated_at: r.updated_at, created_by_id: r.created_by_id, updated_by_id: r.updated_by_id });
export type ItemContext = {
    id: string;
    code: string;
    name: string;
    category_id: string;
    category_code: string;
    category_name: string;
    unit_code: string;
    family_code: string;
    active: boolean;
};
export function configurationView(t: StoredTemplate, c: ItemContext): ItemConfiguration {
    return { id: t.id, code: c.code, name: c.name, is_active: t.is_active && c.active, version: t.revision, category_id: c.category_id, data: { kind: "type", category_code: c.category_code, family_code: c.family_code || c.code, uom: c.unit_code, revision: t.revision, model_required: t.model_required, fields: t.fields.filter(f => f.is_active).map(f => ({ code: f.field_code, label: f.field_name, required: f.mandatory, options: [...f.options.filter(o => o.is_active).map(o => ({ code: o.code, label: o.label })), ...(f.allow_na ? [{ code: "__NA__", label: "N/A" }] : [])] })) } };
}
export class ItemRepository {
    constructor(private db: DB) { }
    async lock() { await query(this.db, "ItemsLockItems"); }
    async list() { return (await query<StoredItem>(this.db, "ItemsListItems")).rows; }
    async get(id: string) { const r = (await query<StoredItem>(this.db, "ItemsGetItem", { id })).rows[0]; if (!r)
        throw new AppError(404, "Item or reference not found"); return r!; }
    async find(fingerprint: string) { return (await query<StoredItem>(this.db, "ItemsFindItem", { fingerprint })).rows[0] ?? null; }
    async candidates(family: string, uom: string, manufacturer_id: string, specifications: Record<string, string>) { return (await query<StoredItem>(this.db, "ItemsIdentityCandidates", { family, uom, manufacturer_id, specifications })).rows; }
    async next(category_code: string) { const row = (await query<Record<string, number>>(this.db, "ItemsNextSerial", { category_code })).rows[0]; return Object.values(row)[0]; }
    async configuration(id: string) { const t = await new TemplateRepository(this.db).get(id, true); if (!t)
        fieldError("configuration_id", "Configuration does not exist"); const c = (await query<ItemContext>(this.db, "CategoriesGetItemContext", { id: t!.subcategory_id })).rows[0]; if (!c)
        fieldError("configuration_id", "Configuration parent is missing"); return configurationView(t!, c!); }
    async manufacturer(id: string): Promise<ItemMake> { const m = await new ManufacturerRepository(this.db).get(id, true); if (!m)
        fieldError("manufacturer_id", "Manufacturer does not exist"); return { id: m!.id, code: m!.code, name: m!.name, is_active: m!.is_active, data: { eligible: m!.data.eligible ?? null } }; }
    async references() {
        const contexts = (await query<ItemContext>(this.db, "CategoriesItemContexts")).rows, templates = await new TemplateRepository(this.db).latest(), catalogue: ItemConfiguration[] = [], seen = new Map<string, ItemConfiguration>();
        for (const c of contexts) {
            if (!seen.has(c.category_id)) {
                const data: ItemDefinition = { kind: "category", category_code: "", family_code: "", uom: c.unit_code, revision: 0, model_required: false, fields: [] }, v = { id: c.category_id, code: c.category_code, name: c.category_name, is_active: c.active, version: 0, data };
                seen.set(c.category_id, v);
                catalogue.push(v);
            }
            else if (c.active)
                seen.get(c.category_id)!.is_active = true;
        }
        for (const t of templates) {
            const c = contexts.find(c => c.id === t.subcategory_id);
            if (c) {
                const v = configurationView(t, c);
                delete v.category_id;
                catalogue.push(v);
            }
        }
        const manufacturers = (await new ManufacturerRepository(this.db).list()).map(m => ({ id: m.id, code: m.code, name: m.name, is_active: m.is_active, data: { eligible: m.data.eligible ?? null } }));
        return { catalogue, manufacturers };
    }
    async create(v: Omit<StoredItem, "version" | "created_at" | "updated_at" | "created_by_id" | "updated_by_id">, actor: string) { return (await query<StoredItem>(this.db, "ItemsCreateItem", { ...v, actor_id: actor })).rows[0]; }
    async change(old: StoredItem, status: string, actor: string) { const [row] = (await query<StoredItem>(this.db, "ItemsChangeItem", { id: old.id, expected_version: old.version, status, is_active: status === "approved", actor_id: actor })).rows; if (!row)
        throw new AppError(409, "Record changed; reload before saving"); return row!; }
    async event(saved: Item, action: string, reason: string, actor: string, before: Item | null = null) { const id = randomUUID(); await query(this.db, "ItemsRecordItemEvent", { id, subject_id: saved.id, action, reason, before_snapshot: before ? itemView(before) : "null", after_snapshot: itemView(saved), actor_id: actor }); await recordMasterEvent(this.db, "catalogue_items", "item", saved.id, saved.version, action, actor, id, saved.updated_at); }
    async history(id: string) { await this.get(id); const rows = (await query<{
        id: string;
        subject_id: string;
        action: string;
        reason: string;
        before_snapshot: Item | null;
        after_snapshot: Item;
        created_at: string;
        created_by_id: string | null;
    }>(this.db, "ItemsItemHistory", { subject_id: id })).rows; return rows.map(r => ({ id: r.id, subject_id: r.subject_id, action: r.action, reason: r.reason, before: r.before_snapshot, after: r.after_snapshot, created_at: r.created_at, created_by_id: r.created_by_id ?? "00000000-0000-0000-0000-000000000000" })); }
}
