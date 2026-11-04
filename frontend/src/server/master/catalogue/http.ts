import "server-only";
import { MasterValidationError } from "../shared/validation";
import { importCategories, readCategories, saveCategory } from "../categories/service";
import { importManufacturer, readManufacturers, saveManufacturer } from "../manufacturers/service";
import { importTemplates, publishTemplate, readTemplate } from "../templates/service";
import { changeItem, createItem, readItems } from "../items/service";
export async function routeCatalogue(request: Request, path: string, token: string, read: (r: Request, max: number) => Promise<unknown>): Promise<{
    value: unknown;
    status?: number;
} | null> {
    const method = request.method, url = new URL(request.url), query = url.searchParams;
    const result = async (value: Promise<unknown>, status = 200) => ({ value: await value, status });
    const decode = (value: string) => { try {
        return decodeURIComponent(value);
    }
    catch {
        throw new MasterValidationError("identity", "path");
    } };
    if (path === "/categories" && method === "GET")
        return result(readCategories(token));
    if (path === "/categories/browse" && method === "GET")
        return result(readCategories(token, "browse", query));
    let match = /^\/categories\/detail\/(category|subcategory|unit)\/([^/]+)$/.exec(path);
    if (match && method === "GET")
        return result(readCategories(token, "detail", query, match[1], decode(match[2])));
    if (path === "/categories/import" && method === "POST")
        return result(importCategories(token, await read(request, 1024 * 1024)));
    match = /^\/categories\/(categories|subcategories|units)(?:\/([^/]+))?$/.exec(path);
    if (match) {
        const kind = { categories: "category", subcategories: "subcategory", units: "unit" }[match[1]]!;
        if (!match[2] && method === "POST")
            return result(saveCategory(token, kind, null, await read(request, 16384)), 201);
        if (match[2] && method === "PATCH")
            return result(saveCategory(token, kind, decode(match[2]), await read(request, 16384)));
    }
    if (path === "/manufacturers" && method === "GET")
        return result(readManufacturers(token));
    if (path === "/manufacturers/eligibility" && method === "GET")
        return result(readManufacturers(token, "options", "", query));
    if (path === "/manufacturers/legacy-import" && method === "POST")
        return result(importManufacturer(token, await read(request, 1024 * 1024)));
    if (path === "/manufacturers" && method === "POST")
        return result(saveManufacturer(token, "create", null, await read(request, 1024 * 1024)), 201);
    match = /^\/manufacturers\/([^/]+)(?:\/(profile|history))?$/.exec(path);
    if (match) {
        if (match[2] === "history" && method === "GET")
            return result(readManufacturers(token, "history", decode(match[1])));
        if (method === "PATCH" && match[2] !== "history")
            return result(saveManufacturer(token, match[2] === "profile" ? "profile" : "active", decode(match[1]), await read(request, 1024 * 1024)));
    }
    if (path === "/configuration-templates/import" && method === "POST")
        return result(importTemplates(token, await read(request, 1024 * 1024)));
    match = /^\/configuration-templates\/([^/]+)(?:\/(preview|publish))?$/.exec(path);
    if (match) {
        const id = decode(match[1]);
        if (!match[2] && method === "GET")
            return result(readTemplate(token, id));
        if (match[2] === "preview" && method === "POST")
            return result(readTemplate(token, id, await read(request, 16384)));
        if (match[2] === "publish" && method === "POST")
            return result(publishTemplate(token, id, await read(request, 1024 * 1024)));
    }
    if (path === "/catalogue-items" && method === "GET")
        return result(readItems(token));
    if (path === "/catalogue-items/references" && method === "GET")
        return result(readItems(token, "references"));
    if (path === "/catalogue-items" && method === "POST")
        return result(createItem(token, await read(request, 65536)), 201);
    match = /^\/catalogue-items\/([^/]+)(?:\/(history))?$/.exec(path);
    if (match) {
        const id = decode(match[1]);
        if (match[2] && method === "GET")
            return result(readItems(token, "history", id));
        if (!match[2] && method === "PATCH")
            return result(changeItem(token, id, await read(request, 65536)));
    }
    return null;
}
