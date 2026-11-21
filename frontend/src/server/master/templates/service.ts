import "server-only";
import { transaction } from "../../db";
import { AppError } from "../../errors";
import { requirePermission } from "../../auth/access";
import { hash } from "../catalogue/serialization";
import { byteLength, goObject, integer, invalid, list, strictID, text } from "../catalogue/validation";
import { trim } from "../shared/text";
import { evaluateTemplate, parseDefinition, validateDefinitions } from "./domain";
import { newTemplate, templateView, TemplateRepository } from "./repository";
export function readTemplate(token: string, id: string, preview?: unknown) {
    return transaction(async (db) => {
        await requirePermission(db, token, "settings.configuration_templates.read");
        strictID(id, "Invalid subcategory identity");
        const t = await new TemplateRepository(db).get(id);
        if (!t)
            throw new AppError(404, "Configuration template pending");
        return preview === undefined ? templateView(t!) : evaluateTemplate(t!, preview);
    });
}
export function importTemplates(token: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.configuration_templates.import", true), input = goObject(value, ["templates"]), defs = validateDefinitions(list(input.templates).map(parseDefinition)), repo = new TemplateRepository(db);
        await repo.lock();
        const result = { created: 0, unchanged: 0 };
        for (const def of defs) {
            const parent = await repo.resolve(def.subcategory_code), existing = await repo.get(parent.id), fingerprint = hash(def);
            if (existing) {
                if (existing.source_fingerprint !== fingerprint)
                    throw new AppError(409, "Existing template differs; publish a reviewed revision instead");
                result.unchanged++;
                continue;
            }
            await repo.insert(newTemplate(def, parent.id, 1, fingerprint), def, actor.id, "import");
            result.created++;
        }
        return result;
    });
}
export function publishTemplate(token: string, id: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.configuration_templates.write", true);
        strictID(id, "Invalid subcategory identity");
        const input = goObject(value, ["definition", "expected_revision", "reason"]);
        if (input.expected_revision == null)
            invalid("Expected revision is required");
        const expected = integer(input.expected_revision), reason = text(input.reason);
        if (!trim(reason) || byteLength(reason) > 1000)
            invalid("Review reason must contain 1 to 1000 characters");
        if (expected < 0 || expected >= 2147483647)
            invalid("Expected revision is out of range");
        const [def] = validateDefinitions([parseDefinition(input.definition)]), repo = new TemplateRepository(db);
        await repo.lock();
        const parent = await repo.resolve(def.subcategory_code);
        if (parent.id !== id)
            invalid("Subcategory code does not match the selected parent");
        const before = await repo.get(id);
        if ((before?.revision ?? 0) !== expected)
            throw new AppError(409, "Configuration changed. Reopen the editor before saving.");
        for (const old of before?.fields ?? []) {
            const next = def.fields.find(f => f.field_code === old.field_code);
            if (!next)
                invalid("Retain existing field codes; deactivate fields instead of removing them");
            for (const o of old.options)
                if (!next!.options.some(v => v.code === o.code))
                    invalid("Retain existing option codes; deactivate options instead of removing them");
        }
        if (!parent.active)
            invalid("Activate the subcategory, category and unit before configuring it");
        const saved = newTemplate(def, id, expected + 1, hash(def));
        await repo.insert(saved, def, actor.id, "publish", before, reason);
        return templateView(saved);
    });
}
