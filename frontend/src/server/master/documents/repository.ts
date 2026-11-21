import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../../db";
import { recordMasterEvent } from "../../events/journal";
import { query } from "../catalogue/database";
import { VendorRepository } from "../vendors/repository";
import { sourceStorage } from "../vendors/domain";
import { missing, stale, type Document, type Upload } from "./domain";
export class DocumentRepository {
    constructor(private db: DB) { }
    async supplier(id: string) { if (!await new VendorRepository(this.db).get(id))
        throw missing(); }
    async list(supplier_id: string) { return (await query<Document>(this.db, "VendorDocumentsListDocuments", { supplier_id })).rows; }
    async get(supplier_id: string, id: string) { return (await query<Document>(this.db, "VendorDocumentsGetDocument", { supplier_id, id })).rows[0] ?? null; }
    async source(source_id: string) { await query(this.db, "VendorDocumentsLockImport"); return (await query<Document>(this.db, "VendorDocumentsSourceDocument", { source_id })).rows[0] ?? null; }
    async insert(id: string, supplier_id: string, input: Upload, actor: string, sha256: string, bucket: string, key: string, active = true, source?: {
        id: string;
        created_at: string;
        updated_at: string;
    }) {
        return (await query<Document>(this.db, "VendorDocumentsInsertDocument", { id, supplier_id, doc_type: input.doc_type, title: input.title, filename: input.original_filename, content_type: input.content_type, byte_size: input.content.length, encrypted_content: null, sha256, active, actor, source_id: source?.id ?? null, source_created_at: source ? sourceStorage(source.created_at) : null, source_updated_at: source ? sourceStorage(source.updated_at) : null, object_bucket: bucket, object_key: key })).rows[0];
    }
    async archive(supplier: string, id: string, version: number, actor: string) { const saved = (await query<Document>(this.db, "VendorDocumentsArchiveDocument", { supplier, id, version, actor })).rows[0]; if (!saved)
        throw stale(); return saved; }
    async changed(d: Document, actor: string, action: string, reason: string) { const id = randomUUID(); await query(this.db, "VendorDocumentsRecordChange", { id, document_id: d.id, version: d.version, action, reason, created_by_id: actor }); await recordMasterEvent(this.db, "supplier_documents", "supplier_document", d.id, d.version, action, actor, id, d.updated_at); }
    async imported(d: Document, actor: string) { await recordMasterEvent(this.db, "supplier_documents", "supplier_document", d.id, 1, "legacy_imported", actor, undefined, d.created_at); }
}
