import "server-only";
import { types, type QueryResultRow } from "pg";
import type { DB } from "../../db";
import { catalogueSQL } from "./sql";
import { parseCatalogueJSON } from "./json";
// Keep PostgreSQL microseconds; Date would silently truncate issued evidence.
export function timestamp(value: string) {
    return value.replace(' ', 'T').replace(/\+00(?::00)?$/, 'Z').replace(/(\.\d*?[1-9])0+Z$/, '$1Z').replace(/\.0+Z$/, 'Z');
}
export async function query<T extends QueryResultRow>(db: DB, name: keyof typeof catalogueSQL, values: Record<string, unknown> = {}) {
    const statement = catalogueSQL[name];
    return db.query<T>({ text: statement.text, values: statement.keys.map(k => values[k] ?? null), types: { getTypeParser: (oid: number) => oid === 1184 ? timestamp : oid === 3802 || oid === 114 ? parseCatalogueJSON : types.getTypeParser(oid) } });
}
