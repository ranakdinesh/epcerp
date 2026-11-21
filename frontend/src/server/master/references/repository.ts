import "server-only";
import { rows, type DB } from "../../db";
import { AppError } from "../../errors";
import { reference, referenceTables, type Reference, type ReferenceKind } from "../shared/references";
import { normalizedName } from "../shared/text";
import type { ReferenceInput } from "./domain";
export class ReferenceRepository {
    constructor(private db: DB) { }
    get(kind: ReferenceKind, id: string) { return reference(this.db, kind, id); }
    list(kind: ReferenceKind, state: string | null, activeOnly: boolean) {
        const city = kind === "cities";
        return rows<Reference>(this.db, `SELECT id,name,code,is_active,${city ? "state_id" : "NULL::uuid AS state_id"}
      FROM public.${referenceTables[kind]} WHERE (NOT $1::boolean OR is_active)${city ? " AND ($2::uuid IS NULL OR state_id=$2)" : ""} ORDER BY name`, city ? [activeOnly, state] : [activeOnly]);
    }
    async save(kind: ReferenceKind, id: string, input: ReferenceInput, by: string, create: boolean) {
        const table = referenceTables[kind];
        const values = [id, input.name, normalizedName(input.name), input.code, by, ...(kind === "cities" ? [input.state_id] : [])];
        try {
            if (create)
                await this.db.query(`INSERT INTO public.${table}(id,name,normalized_name,code,is_active,created_by_id,updated_by_id${kind === "cities" ? ",state_id" : ""})
        VALUES($1,$2,$3,$4,true,$5,$5${kind === "cities" ? ",$6" : ""})`, values);
            else
                await this.db.query(`UPDATE public.${table} SET name=$2,normalized_name=$3,code=$4,updated_by_id=$5,updated_at=now()${kind === "cities" ? ",state_id=$6" : ""} WHERE id=$1`, values);
        }
        catch (error) {
            if (["23505", "23503"].includes((error as {
                code?: string;
            }).code ?? ""))
                throw new AppError(409, "This master value already exists");
            throw error;
        }
    }
    async retireCities(state: string, by: string) {
        await this.db.query("UPDATE public.indian_cities SET is_active=false,updated_by_id=$2,updated_at=now() WHERE state_id=$1 AND is_active", [state, by]);
    }
    async status(kind: ReferenceKind, id: string, by: string, active: boolean) {
        await this.db.query(`UPDATE public.${referenceTables[kind]} SET is_active=$3,updated_by_id=$2,updated_at=now() WHERE id=$1`, [id, by, active]);
    }
}
