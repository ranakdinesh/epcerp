import "server-only";
import { lock, rows, type DB } from "../../db";
export const referenceTables = { "client-types": "client_types", industries: "industries", states: "indian_states", cities: "indian_cities" } as const;
export type ReferenceKind = keyof typeof referenceTables;
export type Reference = {
    id: string;
    name: string;
    code: string;
    is_active: boolean;
    state_id: string | null;
};
export async function lockReferences(db: DB) {
    // Same lock as Go reference maintenance throughout the gradual migration.
    await lock(db, "epcERP.client_references.administration");
}
export async function reference(db: DB, kind: ReferenceKind, id: string): Promise<Reference | null> {
    const [record] = await rows<Reference>(db, `SELECT id,name,code,is_active,${kind === "cities" ? "state_id" : "NULL::uuid AS state_id"} FROM public.${referenceTables[kind]} WHERE id=$1`, [id]);
    return record ?? null;
}
