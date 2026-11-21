import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../../db";
import { AppError } from "../../errors";
import { recordMasterEvent } from "../../events/journal";
import { query } from "../catalogue/database";
import { invalid } from "../catalogue/validation";
import type { Manufacturer, ManufacturerData } from "./domain";
export type Eligibility = {
    code: string;
    name: string;
    kind: string;
    parent_name: string;
    parent_code: string;
    active: boolean;
};
const choice = (r: Eligibility) => ({ code: r.code, name: r.name, kind: r.kind, parent_name: r.parent_name, parent_code: r.parent_code, is_active: r.active });
export class ManufacturerRepository {
    constructor(private db: DB) { }
    async lock() { await query(this.db, "ManufacturersLockNames"); }
    async list() { return (await query<Manufacturer>(this.db, "ManufacturersListManufacturers")).rows; }
    async get(id: string, lock = false) { return (await query<Manufacturer>(this.db, lock ? "ManufacturersLockManufacturer" : "ManufacturersGetManufacturer", { id })).rows[0] ?? null; }
    async validate(data: ManufacturerData) {
        const records = (await query<Eligibility>(this.db, "CategoriesFindEligibility", { codes: data.eligible })).rows, seen = new Set<string>();
        for (const code of data.eligible) {
            if (records.filter(r => r.code === code).length !== 1)
                invalid("Eligibility code is missing or ambiguous");
        }
        for (const code of data.eligible) {
            if (seen.has(code))
                invalid("Duplicate manufacturer eligibility");
            seen.add(code);
        }
        if (data.eligible.some(c => !records.some(r => r.code === c && r.active)))
            invalid("Manufacturer eligibility must reference active categories or types");
    }
    async options(values: Record<string, unknown> & {
        codes: string[];
    }) {
        if (values.codes.length) {
            const rows = (await query<Eligibility>(this.db, "CategoriesFindEligibility", values)).rows;
            return { rows: rows.map(choice), total: rows.length };
        }
        const rows = (await query<Eligibility>(this.db, "CategoriesBrowseEligibility", values)).rows, total = Number((await query<{
            count: string;
        }>(this.db, "CategoriesCountEligibility", values)).rows[0].count);
        return { rows: rows.map(choice), total };
    }
    async create(code: string, name: string, data: ManufacturerData, actor: string, active = true) { return (await query<Manufacturer>(this.db, "ManufacturersInsertManufacturer", { id: randomUUID(), code, name, data, is_active: active, actor_id: actor })).rows[0]; }
    async update(old: Manufacturer, name: string, data: ManufacturerData, actor: string, active?: boolean) { return (await query<Manufacturer>(this.db, active === undefined ? "ManufacturersProfileManufacturer" : "ManufacturersStatusManufacturer", { id: old.id, name, data, active, version: old.version + 1, actor_id: actor })).rows[0]; }
    async event(saved: Manufacturer, action: string, reason: string, before: object, after: object, actor: string) {
        const id = randomUUID();
        await query(this.db, "ManufacturersInsertEvent", { id, subject_id: saved.id, action, reason, before_snapshot: before, after_snapshot: after, actor_id: actor });
        await recordMasterEvent(this.db, "manufacturers", "manufacturer", saved.id, saved.version, action, actor, id, saved.updated_at);
    }
    async history(id: string) { if (!await this.get(id))
        throw new AppError(404, "Record not found"); const rows = (await query<{
        id: string;
        subject_id: string;
        action: string;
        reason: string;
        before: object;
        after: object;
        created_at: string;
        created_by_id: string | null;
    }>(this.db, "ManufacturersHistory", { subject_id: id })).rows; return rows; }
    async findImport(source: string) { return (await query<{
        go_id: string;
        source_fingerprint: string;
    }>(this.db, "ManufacturersFindImport", { source_id: source })).rows[0] ?? null; }
    async importRecord(id: string, source: object, sourceID: string, fingerprint: string, actor: string) { await query(this.db, "ManufacturersInsertImport", { source_id: sourceID, go_id: id, source_snapshot: source, source_fingerprint: fingerprint, actor_id: actor }); }
}
