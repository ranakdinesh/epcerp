import "server-only";
import { randomUUID } from "node:crypto";
import { transaction } from "../../db";
import { AppError } from "../../errors";
import { requirePermission } from "../../auth/access";
import { fieldError, reason, strictID } from "../catalogue/validation";
import { buildItemSku, deriveItem, eligible, normalizeModel, parseItem } from "./domain";
import { ItemRepository, itemView } from "./repository";
export function readItems(token: string, mode = "list", id = "") {
    return transaction(async (db) => {
        await requirePermission(db, token, "settings.items.read");
        const repo = new ItemRepository(db);
        if (mode === "references")
            return repo.references();
        if (mode === "history")
            return repo.history(strictID(id));
        return (await repo.list()).map(itemView);
    });
}
export function createItem(token: string, value: unknown) {
    const input = parseItem(value);
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.items.write", true), repo = new ItemRepository(db);
        await repo.lock();
        const config = await repo.configuration(input.configuration_id), make = await repo.manufacturer(input.manufacturer_id), d = deriveItem(config, make, input.specifications, input.model);
        const existing = await repo.find(d.fingerprint);
        if (existing)
            return itemView(existing);
        const candidates = await repo.candidates(config.data.family_code, config.data.uom, d.specifications);
        const normalizedModel = normalizeModel(d.model);
        const matching = candidates.filter(item => normalizeModel(item.model) === normalizedModel);
        const sameMake = matching.find(item => item.manufacturer_id === make.id);
        if (sameMake)
            return itemView(sameMake);
        const duplicateSku = () => new AppError(409, "An item already exists for this SKU. Use the existing item.");
        if (matching.length)
            throw duplicateSku();

        // Legacy items reserve their vendor-free SKU while keeping the code they were issued.
        const legacy = await repo.legacySkuCandidates(config.data.category_code);
        if (legacy.some(item => buildItemSku(item.snapshot.configuration, item.specifications, normalizeModel(item.model)) === d.prefix))
            throw duplicateSku();

        const saved = await repo.create({ id: randomUUID(), configuration_id: config.id, manufacturer_id: make.id, category_id: config.category_id!, model: d.model, specifications: d.specifications, snapshot: { configuration: { ...config.data, name: config.name, code: config.code }, manufacturer: make, sku_prefix: d.prefix }, fingerprint: d.fingerprint, engineering_key: d.engineering_key, description: d.description, sku: d.prefix, status: "approved", is_active: true }, actor.id);
        if (!saved)
            throw duplicateSku();
        await repo.event(saved, "created", "Created catalogue item", actor.id);
        return itemView(saved);
    });
}
export function changeItem(token: string, id: string, value: unknown) {
    const input = parseItem(value, true);
    return transaction(async (db) => {
        const actor = await requirePermission(db, token, "settings.items.write", true);
        id = strictID(id);
        const why = reason(input.reason), repo = new ItemRepository(db);
        await repo.lock();
        const before = await repo.get(id);
        if (before.version !== input.version)
            throw new AppError(409, "Record changed; reload before saving");
        if (!((before.status === "approved" && input.status === "retired") || (before.status === "retired" && input.status === "approved")))
            fieldError("status", "Invalid item status transition");
        if (input.status === "approved" && !eligible(await repo.configuration(before.configuration_id), await repo.manufacturer(before.manufacturer_id)))
            throw new AppError(422, "Configuration or manufacturer is inactive or no longer eligible");
        const saved = await repo.change(before, input.status, actor.id);
        await repo.event(saved, input.status, why, actor.id, before);
        return itemView(saved);
    });
}
