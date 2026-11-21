import "server-only";
import { MasterValidationError } from "../shared/validation";
import { createVendor, importVendor, listVendors, updateVendor } from "./service";
// Only profile routes migrate here. Nested documents, assignments and Finance
// verification remain explicitly Go-owned; native errors never retry in Go.
export const ownsVendorPath = (path: string) => /^\/suppliers(?:\/[^/]+)?$/.test(path);
export async function routeVendors(request: Request, path: string, token: string, read: (r: Request, max: number) => Promise<unknown>): Promise<{
    value: unknown;
    status?: number;
} | null> {
    if (path === "/suppliers" && request.method === "GET")
        return { value: await listVendors(token) };
    if (path === "/suppliers" && request.method === "POST")
        return { value: await createVendor(token, await read(request, 128 * 1024)), status: 201 };
    if (path === "/suppliers/legacy-import" && request.method === "POST")
        return { value: await importVendor(token, await read(request, 128 * 1024)) };
    const match = /^\/suppliers\/([^/]+)$/.exec(path);
    if (match && request.method === "PATCH") {
        let id: string;
        try {
            id = decodeURIComponent(match[1]);
        }
        catch {
            throw new MasterValidationError("supplier_id", "path");
        }
        return { value: await updateVendor(token, id, await read(request, 128 * 1024)) };
    }
    return null;
}
