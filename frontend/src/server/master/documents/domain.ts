import { AppError } from "../../errors";
import { boolean, byteLength, fieldError, goObject, invalid, runeLength, text } from "../catalogue/validation";
import { sourceTimestamp } from "../categories/domain";
import { vendorID, sourceInstant } from "../vendors/domain";
import { trim } from "../shared/text";
import { maximumBytes } from "./files";
export type Document = {
    id: string;
    supplier_id: string;
    version: number;
    doc_type: string;
    title: string;
    original_filename: string;
    content_type: string;
    byte_size: number;
    sha256: string;
    is_active: boolean;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
    approval_state?: string;
    approval_comment?: string;
    encrypted_content: Buffer | null;
    object_bucket: string | null;
    object_key: string | null;
    source_id: string | null;
    source_created_at: string | null;
    source_updated_at: string | null;
};
export type Upload = {
    doc_type: string;
    title: string;
    original_filename: string;
    content_type: string;
    content: Buffer;
    replaces_document_id: string;
};
export const missing = () => new AppError(404, "Supplier document not found");
export const conflict = () => new AppError(409, "Source document conflicts with an existing import");
export const stale = () => new AppError(409, "Document changed; refresh before archiving");
export function documentView(d: Document) { return { approval_state: d.approval_state ?? "", approval_comment: d.approval_comment ?? "", version: d.version, id: d.id, supplier_id: d.supplier_id, doc_type: d.doc_type, title: d.title, original_filename: d.original_filename, content_type: d.content_type, byte_size: d.byte_size, sha256: d.sha256, is_active: d.is_active, is_encrypted: true, created_at: d.created_at, updated_at: d.updated_at, created_by_id: d.created_by_id, updated_by_id: d.updated_by_id }; }
export function validateUpload(input: Upload) {
    if (input.replaces_document_id && !vendorID.test(input.replaces_document_id))
        fieldError("file", "Invalid replacement document");
    if (!input.content.length || input.content.length > maximumBytes)
        fieldError("file", "Choose a nonempty file up to 15 MiB");
    if (!["aadhar", "pan", "gst", "cheque", "other"].includes(input.doc_type))
        fieldError("doc_type", "Choose a document type");
    for (const [value, max] of [[input.title, 255], [input.original_filename, 255], [input.content_type, 120]] as const)
        if (runeLength(value) > max || /[\x00\r\n]/.test(value))
            invalid("Invalid document metadata");
    if (!trim(input.original_filename))
        invalid("Filename is required");
}
export function documentReason(value: string, archive = false) { if (!trim(value) || byteLength(value) > 2000)
    fieldError("reason", archive ? "Archive reason is required" : "Invalid upload reason"); return value; }
function importedTime(value: unknown) {
    const result = sourceTimestamp(value), [y, m, d, h, min, s] = result.slice(0, 19).split(/[-T:]/).map(Number);
    const days = [31, y % 4 === 0 && (y % 100 !== 0 || y % 400 === 0) ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    if (m < 1 || m > 12 || d < 1 || d > days[m - 1] || h > 23 || min > 59 || s > 59 || sourceInstant(result) === "0001-01-01T00:00:00.000000000")
        invalid("Document source and status are required");
    return result;
}
export function parseImport(value: unknown) {
    const r = goObject(value, ["source", "doc_type", "title", "original_filename", "content_type", "content", "is_active"]), s = goObject(r.source, ["id", "created_at", "updated_at"]);
    const id = text(s.id);
    if (!vendorID.test(id) || r.is_active == null)
        invalid("Document source and status are required");
    let content: Buffer;
    if (Array.isArray(r.content)) {
        if (r.content.some(v => typeof v !== "number" || !Number.isInteger(v) || v < 0 || v > 255))
            invalid("Invalid import payload");
        content = Buffer.from(r.content);
    }
    else {
        const encoded = text(r.content).replace(/[\r\n]/g, "");
        if (!/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(encoded))
            invalid("Invalid import payload");
        content = Buffer.from(encoded, "base64");
    }
    const input: Upload = { doc_type: text(r.doc_type), title: text(r.title), original_filename: text(r.original_filename), content_type: text(r.content_type), content, replaces_document_id: "" };
    const source = { id, created_at: importedTime(s.created_at), updated_at: importedTime(s.updated_at) };
    validateUpload(input);
    return { ...input, source, is_active: boolean(r.is_active) };
}
