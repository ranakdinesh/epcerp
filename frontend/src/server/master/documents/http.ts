import "server-only";
import { AppError } from "../../errors";
import { goObject, integer, invalid, text } from "../catalogue/validation";
import { vendorID } from "../vendors/domain";
import { maximumBytes } from "./files";
import { listDocuments, authorizeUpload, uploadDocument, archiveDocument, downloadDocument, importDocument } from "./service";
import type { Upload } from "./domain";
export const ownsDocumentPath = (path: string) => /^\/suppliers\/[^/]+\/documents(?:\/(?:legacy-import|[^/]+\/(?:preview|download|archive)))?$/.test(path);
async function multipart(request: Request) {
    if (!request.headers.get("content-type")?.toLowerCase().startsWith("multipart/form-data;"))
        invalid("Multipart upload is required");
    // Bound the stream before Request.formData buffers any multipart payload.
    const reader = request.body?.getReader(), chunks: Uint8Array[] = [];
    let size = 0;
    if (reader)
        try {
            while (true) {
                const { done, value } = await reader.read();
                if (done)
                    break;
                size += value.length;
                if (size > maximumBytes + 64 * 1024) {
                    await reader.cancel();
                    throw new AppError(413, "Upload exceeds permitted size");
                }
                chunks.push(value);
            }
        }
        finally {
            reader.releaseLock();
        }
    let form: FormData;
    try {
        form = await new Request("http://localhost/", { method: "POST", headers: { "content-type": request.headers.get("content-type")! }, body: Buffer.concat(chunks) }).formData();
    }
    catch {
        invalid("Invalid multipart upload");
    }
    const seen = new Set<string>(), fields: Record<string, string> = {}, input: Upload = { doc_type: "", title: "", original_filename: "", content_type: "", content: Buffer.alloc(0), replaces_document_id: "" };
    for (const [name, value] of form!) {
        if (seen.has(name) || !["file", "doc_type", "title", "reason", "replaces_document_id"].includes(name))
            invalid("Unknown or duplicate upload field");
        seen.add(name);
        const length = typeof value === "string" ? Buffer.byteLength(value) : value.size;
        if (length > (name === "file" ? maximumBytes : 2000))
            throw new AppError(413, "Upload exceeds permitted size");
        if (name === "file") {
            if (typeof value === "string") {
                input.content = Buffer.from(value);
                continue;
            }
            input.content = Buffer.from(await value.arrayBuffer());
            input.original_filename = value.name.split("/").pop() ?? "";
        }
        else
            fields[name] = typeof value === "string" ? value : Buffer.from(await value.arrayBuffer()).toString("utf8");
    }
    input.doc_type = fields.doc_type ?? "";
    input.title = fields.title ?? "";
    input.replaces_document_id = fields.replaces_document_id ?? "";
    return { input, reason: fields.reason ?? "" };
}
function disposition(kind: string, filename: string) {
    // RFC 5987 for Unicode; ASCII uses Go's quoted parameter representation.
    return /^[\x20-\x7e]*$/.test(filename) ? `${kind}; filename="${filename.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"` : `${kind}; filename*=utf-8''${encodeURIComponent(filename).replace(/['()*]/g, c => '%' + c.charCodeAt(0).toString(16).toUpperCase())}`;
}
export async function routeDocuments(request: Request, path: string, token: string, readJSON: (request: Request, max?: number) => Promise<unknown>): Promise<Response | null> {
    if (!ownsDocumentPath(path))
        return null;
    const match = /^\/suppliers\/([^/]+)\/documents(?:\/(legacy-import|([^/]+)\/(preview|download|archive)))?$/.exec(path)!;
    const [, supplier, tail, id, action] = match, method = request.method;
    if (!vendorID.test(supplier))
        invalid("Invalid supplier identity");
    const json = (value: unknown, status = 200) => Response.json(value, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });
    if (!tail) {
        if (method === "GET")
            return json(await listDocuments(token, supplier));
        if (method === "HEAD") {
            await authorizeUpload(token, supplier);
            return new Response(null, { status: 204 });
        }
        if (method === "POST") {
            await authorizeUpload(token, supplier);
            const { input, reason } = await multipart(request);
            return json(await uploadDocument(token, supplier, input, reason), 201);
        }
    }
    if (tail === "legacy-import" && method === "POST")
        return json(await importDocument(token, supplier, await readJSON(request, 512 * 1024)));
    if (id && action === "archive" && method === "POST") {
        const body = goObject(await readJSON(request, 512 * 1024), ["version", "reason"]);
        return json(await archiveDocument(token, supplier, id, integer(body.version), text(body.reason)));
    }
    if (id && (action === "preview" || action === "download") && method === "GET") {
        if (!vendorID.test(id))
            invalid("Invalid document identity");
        const preview = action === "preview", result = await downloadDocument(token, supplier, id, preview);
        const headers: Record<string, string> = { "Content-Type": result.contentType, "Content-Disposition": disposition(preview ? "inline" : "attachment", result.document.original_filename), "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" };
        if (preview) {
            headers["Content-Security-Policy"] = "sandbox; default-src 'none'; frame-ancestors 'self'";
            headers["X-Frame-Options"] = "SAMEORIGIN";
        }
        else
            headers["X-Encrypted-Storage"] = "true";
        return new Response(new Uint8Array(result.data), { headers });
    }
    return json({ detail: "Not found" }, 404);
}
