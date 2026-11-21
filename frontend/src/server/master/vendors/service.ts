import "server-only";
import { transaction } from "../../db";
import { AppError } from "../../errors";
import { requirePermission } from "../../auth/access";
import { dataFingerprint, missingVendor, parseVendorCreate, parseVendorImport, parseVendorUpdate, sourceInstant, validateVendorMaster, vendorConflict, vendorID } from "./domain";
import { VendorRepository, vendorView } from "./repository";
export function listVendors(token: string) {
    return transaction(async (db) => {
        try {
            await requirePermission(db, token, "settings.suppliers.read");
        }
        catch (error) {
            if (!(error instanceof AppError) || error.status !== 403)
                throw error;
            await requirePermission(db, token, "accounts.vendor_verification.read");
        }
        return (await new VendorRepository(db).list()).map(vendorView);
    });
}
export function createVendor(token: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.suppliers.write", true), input = parseVendorCreate(value), repo = new VendorRepository(db), saved = await repo.insert(input.code, input.name, input.data, true, actor.id);
        await repo.changed(saved, actor.id, input.reason, "created");
        return vendorView(saved);
    });
}
export function updateVendor(token: string, id: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.suppliers.write", true);
        if (!vendorID.test(id))
            throw missingVendor();
        const input = parseVendorUpdate(value), repo = new VendorRepository(db), old = await repo.get(id);
        if (!old)
            throw missingVendor();
        if (input.version !== old!.version)
            throw new AppError(409, "Supplier changed; refresh before editing");
        const data = validateVendorMaster(old!.code, input.name, input.data), saved = await repo.update(old!, input.name, data, input.is_active ?? old!.is_active, actor.id);
        await repo.changed(saved, actor.id, input.reason, "updated");
        return vendorView(saved);
    });
}
export function importVendor(token: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.suppliers.import", true), input = parseVendorImport(value), repo = new VendorRepository(db), existing = await repo.bySource(input.source.id);
        if (existing) {
            if (existing.code !== input.code || existing.name !== input.name || existing.is_active !== input.is_active || existing.source_version !== input.source.version || sourceInstant(existing.source_created_at!) !== sourceInstant(input.source.created_at) || sourceInstant(existing.source_updated_at!) !== sourceInstant(input.source.updated_at) || dataFingerprint(existing.data) !== dataFingerprint(input.data))
                throw vendorConflict();
            return vendorView(existing);
        }
        const saved = await repo.insert(input.code, input.name, input.data, input.is_active, actor.id, input.source);
        await repo.imported(saved, actor.id);
        return vendorView(saved);
    });
}
