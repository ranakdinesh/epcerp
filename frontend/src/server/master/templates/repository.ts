import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../../db";
import { recordMasterEvent } from "../../events/journal";
import { query } from "../catalogue/database";
import { invalid } from "../catalogue/validation";
import type { StoredTemplate, Template, TemplateDefinition, TemplateField, TemplateOption } from "./domain";
export const templateView = (t: StoredTemplate): Template => ({ model_required: t.model_required, id: t.id, subcategory_id: t.subcategory_id, template_name: t.template_name, revision: t.revision, is_active: t.is_active, fields: t.fields });
export class TemplateRepository {
    constructor(private db: DB) { }
    async lock() { await query(this.db, "TemplatesLockTemplates"); }
    async resolve(code: string) { const [parent] = (await query<{
        id: string;
        active: boolean;
    }>(this.db, "CategoriesFindSubcategoryByCode", { code })).rows; if (!parent)
        invalid("Subcategory does not exist"); return parent!; }
    async get(id: string, byRevision = false) { const row = (await query<StoredTemplate>(this.db, byRevision ? "TemplatesGetTemplateByID" : "TemplatesGetTemplate", byRevision ? { id } : { subcategory_id: id })).rows[0]; return row ? this.attach(row) : null; }
    async latest() { const rows = (await query<StoredTemplate>(this.db, "TemplatesListLatestTemplates")).rows; return Promise.all(rows.map(r => this.attach(r))); }
    private async attach(row: StoredTemplate) {
        const fields = (await query<TemplateField>(this.db, "TemplatesListFields", { template_id: row.id })).rows;
        const options = (await query<{
            id: string;
            field_id: string;
            option_code: string;
            option_label: string;
            is_active: boolean;
            sort_order: number;
        }>(this.db, "TemplatesListOptions", { template_id: row.id })).rows;
        const parsed = fields.map(f => ({ id: f.id!, field_code: f.field_code, field_name: f.field_name, input_type: f.input_type, mandatory: f.mandatory, allow_na: f.allow_na, use_in_description: f.use_in_description, description_order: f.description_order, description_prefix: f.description_prefix, description_suffix: f.description_suffix, use_in_duplicate_check: f.use_in_duplicate_check, sort_order: f.sort_order, is_active: f.is_active, options: options.filter(o => o.field_id === f.id).map(o => ({ id: o.id, code: o.option_code, label: o.option_label, is_active: o.is_active, sort_order: o.sort_order } as TemplateOption)) }));
        return { model_required: row.model_required, id: row.id, subcategory_id: row.subcategory_id, template_name: row.template_name, revision: row.revision, is_active: row.is_active, fields: parsed, source_fingerprint: row.source_fingerprint };
    }
    async insert(t: StoredTemplate, def: TemplateDefinition, actor: string, action: "import" | "publish", before: StoredTemplate | null = null, reason = "") {
        await query(this.db, "TemplatesInsertTemplate", { ...t, legacy_definition: action === "import" ? def : {}, actor_id: actor });
        for (const f of t.fields) {
            await query(this.db, "TemplatesInsertField", { ...f, template_id: t.id, actor_id: actor });
            for (const o of f.options)
                await query(this.db, "TemplatesInsertOption", { id: o.id, field_id: f.id, option_code: o.code, option_label: o.label, sort_order: o.sort_order, is_active: o.is_active, actor_id: actor });
        }
        await recordMasterEvent(this.db, "configuration_templates", "template", t.id, t.revision, action, actor);
        if (action === "publish")
            await query(this.db, "TemplatesInsertPublication", { template_id: t.id, previous_template_id: before?.id ?? null, reason, before_definition: before ? templateView(before) : { model_required: false, id: "", subcategory_id: "", template_name: "", revision: 0, is_active: false, fields: null }, after_definition: def, actor_id: actor });
    }
}
export function newTemplate(def: TemplateDefinition, parent: string, revision: number, fingerprint: string): StoredTemplate {
    return { model_required: def.model_required, id: randomUUID(), subcategory_id: parent, template_name: def.template_name, revision, is_active: true, fields: def.fields.map(f => ({ ...f, id: randomUUID(), options: f.options.map(o => ({ ...o, id: randomUUID() })) })), source_fingerprint: fingerprint };
}
