import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
import { hasSameOrigin } from "@/lib/auth/request-origin";
const methods = new Set(["GET", "POST", "PATCH"]);
type Context = { params: Promise<{ path?: string[] }> };
async function route(request: Request, context: Context) {
 const { path = [] } = await context.params;
 const id = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
 const validPath = path.length === 0 || (path.length === 1 && id.test(path[0])) || (request.method === "GET" && path.length === 2 && id.test(path[0]) && ["preview.pdf", "template.docx"].includes(path[1]));
 if (!validPath || (request.method === "POST" && path.length !== 0) || (request.method === "PATCH" && path.length !== 1)) return Response.json({ detail: "Unsupported TSC format route" }, { status: 404 });
 if (request.method !== "GET" && !hasSameOrigin(request)) return Response.json({ detail: "Same-origin request required" }, { status: 403 });
 return proxyAuthenticatedRequest(request, context, "settings/tsc-formats", methods, 512 * 1024);
}
export const GET = route; export const POST = route; export const PATCH = route;
