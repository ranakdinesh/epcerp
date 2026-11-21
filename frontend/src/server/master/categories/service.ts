import "server-only";
import { randomUUID } from "node:crypto";
import { transaction } from "../../db";
import { AppError } from "../../errors";
import { requirePermission } from "../../auth/access";
import { hash } from "../catalogue/serialization";
import { invalid, strictID } from "../catalogue/validation";
import { categoryView, CategoryRepository } from "./repository";
import { browseQuery, categoryKind, cleanCategory, parseCategorySources, sameName, unitNames, type StoredCategory } from "./domain";
export function readCategories(token: string, mode = "list", query = new URLSearchParams(), kind = "", id = "") {
    return transaction(async (db) => {
        await requirePermission(db, token, "settings.categories.read");
        const repo = new CategoryRepository(db);
        if (mode === "browse")
            return repo.browse(browseQuery(query));
        if (mode === "detail")
            return repo.detail(categoryKind(kind), strictID(id, "Invalid record identity"));
        return repo.all();
    });
}
export function saveCategory(token: string, rawKind: string, id: string | null, value: unknown) {
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.categories.write", true), kind = categoryKind(rawKind), input = cleanCategory(kind, value, id !== null);
        if (id !== null)
            strictID(id, "Invalid record identity");
        const repo = new CategoryRepository(db);
        await repo.lock();
        const records = await repo.list(kind);
        const old = id === null ? null : records.find(r => r.id === id);
        if (id !== null && !old)
            throw new AppError(404, "Record not found");
        if (old && old.version !== input.version)
            throw new AppError(409, "Record changed; reload before saving");
        const code = old && sameName(input.originalCode, old.code) ? old.code : input.code;
        const draft = { id: id ?? randomUUID(), code, name: input.name, is_active: true, version: 1, source_fingerprint: null } as StoredCategory;
        if (old)
            Object.assign(draft, old, { code, name: input.name });
        else if (kind === "category") {
            const unit = (await repo.list("unit")).find(u => u.id === input.unit_id && u.is_active);
            if (!unit)
                invalid("Select an active base unit");
            draft.unit_id = unit!.id;
        }
        else if (kind === "subcategory") {
            const parent = (await repo.list("category")).find(c => c.id === input.category_id && c.is_active);
            if (!parent)
                invalid("Select an active category");
            if (!(await repo.list("unit")).some(u => u.id === parent!.unit_id && u.is_active))
                invalid("Category base unit is inactive");
            draft.category_id = parent!.id;
            draft.unit_id = parent!.unit_id;
            draft.configuration = { fields: [], family_code: code, revision: 1, model_required: false };
        }
        for (const r of records)
            if (r.id !== id) {
                if (sameName(r.code, code))
                    invalid("Code already exists");
                if (sameName(r.name, input.name) && (kind !== "subcategory" || r.category_id === draft.category_id))
                    invalid("Name already exists");
            }
        return categoryView(old ? await repo.update(kind, old, code, input.name, actor.id, input.reason) : await repo.insert(kind, draft, actor.id, input.reason));
    });
}
export function importCategories(token: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.categories.import", true), sources = parseCategorySources(value), repo = new CategoryRepository(db);
        await repo.lock();
        const result = { categories: 0, subcategories: 0, units: 0 }, units = new Map<string, string>(), parents = new Map<string, string>(), categoryUnits = new Map<string, string>();
        for (const s of sources)
            if (s.data.kind === "type")
                categoryUnits.set(s.data.category_code!, s.data.uom!);
        const existing = { unit: await repo.list("unit"), category: await repo.list("category"), subcategory: await repo.list("subcategory") };
        const save = async (kind: "category" | "subcategory" | "unit", v: Partial<StoredCategory> & Pick<StoredCategory, "id" | "code" | "name" | "is_active">, source: typeof sources[number] | null) => {
            const old = existing[kind].find(r => r.code === v.code);
            if (old) {
                if (old.source_fingerprint !== v.source_fingerprint || old.name !== v.name || old.is_active !== v.is_active || (old.category_id ?? "") !== (v.category_id ?? "") || (old.unit_id ?? "") !== (v.unit_id ?? ""))
                    throw new AppError(409, "Existing catalogue conflicts with source; review before import");
                return old;
            }
            const saved = await repo.insert(kind, v, actor.id, "", source);
            existing[kind].push(saved);
            result[kind === "unit" ? "units" : kind === "category" ? "categories" : "subcategories"]++;
            return saved;
        };
        for (const s of sources)
            if (s.data.kind === "type" && !units.has(s.data.uom!)) {
                const uom = s.data.uom!;
                const v = await save("unit", { id: randomUUID(), code: uom, name: unitNames[uom], is_active: true, source_fingerprint: `controlled-unit-v1:${uom}` }, null);
                units.set(uom, v.id);
            }
        for (const s of sources)
            if (s.data.kind === "category") {
                const v = await save("category", { id: randomUUID(), code: s.code, name: s.name, is_active: s.is_active, source_fingerprint: hash(s), unit_id: units.get(categoryUnits.get(s.code)!) }, s);
                parents.set(s.code, v.id);
            }
        for (const s of sources)
            if (s.data.kind === "type") {
                const d = s.data;
                await save("subcategory", { id: randomUUID(), code: s.code, name: s.name, is_active: s.is_active, source_fingerprint: hash(s), category_id: parents.get(d.category_code!), unit_id: units.get(d.uom!), configuration: { fields: d.fields!, model_required: d.model_required ?? false, family_code: d.family_code!, revision: d.revision! } }, s);
            }
        return result;
    });
}
