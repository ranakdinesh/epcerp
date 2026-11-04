import "server-only";
import { randomUUID } from "node:crypto";
import { transaction } from "../../db";
import { AppError } from "../../errors";
import { requirePermission } from "../../auth/access";
import { ClientRepository } from "./repository";
import { parseClient, parseFilter } from "./validation";
import { stateMismatch } from "./domain";
import { canonicalID } from "../shared/validation";
import { lockReferences, reference } from "../shared/references";
export function listClients(token: string, query: URLSearchParams) {
    const filter = parseFilter(query);
    return transaction(async (db) => {
        await requirePermission(db, token, "settings.clients.read");
        return new ClientRepository(db).list(filter);
    });
}
export function getClient(token: string, rawID: string) {
    const id = canonicalID(rawID, "client_id", "path");
    return transaction(async (db) => {
        await requirePermission(db, token, "settings.clients.read");
        const client = await new ClientRepository(db).get(id);
        if (!client)
            throw new AppError(404, "Client not found");
        return client;
    });
}
export async function saveClient(token: string, rawID: string | null, value: unknown) {
    const id = rawID === null ? randomUUID() : canonicalID(rawID, "client_id", "path"), input = parseClient(value);
    const result = await transaction(async (db) => {
        const by = await requirePermission(db, token, "settings.clients.write", true);
        await lockReferences(db);
        for (const [kind, key, target] of [["client-types", "client_type_id", "client_type"], ["industries", "industry_id", "industry"], ["states", "state_id", "state"], ["cities", "city_id", "city"]] as const) {
            const selected = await reference(db, kind, input[key]);
            if (!selected?.is_active)
                throw new AppError(422, `The selected ${target.replace(/_/g, " ")} is not active`);
            if (kind === "cities" && selected.state_id !== input.state_id)
                throw new AppError(422, "The selected city does not belong to the selected state");
            input[target] = selected.name;
        }
        const mismatch = stateMismatch(input.gstin, input.state);
        if (mismatch)
            throw new AppError(422, mismatch);
        const repo = new ClientRepository(db), existing = rawID === null ? null : await repo.get(id, true);
        if (rawID !== null && !existing)
            throw new AppError(404, "Client not found");
        if (existing && input.contacts.some(c => c.id && !existing.contacts.some(old => old.id === c.id)))
            throw new AppError(422, "A contact does not belong to this client");
        const contacts = input.contacts.map(c => ({ ...c, id: rawID === null || !c.id ? randomUUID() : c.id, is_active: true }));
        await repo.save(id, existing?.client_code ?? `CLI-${randomUUID().replace(/-/g, "").slice(0, 12).toUpperCase()}`, input, existing?.is_active ?? true, by.id, rawID === null);
        await repo.contacts(id, contacts, by.id);
        const saved = (await repo.get(id))!;
        return { saved, actorID: by.id };
    });
    console.info(JSON.stringify({ event: rawID === null ? "master.clients.created" : "master.clients.updated", client_id: id, actor_id: result.actorID }));
    return result.saved;
}
export async function setClientActive(token: string, rawID: string, active: boolean) {
    const id = canonicalID(rawID, "client_id", "path");
    const result = await transaction(async (db) => {
        const by = await requirePermission(db, token, "settings.clients.write", true), repo = new ClientRepository(db), client = await repo.get(id, true);
        if (!client)
            throw new AppError(404, "Client not found");
        await repo.save(id, client.client_code, client, active, by.id, false);
        if (!active)
            await repo.contacts(id, [], by.id);
        return { saved: (await repo.get(id))!, actorID: by.id };
    });
    console.info(JSON.stringify({ event: active ? "master.clients.activated" : "master.clients.deactivated", client_id: id, actor_id: result.actorID }));
    return result.saved;
}
