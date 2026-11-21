import "server-only";
import { randomUUID } from "node:crypto";
import { transaction } from "../../db";
import { requireAnyPermission } from "../../auth/access";
import { vendorID, sourceInstant } from "../vendors/domain";
import { documentView, validateUpload, documentReason, parseImport, missing, conflict, type Upload } from "./domain";
import { DocumentRepository } from "./repository";
import { submitted, retired } from "./approval";
import { seal, open, hash, inspect, documentBucket, putObject, getObject } from "./files";
const read = ["settings.supplier_documents.read", "accounts.vendor_verification.read"], write = ["settings.supplier_documents.write"];
export async function authorizeUpload(token: string, supplier: string) { return transaction(async (db) => { await requireAnyPermission(db, token, write, true); await new DocumentRepository(db).supplier(supplier); }); }
export async function listDocuments(token: string, supplier: string) { return transaction(async (db) => { await requireAnyPermission(db, token, read); const repo = new DocumentRepository(db); await repo.supplier(supplier); return (await repo.list(supplier)).map(documentView); }); }
export async function uploadDocument(token: string, supplier: string, input: Upload, reason: string) {
    // Authorize before decoding and again under the write lock before any mutation.
    await authorizeUpload(token, supplier);
    documentReason(reason ||= "Supplier document uploaded");
    validateUpload(input);
    input = { ...input, content_type: await inspect(input.content) };
    // Never delete on an uncertain database commit. Offline storage maintenance can
    // reconcile an orphan after rollback, matching the existing Go policy.
    const id = randomUUID(), bucket = documentBucket(), key = `suppliers/${supplier}/${id}`, sealed = seal(input.content);
    return transaction(async (db) => {
        const actor = await requireAnyPermission(db, token, write, true), repo = new DocumentRepository(db);
        await repo.supplier(supplier);
        if (input.doc_type !== "other" || input.replaces_document_id) {
            const old = await repo.list(supplier);
            if (input.replaces_document_id && !old.some(d => d.id === input.replaces_document_id && d.is_active))
                throw conflict();
            for (const d of old)
                if (d.is_active && (d.id === input.replaces_document_id || (input.doc_type !== "other" && d.doc_type === input.doc_type))) {
                    const saved = await repo.archive(supplier, d.id, d.version, actor.id);
                    await repo.changed(saved, actor.id, "archived", "Replaced by a new document");
                    await retired(db, saved, actor.id, "replaced");
                }
        }
        await putObject(bucket, key, sealed.encrypted);
        const saved = await repo.insert(id, supplier, input, actor.id, sealed.sha256, bucket, key);
        await repo.changed(saved, actor.id, "uploaded", reason);
        await submitted(db, saved, actor.id);
        return documentView(saved);
    });
}
export async function archiveDocument(token: string, supplier: string, id: string, version: number, reason: string) { return transaction(async (db) => { const actor = await requireAnyPermission(db, token, write, true); if (!vendorID.test(id))
    throw missing(); documentReason(reason, true); const repo = new DocumentRepository(db); await repo.supplier(supplier); const saved = await repo.archive(supplier, id, version, actor.id); await repo.changed(saved, actor.id, "archived", reason); await retired(db, saved, actor.id, "archived"); return documentView(saved); }); }
export async function downloadDocument(token: string, supplier: string, id: string, preview = false) {
    return transaction(async (db) => {
        const actor = await requireAnyPermission(db, token, read), d = await new DocumentRepository(db).get(supplier, id);
        if (!d || (!d.is_active && !actor.allows("settings.supplier_documents.import") && !actor.allows("accounts.vendor_verification.read") && (d.source_id || d.created_by_id !== actor.id)))
            throw missing();
        const encrypted = d.object_key ? await getObject(d.object_bucket!, d.object_key) : d.encrypted_content;
        const data = open(encrypted ?? Buffer.alloc(0), d.sha256), contentType = preview ? await inspect(data) : "application/octet-stream";
        return { document: d, data, contentType };
    });
}
export async function importDocument(token: string, supplier: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await requireAnyPermission(db, token, ["settings.supplier_documents.import"], true), input = parseImport(value), repo = new DocumentRepository(db);
        await repo.supplier(supplier);
        const old = await repo.source(input.source.id);
        if (old) {
            if (old.supplier_id !== supplier || old.sha256 !== hash(input.content) || old.doc_type !== input.doc_type || old.title !== input.title || old.original_filename !== input.original_filename || old.content_type !== input.content_type || old.is_active !== input.is_active || sourceInstant(old.source_created_at!) !== sourceInstant(input.source.created_at) || sourceInstant(old.source_updated_at!) !== sourceInstant(input.source.updated_at))
                throw conflict();
            return documentView(old);
        }
        const id = randomUUID(), bucket = documentBucket(), key = `suppliers/${supplier}/${id}`, sealed = seal(input.content);
        // Imports deliberately do not submit Finance approvals, matching the legacy import contract.
        await putObject(bucket, key, sealed.encrypted);
        const saved = await repo.insert(id, supplier, input, actor.id, sealed.sha256, bucket, key, input.is_active, input.source);
        await repo.imported(saved, actor.id);
        return documentView(saved);
    });
}
