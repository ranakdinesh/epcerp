import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../../db";
import { AppError } from "../../errors";
import { recordMasterEvent } from "../../events/journal";
import { query } from "../catalogue/database";
import type { Category, CategoryKind, CategorySource, StoredCategory } from "./domain";
export function categoryView(r: Category): Category {
    return { id: r.id, code: r.code, name: r.name, is_active: r.is_active, version: r.version, created_at: r.created_at, updated_at: r.updated_at, created_by_id: r.created_by_id, updated_by_id: r.updated_by_id, ...(r.category_id ? { category_id: r.category_id } : {}), ...(r.unit_id ? { unit_id: r.unit_id } : {}), ...(r.configuration ? { configuration: r.configuration } : {}) };
}
const names = { category: { list: "CategoriesListCategories", insert: "CategoriesInsertCategory", update: "CategoriesUpdateCategory" }, subcategory: { list: "CategoriesListSubcategories", insert: "CategoriesInsertSubcategory", update: "CategoriesUpdateSubcategory" }, unit: { list: "CategoriesListUnits", insert: "CategoriesInsertUnit", update: "CategoriesUpdateUnit" } } as const;
export class CategoryRepository {
    constructor(private db: DB) { }
    async lock() { await query(this.db, "CategoriesLockImport"); }
    async list(kind: CategoryKind) { return (await query<StoredCategory>(this.db, names[kind].list)).rows; }
    async all() { return { categories: (await this.list("category")).map(categoryView), subcategories: (await this.list("subcategory")).map(categoryView), units: (await this.list("unit")).map(categoryView) }; }
    async insert(kind: CategoryKind, value: Partial<StoredCategory> & Pick<StoredCategory, "id" | "code" | "name" | "is_active">, actor: string, reason: string, source: CategorySource | null = null) {
        try {
            await query(this.db, names[kind].insert, { ...value, actor_id: actor, legacy_source: source, source_fingerprint: value.source_fingerprint ?? null });
        }
        catch (error) {
            if (["23505", "23503"].includes((error as {
                code?: string;
            }).code ?? ""))
                throw new AppError(409, "Existing catalogue conflicts with source; review before import");
            throw error;
        }
        const saved = (await this.list(kind)).find(v => v.id === value.id)!;
        if (reason)
            await this.history(kind, "create", {}, categoryView(saved), actor, reason);
        await recordMasterEvent(this.db, "category_catalogue", kind, saved.id, 1, reason ? "create" : "import", actor);
        return saved;
    }
    async update(kind: CategoryKind, before: StoredCategory, code: string, name: string, actor: string, reason: string) {
        const changed = await query(this.db, names[kind].update, { id: before.id, code, name, actor_id: actor, expected_version: before.version });
        if (changed.rowCount !== 1)
            throw new AppError(409, "Record changed; reload before saving");
        const saved = (await this.list(kind)).find(v => v.id === before.id)!;
        await this.history(kind, "update", categoryView(before), categoryView(saved), actor, reason);
        await recordMasterEvent(this.db, "category_catalogue", kind, saved.id, saved.version, "update", actor, undefined, saved.updated_at);
        return saved;
    }
    private async history(kind: CategoryKind, action: string, before: object, after: Category, actor: string, reason: string) { await query(this.db, "CategoriesInsertCreationHistory", { id: randomUUID(), kind, subject_id: after.id, action, reason, before_snapshot: before, after_snapshot: after, actor_id: actor }); }
    async browse(values: Record<string, unknown>) {
        const total = Number((await query<{
            count: string;
        }>(this.db, "CategoriesCountBrowse", values)).rows[0].count);
        const rows = (await query<Category & {
            unit_code: string;
            unit_name: string;
            child_count: string;
        }>(this.db, "CategoriesBrowseRecords", values)).rows;
        return { rows: rows.map(summaryView), total, page: values.page, size: values.size };
    }
    async detail(kind: CategoryKind, id: string) {
        const [row] = (await query<Category & {
            unit_code: string;
            unit_name: string;
            child_count: string;
        }>(this.db, "CategoriesCatalogueDetail", { kind, identity: id })).rows;
        if (!row)
            throw new AppError(404, "Catalogue record not found");
        return summaryView(row);
    }
}
function summaryView(r: Category & {
    unit_code: string;
    unit_name: string;
    child_count: string;
}) { return { id: r.id, code: r.code, name: r.name, is_active: r.is_active, version: r.version, ...(r.category_id ? { category_id: r.category_id } : {}), ...(r.unit_id ? { unit_id: r.unit_id } : {}), ...(r.configuration ? { configuration: r.configuration } : {}), unit_code: r.unit_code, unit_name: r.unit_name, child_count: Number(r.child_count) }; }
