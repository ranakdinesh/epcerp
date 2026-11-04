import "server-only";
import { randomUUID } from "node:crypto";
import { transaction } from "../../db";
import { AppError } from "../../errors";
import { requirePermission } from "../../auth/access";
import { lockReferences } from "../shared/references";
import { canonicalID } from "../shared/validation";
import { prepareReference, referenceKind, referencePermission } from "./domain";
import { parseReference, parseReferenceQuery } from "./validation";
import { ReferenceRepository } from "./repository";
export function listReferences(token: string, rawKind: string, query: URLSearchParams) {
    const kind = referenceKind(rawKind), filter = parseReferenceQuery(query);
    return transaction(async (db) => {
        await requirePermission(db, token, referencePermission(kind));
        return new ReferenceRepository(db).list(kind, filter.state, filter.activeOnly);
    });
}
export async function saveReference(token: string, rawKind: string, rawID: string | null, value: unknown) {
    const kind = referenceKind(rawKind), id = rawID === null ? randomUUID() : canonicalID(rawID, "reference_id", "path");
    const input = prepareReference(kind, parseReference(value));
    const result = await transaction(async (db) => {
        const by = await requirePermission(db, token, referencePermission(kind, true), true);
        await lockReferences(db);
        const repo = new ReferenceRepository(db);
        if (kind === "cities" && !(await repo.get("states", input.state_id!))?.is_active)
            throw new AppError(422, "Select an active state or union territory");
        if (rawID !== null && !await repo.get(kind, id))
            throw new AppError(404, "Master value not found");
        await repo.save(kind, id, input, by.id, rawID === null);
        return { saved: (await repo.get(kind, id))!, actor: by.id };
    });
    console.info(JSON.stringify({ event: rawID === null ? "master.references.created" : "master.references.updated", reference_kind: kind, reference_id: id, actor_id: result.actor }));
    return result.saved;
}
export async function setReferenceActive(token: string, rawKind: string, rawID: string, active: boolean) {
    const kind = referenceKind(rawKind), id = canonicalID(rawID, "reference_id", "path");
    const result = await transaction(async (db) => {
        const by = await requirePermission(db, token, referencePermission(kind, true), true);
        await lockReferences(db);
        const repo = new ReferenceRepository(db), record = await repo.get(kind, id);
        if (!record)
            throw new AppError(404, "Master value not found");
        if (active && kind === "cities" && !(await repo.get("states", record.state_id!))?.is_active)
            throw new AppError(422, "Activate the city's state first; select an active state or union territory");
        if (!active && kind === "states")
            await repo.retireCities(id, by.id);
        await repo.status(kind, id, by.id, active);
        return { saved: (await repo.get(kind, id))!, actor: by.id };
    });
    console.info(JSON.stringify({ event: active ? "master.references.activated" : "master.references.deactivated", reference_kind: kind, reference_id: id, actor_id: result.actor }));
    return result.saved;
}
