import "server-only";
import { AppError } from "../errors";
import { MasterValidationError } from "./shared/validation";
import { getClient, listClients, saveClient, setClientActive } from "./clients/service";
import { listReferences, saveReference, setReferenceActive } from "./references/service";
export const ownsMasterPath = (path: string) => /^\/(?:clients|client-references)(?:\/|$)/.test(path);
function response(value: unknown, status = 200) {
    return Response.json(value, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });
}
async function readJSON(request: Request, maximum = 128 * 1024) {
    const reader = request.body?.getReader();
    if (!reader)
        throw new MasterValidationError("");
    let size = 0;
    const chunks: Uint8Array[] = [];
    try {
        while (true) {
            const { done, value } = await reader.read();
            if (done)
                break;
            size += value.length;
            if (size > maximum) {
                await reader.cancel();
                throw new AppError(413, "Request body too large");
            }
            chunks.push(value);
        }
    }
    finally {
        reader.releaseLock();
    }
    try {
        return JSON.parse(Buffer.concat(chunks).toString("utf8")) as unknown;
    }
    catch {
        throw new MasterValidationError("");
    }
}
export async function handleMaster(request: Request): Promise<Response> {
    try {
        const url = new URL(request.url), path = url.pathname.replace(/^\/api\/v1/, ""), method = request.method;
        const token = /^Bearer ([^\s]+)$/i.exec(request.headers.get("authorization") ?? "")?.[1] ?? "";
        const reference = /^\/client-references\/([^/]+)(?:\/([^/]+)(?:\/(activate|deactivate))?)?$/.exec(path);
        if (reference) {
            let id: string | null = null;
            try { id = reference[2] ? decodeURIComponent(reference[2]) : null; }
            catch { throw new MasterValidationError("reference_id", "path"); }
            if (!id && method === "GET") return response(await listReferences(token, reference[1], url.searchParams));
            if (!id && method === "POST") return response(await saveReference(token, reference[1], null, await readJSON(request, 4096)), 201);
            if (id && !reference[3] && method === "PATCH") return response(await saveReference(token, reference[1], id, await readJSON(request, 4096)));
            if (id && reference[3] && method === "POST") return response(await setReferenceActive(token, reference[1], id, reference[3] === "activate"));
        }
        if (path === "/clients" && method === "GET")
            return response(await listClients(token, url.searchParams));
        if (path === "/clients" && method === "POST")
            return response(await saveClient(token, null, await readJSON(request)), 201);
        const client = /^\/clients\/([^/]+)(?:\/(activate|deactivate))?$/.exec(path);
        if (client) {
            let id: string;
            try {
                id = decodeURIComponent(client[1]);
            }
            catch {
                throw new MasterValidationError("client_id", "path");
            }
            if (!client[2] && method === "GET")
                return response(await getClient(token, id));
            if (!client[2] && method === "PATCH")
                return response(await saveClient(token, id, await readJSON(request)));
            if (client[2] && method === "POST")
                return response(await setClientActive(token, id, client[2] === "activate"));
        }
        return response({ detail: "Not found" }, 404);
    }
    catch (error) {
        if (error instanceof MasterValidationError)
            return response({ detail: [{ type: "value_error", loc: [error.source, ...(error.field ? [error.field] : [])], msg: error.message }] }, 422);
        if (error instanceof AppError) {
            const result = response({ detail: error.message }, error.status);
            if (error.status === 401)
                result.headers.set("WWW-Authenticate", "Bearer");
            return result;
        }
        console.error("Master Data operation unavailable");
        return response({ detail: "Service temporarily unavailable" }, 503);
    }
}
