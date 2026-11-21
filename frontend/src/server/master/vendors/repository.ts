import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../../db";
import { AppError } from "../../errors";
import { recordMasterEvent } from "../../events/journal";
import { query } from "../catalogue/database";
import { sourceStorage, vendorConflict, type StoredVendor, type Vendor, type VendorSource } from "./domain";
export function vendorView(r: StoredVendor): Vendor { return { id: r.id, code: r.code, name: r.name, data: r.data, is_active: r.is_active, version: r.version, created_at: r.created_at, updated_at: r.updated_at, created_by_id: r.created_by_id, updated_by_id: r.updated_by_id, is_imported: r.source_id !== null }; }
export class VendorRepository {
    constructor(private db: DB) { }
    async list() { return (await query<StoredVendor>(this.db, "VendorProfileListSuppliers")).rows; }
    async get(id: string) { return (await query<StoredVendor>(this.db, "VendorProfileGetSupplier", { id })).rows[0] ?? null; }
    async bySource(id: string) { await query(this.db, "VendorProfileLockImport"); return (await query<StoredVendor>(this.db, "VendorProfileSourceSupplier", { source_id: id })).rows[0] ?? null; }
    async insert(code: string, name: string, data: Record<string, unknown>, active: boolean, actor: string, source: VendorSource | null = null) {
        try {
            return (await query<StoredVendor>(this.db, "VendorProfileInsertSupplier", { id: randomUUID(), code, name, data, active, actor, source_id: source?.id ?? null, source_version: source?.version ?? null, source_created_at: source ? sourceStorage(source.created_at) : null, source_updated_at: source ? sourceStorage(source.updated_at) : null })).rows[0];
        }
        catch (error) {
            if ((error as {
                code?: string;
            }).code === "23505")
                throw vendorConflict();
            throw error;
        }
    }
    async update(old: StoredVendor, name: string, data: Record<string, unknown>, active: boolean, actor: string) {
        const r = (await query<StoredVendor>(this.db, "VendorProfileUpdateSupplier", { id: old.id, name, data, active, actor, previous_version: old.version })).rows[0];
        if (!r)
            throw new AppError(409, "Supplier changed; refresh before editing");
        return r!;
    }
    async changed(saved: StoredVendor, actor: string, reason: string, action: "created" | "updated") {
        const id = randomUUID();
        await query(this.db, "VendorProfileRecordChange", { id, supplier_id: saved.id, version: saved.version, action, reason, snapshot: vendorView(saved), created_by_id: actor });
        await recordMasterEvent(this.db, "suppliers", "supplier", saved.id, saved.version, action, actor, id, saved.updated_at);
    }
    async imported(saved: StoredVendor, actor: string) { await recordMasterEvent(this.db, "suppliers", "supplier", saved.id, 1, "legacy_imported", actor, undefined, saved.created_at); }
}
