import "server-only";
import { transaction } from "../../db";
import { AppError } from "../../errors";
import { requirePermission } from "../../auth/access";
import { canonicalID } from "../shared/validation";
import { hash } from "../catalogue/serialization";
import { invalid } from "../catalogue/validation";
import { eligibilityQuery, nameKey, parseManufacturer, parseManufacturerImport } from "./domain";
import { ManufacturerRepository } from "./repository";
export function readManufacturers(token: string, mode = "list", id = "", query = new URLSearchParams()) {
    return transaction(async (db) => {
        await requirePermission(db, token, "settings.manufacturers.read");
        const repo = new ManufacturerRepository(db);
        if (mode === "history")
            return repo.history(canonicalID(id, "identity", "path"));
        if (mode === "options")
            return repo.options(eligibilityQuery(query));
        return repo.list();
    });
}
export function saveManufacturer(token: string, kind: "create" | "profile" | "active", rawID: string | null, value: unknown) {
    const input = parseManufacturer(value, kind), id = rawID === null ? null : canonicalID(rawID, "identity", "path");
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.manufacturers.write", true), repo = new ManufacturerRepository(db);
        if (kind !== "active") {
            await repo.lock();
            for (const r of await repo.list())
                if (r.id !== id && ((kind === "create" && r.code === input.code) || nameKey(r.name) === nameKey(input.name)))
                    invalid(kind === "create" ? "Code or name already exists" : "Name already exists");
            await repo.validate(input.data);
        }
        if (kind === "create") {
            const saved = await repo.create(input.code, input.name, input.data, actor.id);
            await repo.event(saved, "create", input.reason, {}, { code: saved.code, name: saved.name, data: saved.data }, actor.id);
            return saved;
        }
        const old = await repo.get(id!, true);
        if (!old)
            throw new AppError(404, "Record not found");
        if (old!.version !== input.version || input.version === 2147483647)
            throw new AppError(409, "Record changed; refresh before saving");
        const before = kind === "active" ? { is_active: old.is_active, version: old.version } : { name: old.name, data: old.data, version: old.version };
        const saved = await repo.update(old, input.name, input.data, actor.id, kind === "active" ? input.is_active : undefined);
        const after = kind === "active" ? { is_active: saved.is_active, version: saved.version } : { name: saved.name, data: saved.data, version: saved.version };
        await repo.event(saved, kind === "active" ? "lifecycle" : "update", input.reason, before, after, actor.id);
        return saved;
    });
}
export function importManufacturer(token: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.manufacturers.import", true), source = parseManufacturerImport(value), sourceID = canonicalID(source.source.id), fingerprint = hash(source), repo = new ManufacturerRepository(db);
        await repo.lock();
        const existing = await repo.findImport(sourceID);
        if (existing) {
            if (existing.source_fingerprint !== fingerprint)
                throw new AppError(409, "Duplicate or conflicting record; refresh and review the existing entry");
            return repo.get(existing.go_id);
        }
        for (const r of await repo.list())
            if (r.code === source.code || nameKey(r.name) === nameKey(source.name))
                throw new AppError(409, "Duplicate or conflicting record; refresh and review the existing entry");
        await repo.validate(source.data);
        const saved = await repo.create(source.code, source.name, source.data, actor.id, source.is_active);
        await repo.importRecord(saved.id, source, sourceID, fingerprint, actor.id);
        await repo.event(saved, "import", "Reviewed Python manufacturer master transfer", {}, { code: saved.code, name: saved.name, data: saved.data, source_id: source.source.id, source_fingerprint: fingerprint }, actor.id);
        return saved;
    });
}
